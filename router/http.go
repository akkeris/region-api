package router

import (
	"database/sql"
	"fmt"
	"encoding/json"
	"github.com/go-martini/martini"
	_ "github.com/lib/pq" //driver
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

func GetPaths(db *sql.DB, domain string) ([]Route, error) {
	stmt, err := db.Prepare("select distinct regexp_replace(path, '/$', '') as path, space, app, replacepath, filters from routerpaths where domain=$1 order by path desc")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(domain)
	defer rows.Close()
	var pathspecs []Route
	for rows.Next() {
		pathspec := Route{Domain: domain}
		filters := make([]structs.HttpFilters, 0)
		filtersBytes := make([]byte, 0)
		if err := rows.Scan(&pathspec.Path, &pathspec.Space, &pathspec.App, &pathspec.ReplacePath, &filtersBytes); err != nil {
			fmt.Printf("Error: cannot pull database records: " + err.Error())
			return nil, err
		}
		if filtersBytes != nil && string(filtersBytes) != "" {
			if err := json.Unmarshal(filtersBytes, &filters); err != nil {
				fmt.Printf("Error: cannot unmarshal filter information: %s - original data: %s\n", err.Error(), string(filtersBytes))
				return nil, err
			}
		}
		pathspec.Filters = filters
		pathspecs = append(pathspecs, pathspec)
	}
	return pathspecs, nil
}

func GetPathsByApp(db *sql.DB, app string, space string) ([]Route, error) {
	stmt, err := db.Prepare("select distinct regexp_replace(path, '/$', '') as path, domain, space, app, replacepath, filters from routerpaths where app=$1 and space=$2 order by path desc")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(app, space)
	defer rows.Close()
	var pathspecs []Route
	for rows.Next() {
		pathspec := Route{}
		filters := make([]structs.HttpFilters, 0)
		filtersBytes := make([]byte, 0)
		if err := rows.Scan(&pathspec.Path, &pathspec.Domain, &pathspec.Space, &pathspec.App, &pathspec.ReplacePath, &filtersBytes); err != nil {
			return nil, err
		}
		if filtersBytes != nil && string(filtersBytes) != "" {
			if err := json.Unmarshal(filtersBytes, &filters); err != nil {
				return nil, err
			}
		}
		pathspec.Filters = filters
		pathspecs = append(pathspecs, pathspec)
	}
	return pathspecs, nil
}

func IsInternalRouter(db *sql.DB, domain string) (b bool, e error) {
	var isinternal bool
	err := db.QueryRow("select coalesce(internal,false) as internal from routers where domain=$1", domain).Scan(&isinternal)
	if err != nil {
		return false, err
	}
	return isinternal, nil
}

func HttpOcthc(db *sql.DB, params martini.Params, r render.Render) {
	if _, err := GetAppIngress(db, false); err != nil {
		r.Text(http.StatusInternalServerError, "ERROR")
	}
	r.Text(http.StatusOK, "OK")
}

func HttpDescribeRouters(db *sql.DB, params martini.Params, r render.Render) {
	stmt, err := db.Prepare("select domain from routers")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()
	list := make([]string, 0)
	for rows.Next() {
		var domain string
		err := rows.Scan(&domain)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		list = append(list, domain)
	}

	var routers []Router
	for _, element := range list {
		var spec Router
		spec.Domain = element
		internal, err := IsInternalRouter(db, element)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		spec.Internal = internal
		pathspecs, err := GetPaths(db, element)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		spec.Paths = pathspecs
		routers = append(routers, spec)
	}

	r.JSON(http.StatusOK, routers)
}

func HttpDescribeRouter(db *sql.DB, params martini.Params, r render.Render) {
	spec := Router{Domain: params["router"]}
	internal, err := IsInternalRouter(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Internal = internal
	pathspecs, err := GetPaths(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Paths = pathspecs
	r.JSON(http.StatusOK, spec)
}

func HttpAddPath(db *sql.DB, spec Route, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Path == "" {
		utils.ReportInvalidRequest("Path Cannot be blank", r)
		return
	}
	if spec.Space == "" {
		utils.ReportInvalidRequest("Space Cannot be blank", r)
		return
	}
	if spec.App == "" {
		utils.ReportInvalidRequest("App Cannot be blank", r)
		return
	}
	if spec.ReplacePath == "" {
		utils.ReportInvalidRequest("Replace Path Cannot be blank", r)
		return
	}
	internalrouter, err := IsInternalRouter(db, spec.Domain)
	if err != nil {
		utils.ReportInvalidRequest("Invalid Router", r)
		return
	}
	internalspace, err := utils.IsInternalSpace(db, spec.Space)
	if err != nil {
		utils.ReportInvalidRequest("Invalid Space", r)
		return
	}
	if internalrouter && !internalspace {
		utils.ReportInvalidRequest("Cannot Mix internal and external", r)
		return
	}
	if !internalrouter && internalspace {
		utils.ReportInvalidRequest("Cannot Mix internal and external", r)
		return
	}

	spec.App = strings.Replace(spec.App, "-"+spec.Space, "", -1)
	filtersJson := make([]byte, 0)

	if spec.Filters != nil {
		filtersJson, err = json.Marshal(spec.Filters);
		if err != nil {
			utils.ReportInvalidRequest("Cannot marshal filters: " + err.Error(), r)
			return
		}
	}

	_, err = db.Exec("INSERT INTO routerpaths(domain, path, space, app, replacepath, filters) VALUES($1,$2,$3,$4,$5,$6)", spec.Domain, spec.Path, spec.Space, spec.App, spec.ReplacePath, string(filtersJson))
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "Path Added"})
}

func HttpDeletePath(db *sql.DB, params martini.Params, spec Route, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if params["router"] == "" {
		utils.ReportInvalidRequest("Domain Cannot be blank", r)
		return
	}
	if spec.Path == "" {
		utils.ReportInvalidRequest("Path Cannot be blank", r)
		return
	}
	if _, err := db.Exec("DELETE from routerpaths where domain=$1 and path=$2", params["router"], spec.Path); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Path Deleted"})
}

func HttpCreateRouter(db *sql.DB, spec Router, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Domain == "" {
		utils.ReportInvalidRequest("Domain Cannot be blank", r)
		return
	}
	if spec.Internal == true {
		spec.Internal = true
	} else {
		spec.Internal = false
	}

	config, err := GetDefaultIngressSiteAddresses()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if err := SetDomainName(config, spec.Domain, spec.Internal); err != nil {
		fmt.Printf("WARNING: %s\n", err.Error())
	}
	var routerid string
	newrouteriduuid, _ := uuid.NewV4()
	newrouterid := newrouteriduuid.String()
	if err := db.QueryRow("INSERT INTO routers(routerid,domain,internal) VALUES($1,$2,$3) returning routerid;", newrouterid, spec.Domain, spec.Internal).Scan(&routerid); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "Router created with ID " + routerid})
}

func HttpPushRouter(db *sql.DB, params martini.Params, r render.Render) {
	pathspecs, err := GetPaths(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router := Router{Domain: params["router"], Paths: pathspecs}
	IsInternal, err := IsInternalRouter(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router.Internal = IsInternal
	ingress, err := GetSiteIngress(db, IsInternal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(pathspecs) == 0 {
		if err = ingress.DeleteRouter(router.Domain, router.Internal); err != nil {
			utils.ReportError(err, r)
			return
		}
		r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Router Updated"})
		return
	} else {
		if err = ingress.CreateOrUpdateRouter(router.Domain, router.Internal, router.Paths); err != nil {
			utils.ReportError(err, r)
			return
		}
		r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Router Updated"})
		return
	}
}

func HttpDeleteRouter(db *sql.DB, params martini.Params, r render.Render) {
	pathspecs, err := GetPaths(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router := Router{Domain: params["router"], Paths: pathspecs}
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	IsInternal, err := IsInternalRouter(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router.Internal = IsInternal
	ingress, err := GetSiteIngress(db, IsInternal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if err = ingress.DeleteRouter(router.Domain, router.Internal); err != nil {
		utils.ReportError(err, r)
		return
	}
	if _, err := db.Exec("DELETE from routerpaths where domain=$1", params["router"]); err != nil {
		utils.ReportError(err, r)
		return
	}

	dns := GetDnsProvider()
	domains, err := dns.Domain(router.Domain)
	if err != nil {
		fmt.Println("Error trying to fetch domain(s) for " + router.Domain + ": " + err.Error())
	} else {
		config, err := GetDefaultIngressSiteAddresses()
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, domain := range domains {
			if domain.Public && !router.Internal {
				if err := dns.RemoveDomainRecord(domain, GetDNSRecordType(config.PublicExternal.Address), router.Domain, []string{config.PublicExternal.Address}); err != nil {
					fmt.Printf("Error: Failed to remove public (external) dns: %s\n", err.Error())
				}
			}
			if !domain.Public && !router.Internal {
				if err := dns.RemoveDomainRecord(domain, GetDNSRecordType(config.PublicInternal.Address), router.Domain, []string{config.PublicInternal.Address}); err != nil {
					fmt.Printf("Error: Failed to remove private (external) dns: %s\n", err.Error())
				}
			}
			if !domain.Public && router.Internal {
				if err := dns.RemoveDomainRecord(domain, GetDNSRecordType(config.PrivateInternal.Address), router.Domain, []string{config.PrivateInternal.Address}); err != nil {
					fmt.Printf("Error: Failed to remove private (internal) dns: %s\n", err.Error())
				}
			}
		}
	}
	if _, err := db.Exec("DELETE from routers where domain=$1", params["router"]); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Router Deleted"})
}

func HttpUpdatePath(db *sql.DB, spec Route, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Path == "" {
		utils.ReportInvalidRequest("Path Cannot be blank", r)
		return
	}
	if spec.Space == "" {
		utils.ReportInvalidRequest("Space Cannot be blank", r)
		return
	}
	if spec.App == "" {
		utils.ReportInvalidRequest("App Cannot be blank", r)
		return
	}
	if spec.ReplacePath == "" {
		utils.ReportInvalidRequest("Replace Path Cannot be blank", r)
		return
	}
	_, err := db.Exec("UPDATE routerpaths set space=$1, app=$2, replacepath=$3 where domain=$4 and path=$5", spec.Space, spec.App, spec.ReplacePath, spec.Domain, spec.Path)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Path Updated"})
}

func HttpGetDomains(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domains()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, domains)
}

func HttpGetDomain(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}
	r.JSON(http.StatusOK, domains)
}

func HttpGetDomainRecords(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}
	records := make([]DomainRecord, 0)
	for _, domain := range domains {
		record, err := dns.DomainRecords(domain)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, r := range record {
			r.Domain = &Domain{
				ProviderId:  domain.ProviderId,
				Name:        domain.Name,
				Public:      domain.Public,
				Metadata:    domain.Metadata,
				Status:      domain.Status,
				RecordCount: domain.RecordCount,
			}
			records = append(records, r)
		}
	}
	r.JSON(http.StatusOK, records)
}

func HttpCreateDomainRecords(params martini.Params, spec DomainRecord, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if strings.ToUpper(spec.Type) != "A" && strings.ToUpper(spec.Type) != "AAAA" && strings.ToUpper(spec.Type) != "CNAME" {
		utils.ReportInvalidRequest("Only A, AAAA and CNAME records may be added to a domain record.", r)
		return
	}
	if strings.ToUpper(strings.Trim(params["domain"], ".")) == strings.ToUpper(strings.Trim(spec.Name, ".")) {
		utils.ReportInvalidRequest("Entries cannot be created for the root domain.", r)
		return
	}
	if spec.Name == "" {
		utils.ReportInvalidRequest("Entries cannot be created for the root domain.", r)
		return
	}

	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	records := make([]DomainRecord, 0)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}

	for _, domain := range domains {
		if err = dns.CreateDomainRecord(domain, spec.Type, spec.Name, spec.Values); err != nil {
			utils.ReportError(err, r)
			return
		}
		records = append(records, DomainRecord{
			Type:   spec.Type,
			Name:   spec.Name,
			Values: spec.Values,
			Domain: &Domain{
				ProviderId:  domain.ProviderId,
				Name:        domain.Name,
				Public:      domain.Public,
				Metadata:    domain.Metadata,
				Status:      domain.Status,
				RecordCount: domain.RecordCount,
			},
		})
	}
	r.JSON(http.StatusCreated, records)
}

func HttpRemoveDomainRecords(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	if strings.Contains(strings.ToLower(params["domain"]), strings.ToLower(params["name"])) || strings.ToLower(params["name"]) == strings.ToLower(params["domain"]) {
		r.JSON(http.StatusConflict, map[string]interface{}{"error": "CONFLICT", "error_description": "The name entry to delete was the domain itself."})
		return
	}
	if strings.ToUpper(strings.Trim(params["domain"], ".")) == strings.ToUpper(strings.Trim(params["name"], ".")) {
		utils.ReportInvalidRequest("Entries cannot be removed for the root of the domain.", r)
		return
	}
	if params["name"] == "" {
		utils.ReportInvalidRequest("Entries cannot be removed for the root of the domain.", r)
		return
	}

	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}

	toRemoveRecords := make([]DomainRecord, 0)
	for _, domain := range domains {
		records, err := dns.DomainRecords(domain)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, record := range records {
			// we only allow removal of A or CNAME records
			if record.Name == params["name"] && (strings.ToUpper(record.Type) == "A" || strings.ToUpper(record.Type) == "AAAA" || strings.ToUpper(record.Type) == "CNAME") {
				record.Domain = &Domain{
					ProviderId:  domain.ProviderId,
					Name:        domain.Name,
					Public:      domain.Public,
					Metadata:    domain.Metadata,
					Status:      domain.Status,
					RecordCount: domain.RecordCount,
				}
				toRemoveRecords = append(toRemoveRecords, record)
			}
		}
	}

	if len(toRemoveRecords) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["name"] + " was not found."})
		return
	}

	for _, rrec := range toRemoveRecords {
		if err = dns.RemoveDomainRecord(*rrec.Domain, rrec.Type, rrec.Name, rrec.Values); err != nil {
			utils.ReportError(err, r)
			return
		}
	}

	r.JSON(http.StatusCreated, toRemoveRecords)
}

func AddToMartini(m *martini.ClassicMartini) {
	m.Get("/v1/octhc/router", HttpOcthc)
	m.Get("/v1/routers", HttpDescribeRouters)
	m.Get("/v1/router/:router", HttpDescribeRouter)
	m.Post("/v1/router", binding.Json(Router{}), HttpCreateRouter)
	m.Put("/v1/router/:router", HttpPushRouter)
	m.Delete("/v1/router/:router", HttpDeleteRouter)
	m.Post("/v1/router/:router/path", binding.Json(Route{}), HttpAddPath)
	m.Delete("/v1/router/:router/path", binding.Json(Route{}), HttpDeletePath)
	m.Put("/v1/router/:router/path", binding.Json(Route{}), HttpUpdatePath)
	m.Get("/v1/sites/:site", HttpGetSite)
	m.Get("/v1/domains", HttpGetDomains)
	m.Get("/v1/domains/:domain", HttpGetDomain)
	m.Get("/v1/domains/:domain/records", HttpGetDomainRecords)
	m.Post("/v1/domains/:domain/records", binding.Json(DomainRecord{}), HttpCreateDomainRecords)
	m.Delete("/v1/domains/:domain/records/:name", HttpRemoveDomainRecords)
}
