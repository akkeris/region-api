package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	runtime "region-api/runtime"
	utils "region-api/utils"
)

func GetDeployments(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	dslist, err := rt.GetDeploymentHistory(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, dslist)
}
