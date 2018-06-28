package config

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
)

func Includeset(db *sql.DB, params martini.Params, r render.Render) {
	parent := params["parent"]
	child := params["child"]

	var parentname string
	err := db.QueryRow("INSERT INTO includes(parent,child) VALUES($1,$2) returning parent", parent, child).Scan(&parentname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: parent + " now includes " + child})
}

func Createset(db *sql.DB, spec structs.Setspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	setname := spec.Setname
	settype := spec.Settype
	newsetiduuid, _ := uuid.NewV4()
	newsetid := newsetiduuid.String()
	var setid string

	err := db.QueryRow("INSERT INTO sets(setid,name,type) VALUES($1,$2,$3) returning setid;", newsetid, setname, settype).Scan(&setid)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "Set " + setname + " created"})
}

func Deleteset(db *sql.DB, params martini.Params, r render.Render) {
	setname := params["setname"]

	_, err := db.Exec("DELETE from configvars where setname=$1", setname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	_, err = db.Exec("DELETE from sets where name=$1", setname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: setname + " deleted"})
}

func Deleteinclude(db *sql.DB, params martini.Params, r render.Render) {
	parent := params["parent"]
	child := params["child"]

	_, err := db.Exec("DELETE from includes where parent=$1 and child=$2", parent, child)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: parent + "-" + child + " relationship deleted"})
}

func Listsets(db *sql.DB, params martini.Params, r render.Render) {
	var sets []structs.Setspec
	var (
		setname string
		settype string
	)
	rows, err := db.Query("select name, type from sets")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&setname, &settype)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		sets = append(sets, structs.Setspec{Setname: setname, Settype: settype})
	}
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, sets)
}

func Dumpset(db *sql.DB, params martini.Params, r render.Render) {
	setname := params["setname"]
	var (
		varname  string
		varvalue string
	)
	rows, err := db.Query("select varname,varvalue from configvars where setname=$1 union select varname,varvalue from configvars where setname in(select child from includes where parent=$1)", setname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	var vars []structs.Varspec
	for rows.Next() {
		err := rows.Scan(&varname, &varvalue)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		vars = append(vars, structs.Varspec{Setname: setname, Varname: varname, Varvalue: varvalue})
	}
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, vars)
}
