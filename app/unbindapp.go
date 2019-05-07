package app

import (
	"database/sql"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

func Unbindapp(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	space := params["space"]
	bindspec := params["_1"]
	bindtype := strings.Split(bindspec, ":")[0]
	bindname := strings.Split(bindspec, ":")[1]

	_, err := db.Exec("DELETE from appbindings where appname=$1 and bindtype=$2 and bindname=$3 and space=$4", appname, bindtype, bindname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	_, err = db.Exec("DELETE from configvarsmap where appname=$1 and bindtype=$2 and bindname=$3 and space=$4", appname, bindtype, bindname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: bindspec + " deleted from " + appname})
}

func Deletebindmap(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	space := params["space"]
	bindtype := params["bindtype"]
	bindname := params["bindname"]
	mapid := params["mapid"]
	if mapid == "" || appname == "" || space == "" || bindtype == "" || bindname == "" {
		utils.ReportError(fmt.Errorf("Invalid parameter specified in delete bind map."), r)
		return
	}
	_, err := db.Exec("DELETE from configvarsmap where appname=$1 and bindtype=$2 and bindname=$3 and space=$4 and mapid=$5", appname, bindtype, bindname, space, mapid)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: mapid + " deleted"})
}
