package app

import (
	structs "../structs"
	utils "../utils"
	"database/sql"
	"net/http"
	"strings"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func Unbindapp(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	space := params["space"]
	bindspec := params["_1"]
	bindtype := strings.Split(bindspec, ":")[0]
	bindname := strings.Split(bindspec, ":")[1]

	_, err := db.Exec("DELETE from appbindings  where appname=$1 and bindtype=$2 and bindname=$3 and space=$4", appname, bindtype, bindname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status:http.StatusOK, Message:bindspec + " deleted from " + appname})
}
