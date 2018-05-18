package app

import (
	structs "../structs"
	utils "../utils"
	"database/sql"
	"net/http"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Createbind centralized
func Createbind(db *sql.DB, spec structs.Bindspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.App == "" {
		utils.ReportInvalidRequest("Application Name can not be blank", r)
		return
	}
	if spec.Space == "" {
		utils.ReportInvalidRequest("Space Name can not be blank", r)
		return
	}
	if spec.Bindtype == "" {
		utils.ReportInvalidRequest("Bind Type can not be blank", r)
		return
	}
	if spec.Bindname == "" {
		utils.ReportInvalidRequest("Bind Name can not be blank", r)
		return
	}
	var appname string
	inserterr := db.QueryRow("INSERT INTO appbindings(appname,space,bindtype,bindname) VALUES($1,$2,$3,$4) returning appname;", spec.App, spec.Space, spec.Bindtype, spec.Bindname).Scan(&appname)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status:http.StatusCreated, Message:"added " + appname + " bound to " + spec.Bindname + " in space " + spec.Space})
}
