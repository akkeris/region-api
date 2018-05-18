package app

import (
	runtime "../runtime"
	structs "../structs"
	utils "../utils"
	"database/sql"
	"net/http"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func Restart(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	app := params["appname"]	
	
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = rt.RestartDeployment(space, app)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status:http.StatusOK, Message:"Restart Submitted"})
}
