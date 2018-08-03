package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
)

//Deleteapp centralized
func Deleteapp(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	var space string
	err := db.QueryRow("select space from spacesapps where appname = $1", appname).Scan(&space)
	if err == nil && space != "" {
		utils.ReportInvalidRequest("application still exists in spaces: "+space, r)
		return
	}
	_, err = db.Exec("DELETE from apps where name=$1", appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: appname + " deleted"})
}
