package app

import (
	"database/sql"
	"../utils"
	"net/http"
	runtime "../runtime"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

// GetAppLogs gets 100kb dump of pod logs from the top
func GetAppLogs(db *sql.DB, params martini.Params, r render.Render) {
	app := params["appname"]
	space := params["space"]
	instance := params["instanceid"]
	
	rt, err := runtime.GetRuntimeFor(db, space);
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	re, err := rt.GetPodLogs(space, app, instance)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	var log struct {
		Logs string `json:"logs"`
	}
	log.Logs = re
	r.JSON(http.StatusOK, log)
}
