package app

import (
	"database/sql"
	"net/http"
	runtime "region-api/runtime"
	"region-api/utils"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

// GetAppLogs gets 100kb dump of pod logs from the top

func GetAppLogs(req *http.Request, db *sql.DB, params martini.Params, r render.Render) {
	app := params["appname"]
	space := params["space"]
        
	instance := params["instanceid"]
        timestampsparam := req.URL.Query().Get("timestamps")
        var timestamps bool
        if timestampsparam == "true" {
           timestamps = true
        }else{
           timestamps = false
        }       

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
 
	re, err := rt.GetPodLogs(space, app, instance, timestamps)
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

