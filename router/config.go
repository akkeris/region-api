package router

import (
	"database/sql"
	"fmt"
	spacepackage "region-api/space"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"

	"github.com/go-martini/martini"
	_ "github.com/lib/pq" //driver
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
)

func DescribeRouters(db *sql.DB, params martini.Params, r render.Render) {

	list, err := getRouterList(db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	var routers []structs.Routerspec
	for _, element := range list {
		fmt.Println(element)

		var spec structs.Routerspec
		spec.Domain = element
		internal, err := IsInternalRouter(db, element)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		spec.Internal = internal

		pathspecs, err := getPaths(db, element)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		spec.Paths = pathspecs
		routers = append(routers, spec)
	}

	r.JSON(200, routers)
}

func getRouterList(db *sql.DB) (list []string, e error) {
	var msg structs.Messagespec
	stmt, err := db.Prepare("select domain from routers")
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = err.Error()
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()

	for rows.Next() {
		var domain string
		err := rows.Scan(&domain)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		list = append(list, domain)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return list, nil

}
func DescribeRouter(db *sql.DB, params martini.Params, r render.Render) {
	domain := params["router"]
	var spec structs.Routerspec
	spec.Domain = domain
	internal, err := IsInternalRouter(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Internal = internal
	pathspecs, err := getPaths(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Paths = pathspecs
	r.JSON(200, spec)
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

	var msg structs.Messagespec
	fmt.Println(spec.Path)
	fmt.Println(spec.Space)
	fmt.Println(spec.App)
	fmt.Println(spec.ReplacePath)
	spec.App = strings.Replace(spec.App, "-"+spec.Space, "", -1)
	msg, err = addPath(spec, db)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func addPath(spec structs.Routerpathspec, db *sql.DB) (structs.Messagespec, error) {
	var msg structs.Messagespec
	_, err := db.Exec("INSERT INTO routerpaths(domain, path, space, app, replacepath) VALUES($1,$2,$3,$4,$5)", spec.Domain, spec.Path, spec.Space, spec.App, spec.ReplacePath)
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = "Error while inserting"
		return msg, err
	}
	msg.Status = 201
	msg.Message = "Path Added"
	return msg, nil
}

func DeletePath(db *sql.DB, params martini.Params, spec structs.Routerpathspec, berr binding.Errors, r render.Render) {

	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	domain := params["router"]
	path := spec.Path
	if domain == "" {
		utils.ReportInvalidRequest("Domain Cannot be blank", r)
		return
	}
	if path == "" {
		utils.ReportInvalidRequest("Path Cannot be blank", r)
		return
	}

	var msg structs.Messagespec
	fmt.Println(path)
	fmt.Println(domain)

	msg, err := deletePath(domain, path, db)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func deletePath(domain string, path string, db *sql.DB) (m structs.Messagespec, err error) {

	var msg structs.Messagespec
	del, err := db.Exec("DELETE from routerpaths where domain=$1 and path=$2", domain, path)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	_, err = del.RowsAffected()
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Path Deleted"

	return msg, nil

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
	var msg structs.Messagespec
	fmt.Println(spec.Domain)
	msg, err := createRouter(spec, db)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func createRouter(spec structs.Routerspec, db *sql.DB) (m structs.Messagespec, err error) {
	var msg structs.Messagespec
	fmt.Println(spec.Domain)

	newrouteriduuid, _ := uuid.NewV4()
	newrouterid := newrouteriduuid.String()

	var routerid string
	inserterr := db.QueryRow("INSERT INTO routers(routerid,domain,internal) VALUES($1,$2,$3) returning routerid;", newrouterid, spec.Domain, spec.Internal).Scan(&routerid)
	if inserterr != nil {
		fmt.Println(inserterr)
		msg.Status = 500
		msg.Message = "Error while inserting"
		return msg, inserterr
	}
	addDNSRecord(db, spec.Domain)
	msg.Status = 201
	msg.Message = "Router created with ID" + routerid
	return msg, nil
}

func PushRouter(db *sql.DB, params martini.Params, r render.Render) {
	domain := params["router"]
	pathspecs, err := getPaths(db, domain)
	var router structs.Routerspec
	router.Domain = domain
	router.Paths = pathspecs
	isinternal, err := IsInternalRouter(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	router.Internal = isinternal
	if len(pathspecs) == 0 {
		msg, err := DeleteF5(router, db)
		if err != nil {
			utils.ReportError(err, r)
		}
		msg.Status = 200
		msg.Message = "Router Updated"
		r.JSON(msg.Status, msg)
		return
	} else {
		msg, err := pushRouter(db, router)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		msg.Status = 200
		msg.Message = "Router Updated"
		r.JSON(msg.Status, msg)
	}
}

func pushRouter(db *sql.DB, r structs.Routerspec) (m structs.Messagespec, e error) {
	msg, err := UpdateF5(r, db)
	return msg, err

}

func DeleteRouter(db *sql.DB, params martini.Params, r render.Render) {
	domain := params["router"]
	var spec structs.Routerspec
	spec.Domain = domain
	pathspecs, err := getPaths(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Paths = pathspecs
	isinternal, err := IsInternalRouter(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	spec.Internal = isinternal
	msg, err := deleteRouter(db, spec)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	msg, err = deletePaths(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	msg, err = deleteRouterBase(db, domain)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	msg.Status = 200
	msg.Message = "Router Deleted"
	r.JSON(msg.Status, msg)
}

func deleteRouterBase(db *sql.DB, domain string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	delrouter, err := db.Exec("DELETE from routers where domain=$1", domain)
	if err != nil {
		return msg, err
	}
	_, err = delrouter.RowsAffected()
	if err != nil {
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Router removed"
	return msg, nil
}

func deletePaths(db *sql.DB, domain string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	delrouter, err := db.Exec("DELETE from routerpaths where domain=$1", domain)
	if err != nil {
		return msg, err
	}
	_, err = delrouter.RowsAffected()
	if err != nil {
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Paths removed"
	return msg, nil
}

func deleteRouter(db *sql.DB, router structs.Routerspec) (m structs.Messagespec, e error) {

	msg, err := DeleteF5(router, db)
	return msg, err
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
	var msg structs.Messagespec
	fmt.Println(spec.Path)
	fmt.Println(spec.Space)
	fmt.Println(spec.App)
	fmt.Println(spec.ReplacePath)

	msg, err := updatePath(spec, db)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func updatePath(spec structs.Routerpathspec, db *sql.DB) (m structs.Messagespec, e error) {

	var msg structs.Messagespec

	var path string

	err := db.QueryRow("UPDATE routerpaths set space=$1, app=$2, replacepath=$3  where domain=$4 and path=$5 returning path;", spec.Space, spec.App, spec.ReplacePath, spec.Domain, spec.Path).Scan(&path)
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = "Error while updating"
		return msg, err
	}
	msg.Status = 201
	msg.Message = "Path Updated"

	return msg, nil
}

func getPaths(db *sql.DB, domain string) (p []structs.Routerpathspec, err error) {
	var msg structs.Messagespec
	var (
		path        string
		space       string
		app         string
		replacepath string
	)
	stmt, err := db.Prepare("select distinct regexp_replace(path, '/$', '') as path, space,app,replacepath from routerpaths where domain=$1 order by path desc")
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = err.Error()
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(domain)
	defer rows.Close()
	var pathspecs []structs.Routerpathspec
	for rows.Next() {
		var pathspec structs.Routerpathspec
		err := rows.Scan(&path, &space, &app, &replacepath)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		pathspec.Domain = domain
		pathspec.Path = path
		pathspec.Space = space
		pathspec.App = app
		pathspec.ReplacePath = replacepath
		pathspecs = append(pathspecs, pathspec)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return pathspecs, nil
}

func IsInternalRouter(db *sql.DB, domain string) (b bool, e error) {
	var isinternal bool
	err := db.QueryRow("select coalesce(internal,false) as internal from routers where domain=$1", domain).Scan(&isinternal)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	return isinternal, nil
}
