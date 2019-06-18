package router

import (
	"database/sql"
	"fmt"
	"context"
	"github.com/go-martini/martini"
	_ "github.com/lib/pq" //driver
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
	"net"
	"net/http"
	"os"
	runtime "region-api/runtime"
	spacepackage "region-api/space"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"
)

func Octhc(db *sql.DB, params martini.Params, r render.Render) {
	if _, err := GetAppIngress(db, false); err != nil {
		r.Text(http.StatusInternalServerError, "ERROR")
	}
	r.Text(http.StatusOK, "OK")
}

func DescribeRouters(db *sql.DB, params martini.Params, r render.Render) {
	list, err := getRouterList(db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	var routers []structs.Routerspec
	for _, element := range list {
		var spec structs.Routerspec
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

func getRouterList(db *sql.DB) (list []string, e error) {
	stmt, err := db.Prepare("select domain from routers")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()
	for rows.Next() {
		var domain string
		err := rows.Scan(&domain)
		if err != nil {
			return nil, err
		}
		list = append(list, domain)
	}
	return list, nil
}

func DescribeRouter(db *sql.DB, params martini.Params, r render.Render) {
	spec := structs.Routerspec{Domain: params["router"]}
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

func AddPath(db *sql.DB, spec structs.Routerpathspec, berr binding.Errors, r render.Render) {
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
	internalspace, err := spacepackage.IsInternalSpace(db, spec.Space)
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

	_, err = db.Exec("INSERT INTO routerpaths(domain, path, space, app, replacepath) VALUES($1,$2,$3,$4,$5)", spec.Domain, spec.Path, spec.Space, spec.App, spec.ReplacePath)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "Path Added"})
}

func DeletePath(db *sql.DB, params martini.Params, spec structs.Routerpathspec, berr binding.Errors, r render.Render) {
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

func CreateRouter(db *sql.DB, spec structs.Routerspec, berr binding.Errors, r render.Render) {
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
	msg, err := createRouter(spec, db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func GetDNSRecordType(address string) string {
	recType := "A"
	if net.ParseIP(address) == nil {
		recType = "CNAME"
	}
	return recType
}

func ResolveDNS(address string, private bool) ([]string, error) {
	resolver := net.DefaultResolver
	if !private {
		resolver = &net.Resolver{
			PreferGo:true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
	            d := net.Dialer{}
	            nameserver := "8.8.8.8"
	            if os.Getenv("PUBLIC_DNS_RESOLVER") != "" {
	            	nameserver = os.Getenv("PUBLIC_DNS_RESOLVER")
	            }
	            return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
	        },
		}
	}
	result, err := resolver.LookupHost(context.Background(), address)
	if err != nil {
		r, err := resolver.LookupCNAME(context.Background(), address)
		if err != nil {
			return nil, err
		}
		result = []string{r}
	}
	return result, nil
}

func SetDomainName(config *FullIngressConfig, fqdn string, internal bool) (error) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(fqdn)
	if err != nil {
		return err
	}

	// Set the DNS 
	for _, domain := range domains {
		if domain.Public && !internal {
			record, err := ResolveDNS(fqdn, false)
			if err == nil && record[0] == config.PublicExternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public external address was unnecessary, it already is set: %s == %s\n", fqdn, config.PublicExternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public external address to: %s = %s\n", fqdn, config.PublicExternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PublicExternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PublicExternal.Address), fqdn, []string{config.PublicExternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create public (external) dns: %s", err.Error())
			}
		}
		if !domain.Public && !internal {
			record, err := ResolveDNS(fqdn, true)
			if err == nil && record[0] == config.PublicInternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public internal address was unnecessary, it already is set: %s == %s\n", fqdn, config.PublicInternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public internal address to: %s = %s\n", fqdn, config.PublicInternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PublicInternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PublicInternal.Address), fqdn, []string{config.PublicInternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create private (external) dns: %s", err.Error())
			}
		}
		if !domain.Public && internal {
			record, err := ResolveDNS(fqdn, true)
			if err == nil && record[0] == config.PrivateInternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting private internal address was unnecessary, it already is set: %s == %s\n", fqdn, config.PrivateInternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting private internal address to: %s = %s\n", fqdn, config.PrivateInternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PrivateInternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PrivateInternal.Address), fqdn, []string{config.PrivateInternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create private (internal) dns: %s", err.Error())
			}
		}
	}
	return nil
}

func createRouter(spec structs.Routerspec, db *sql.DB) (structs.Messagespec, error) {
	config, err := GetDefaultIngressSiteAddresses()
	if err != nil {
		return structs.Messagespec{Status: http.StatusInternalServerError, Message: "Error while adding router (getting site addresses): " + err.Error()}, err
	}
	if err := SetDomainName(config, spec.Domain, spec.Internal); err != nil {
		fmt.Printf("WARNING: %s\n", err.Error())
	}
	var routerid string
	newrouteriduuid, _ := uuid.NewV4()
	newrouterid := newrouteriduuid.String()
	if err := db.QueryRow("INSERT INTO routers(routerid,domain,internal) VALUES($1,$2,$3) returning routerid;", newrouterid, spec.Domain, spec.Internal).Scan(&routerid); err != nil {
		return structs.Messagespec{Status: http.StatusInternalServerError, Message: "Error while adding router: " + err.Error()}, err
	}
	return structs.Messagespec{Status: http.StatusCreated, Message: "Router created with ID " + routerid}, nil
}

func GetNodePort(db *sql.DB, space string, app string) (string, error) {
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		return "", err
	}
	service, err := rt.GetService(space, app)
	if err != nil {
		return "", err
	}
	if len(service.Spec.Ports) == 1 {
		return strconv.Itoa(service.Spec.Ports[0].NodePort), nil
	} else {
		return "0", nil
	}
}

func GetAppUrl(db *sql.DB, app string, space string) (string, error) {
	externaldomain := os.Getenv("EXTERNAL_DOMAIN")
	if externaldomain == "" {
		return "", fmt.Errorf("No EXTERNAL_DOMAIN was defined")
	}
	internaldomain := os.Getenv("INTERNAL_DOMAIN")
	if externaldomain == "" {
		return "", fmt.Errorf("No INTERNAL_DOMAIN was defined")
	}
	internal, err := spacepackage.IsInternalSpace(db, space)
	if err != nil {
		return "", err
	}
	if internal {
		return app + "-" + space + "." + internaldomain, nil
	} else if space == "default" {
		return app + "." + externaldomain, nil
	} else {
		return app + "-" + space + "." + externaldomain, nil
	}
}

func PushRouter(db *sql.DB, params martini.Params, r render.Render) {
	pathspecs, err := GetPaths(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router := structs.Routerspec{Domain: params["router"], Paths: pathspecs}
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
		if err = ingress.DeleteRouter(router); err != nil {
			utils.ReportError(err, r)
			return
		}
		r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Router Updated"})
		return
	} else {
		if err = ingress.CreateOrUpdateRouter(router); err != nil {
			utils.ReportError(err, r)
			return
		}
		r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Router Updated"})
		return
	}
}

func DeleteRouter(db *sql.DB, params martini.Params, r render.Render) {
	pathspecs, err := GetPaths(db, params["router"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router := structs.Routerspec{Domain: params["router"], Paths: pathspecs}
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
	if err = ingress.DeleteRouter(router); err != nil {
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

func UpdatePath(db *sql.DB, spec structs.Routerpathspec, berr binding.Errors, r render.Render) {
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

func GetPaths(db *sql.DB, domain string) ([]structs.Routerpathspec, error) {
	stmt, err := db.Prepare("select distinct regexp_replace(path, '/$', '') as path, space,app,replacepath from routerpaths where domain=$1 order by path desc")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(domain)
	defer rows.Close()
	var pathspecs []structs.Routerpathspec
	for rows.Next() {
		pathspec := structs.Routerpathspec{Domain: domain}
		if err := rows.Scan(&pathspec.Path, &pathspec.Space, &pathspec.App, &pathspec.ReplacePath); err != nil {
			return nil, err
		}
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
