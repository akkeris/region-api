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

func GetInstances(db *sql.DB, params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, rt.GetPodDetails(space, app))
}

func DeleteInstance(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	instanceid := params["instanceid"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = rt.DeletePod(space, instanceid)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Deleted " + instanceid})
}
