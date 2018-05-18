package app

import (
	structs "../structs"
	utils "../utils"
	runtime "../runtime"
	"database/sql"
	"strconv"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func Rollback(db *sql.DB, params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	revision := params["revision"]

	revisionint, err := strconv.Atoi(revision)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	err = rt.RollbackDeployment(space, app, revisionint)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, structs.Messagespec{Status:200, Message:app + " in space " + space + " rolled back to " + revision})
}

