package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"
)

//Describeapp centralized
func Describeapp(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	var (
		name string
		port int
	)
	err := db.QueryRow("select name, port from apps where name=$1", appname).Scan(&name, &port)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// To be backwards compatible with older systems lets fake a response back
			// this ... isn't ideal, but .. well..
			r.JSON(http.StatusOK, structs.Appspec{Name: "", Port: -1, Spaces: nil})
			return
		}
		utils.ReportError(err, r)
		return
	}
	spaceapps, err := getSpacesapps(db, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Appspec{Name: name, Port: port, Spaces: spaceapps})
}

func getSpacesapps(db *sql.DB, appname string) (sa []structs.Spaceappspec, err error) {
	var spaceapps []structs.Spaceappspec
	var space string
	var instances int
	var plan string
	var healthcheck string

	crows, err := db.Query("select space,instances,coalesce(plan,'noplan') as plan, COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from spacesapps where appname=$1", appname)
	if err != nil {
		utils.LogError("", err)
		return spaceapps, err
	}
	defer crows.Close()
	for crows.Next() {
		err := crows.Scan(&space, &instances, &plan, &healthcheck)
		if err != nil {
			utils.LogError("", err)
			return spaceapps, err
		}
		bindings, err := getBindings(db, appname, space)
		if err != nil {
			utils.LogError("", err)
			return spaceapps, err
		}
		spaceapps = append(spaceapps, structs.Spaceappspec{Appname: appname, Instances: instances, Space: space, Plan: plan, Healthcheck: healthcheck, Bindings: bindings})
	}
	return spaceapps, nil
}

func getBindings(db *sql.DB, appname string, space string) (b []structs.Bindspec, err error) {
	var bindings []structs.Bindspec
	var bindtype string
	var bindname string
	crows, err := db.Query("select bindtype, bindname from appbindings where appname=$1 and space=$2", appname, space)
	defer crows.Close()
	for crows.Next() {
		err := crows.Scan(&bindtype, &bindname)
		if err != nil {
			utils.LogError("", err)
			return bindings, err
		}
		bindings = append(bindings, structs.Bindspec{App: appname, Bindtype: bindtype, Bindname: bindname, Space: space})
	}
	return bindings, nil
}

func Describespace(db *sql.DB, params martini.Params, r render.Render) {
	var list []structs.Spaceappspec
	spacename := params["space"]

	var appname string
	var instances int
	var plan string
	var healthcheck string
	rows, err := db.Query("select appname, instances, coalesce(plan,'noplan') as plan, COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from spacesapps where space = $1", spacename)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&appname, &instances, &plan, &healthcheck)
		if err != nil {
			utils.ReportError(err, r)
			return
		}

		bindings, _ := getBindings(db, appname, spacename)
		list = append(list, structs.Spaceappspec{Appname: appname, Space: spacename, Instances: instances, Plan: plan, Healthcheck: healthcheck, Bindings: bindings})
	}
	r.JSON(http.StatusOK, list)
}

func DescribeappInSpace(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	spacename := params["space"]

	var instances int
	var plan string
	var healthcheck string
	err := db.QueryRow("select appname, instances, coalesce(plan,'noplan') as plan, COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from spacesapps where space = $1 and appname = $2", spacename, appname).Scan(&appname, &instances, &plan, &healthcheck)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// To be backwards compatible with older systems lets fake a response back
			// this ... isn't ideal, but .. well..
			r.JSON(http.StatusOK, structs.Spaceappspec{Appname: appname, Space: spacename, Instances: 0, Plan: "", Healthcheck: "", Bindings: nil})
			return
		}
		utils.ReportError(err, r)
		return
	}
	bindings, _ := getBindings(db, appname, spacename)
	currentapp := structs.Spaceappspec{Appname: appname, Space: spacename, Instances: instances, Plan: plan, Healthcheck: healthcheck, Bindings: bindings}

	rt, err := runtime.GetRuntimeFor(db, spacename)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	currentimage, err := rt.GetCurrentImage(spacename, appname)
	if err != nil {
		if err.Error() == "deployment not found" {
			// if there has yet to be a deployment we'll get a not found error,
			// just set the image to blank and keep moving.
			currentimage = ""
		} else {
			utils.ReportError(err, r)
			return
		}
	}
	currentapp.Image = currentimage
	r.JSON(http.StatusOK, currentapp)
}
