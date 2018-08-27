package app

import (
	"database/sql"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"github.com/nu7hatch/gouuid"
	structs "region-api/structs"
	utils "region-api/utils"
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
	_, inserterr := db.Exec("INSERT INTO appbindings(appname,space,bindtype,bindname) VALUES($1,$2,$3,$4)", spec.App, spec.Space, spec.Bindtype, spec.Bindname)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "added " + spec.App + " bound to " + spec.Bindname + " in space " + spec.Space})
}

func Createbindmap(db *sql.DB, spec structs.Bindmapspec, berr binding.Errors, r render.Render) {
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
	if spec.Action != "delete" && spec.Action != "rename" && spec.Action != "copy" {
		utils.ReportInvalidRequest("Action was an invalid value, only delete, rename or copy are allowed.", r)
		return
	}
	if spec.VarName == "" {
		utils.ReportInvalidRequest("Var name can not be blank", r)
		return
	}
	if spec.Action != "delete" && spec.NewName == "" {
		utils.ReportInvalidRequest("New name can not be blank", r)
		return
	}
	
	mapid, err := uuid.NewV4()
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	_, inserterr := db.Exec("INSERT INTO configvarsmap (appname,space,bindtype,bindname,action,varname,newname,mapid) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", spec.App, spec.Space, spec.Bindtype, spec.Bindname, spec.Action, spec.VarName, spec.NewName, mapid.String())
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}

	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: mapid.String()})
}
