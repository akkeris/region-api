package config

import (
	"database/sql"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

func Deletevar(db *sql.DB, params martini.Params, r render.Render) {
	setname := params["setname"]
	varname := params["varname"]

	_, err := db.Exec("DELETE from configvars where setname = $1 and varname = $2", setname, varname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: varname + " deleted"})
}

func Getvar(db *sql.DB, params martini.Params, r render.Render) {
	setname := params["setname"]
	varname := params["varname"]

	var varvalue string
	err := db.QueryRow("SELECT varvalue from configvars where setname = $1 and varname = $2", setname, varname).Scan(&varvalue)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Varspec{Setname: setname, Varname: varname, Varvalue: varvalue})
}

func Addvar(db *sql.DB, spec structs.Varspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	_, err := db.Exec("INSERT INTO configvars(setname,varname,varvalue) VALUES($1, $2, $3) ON CONFLICT ON CONSTRAINT configvars_pk DO UPDATE SET varvalue = $3", spec.Setname, spec.Varname, spec.Varvalue)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "added " + spec.Varname})
}

func Addvars(db *sql.DB, specs []structs.Varspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	var names []string
	tx, err := db.Begin()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	for _, spec := range specs {
		_, err := tx.Exec("INSERT INTO configvars(setname,varname,varvalue) VALUES($1,$2,$3) ON CONFLICT ON CONSTRAINT configvars_pk DO UPDATE SET varvalue = $3 returning varname;", spec.Setname, spec.Varname, spec.Varvalue)
		if err != nil {
			rollbackerr := tx.Rollback()
			if rollbackerr != nil {
				fmt.Printf("FATAL: Cannot rollback: %s\n", rollbackerr)
			}
			utils.ReportError(err, r)
			return
		}
		names = append(names, spec.Varname)
	}
	err = tx.Commit()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "added " + strings.Join(names, ",")})
}

func Updatevar(db *sql.DB, spec structs.Varspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	_, err := db.Exec("update configvars set varvalue=$3 where setname=$1 and varname=$2", spec.Setname, spec.Varname, spec.Varvalue)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, spec)
}

func GetConfigVars(db *sql.DB, configset string) (map[string]string, error) {
	setname := configset

	dump := make(map[string]string)
	var (
		varname  string
		varvalue string
	)
	rows, err := db.Query("select varname, varvalue from configvars where setname=$1 union select varname, varvalue from configvars where setname in(select child from includes where parent=$1)", setname)
	if err != nil {
		return dump, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&varname, &varvalue)
		if err != nil {
			return dump, err
		}
		dump[varname] = varvalue
	}
	err = rows.Err()
	if err != nil {
		return dump, err
	}
	return dump, nil
}

func GetBindings(db *sql.DB, space string, app string) (configset string, services []structs.Bindspec, err error) {
	rows, err := db.Query("select bindtype, bindname from appbindings where appname = $1 and space = $2 and bindtype != 'build'", app, space)
	if err != nil {
		return "", []structs.Bindspec{}, err
	}
	defer rows.Close()
	configset = ""
	for rows.Next() {
		var bindtype string
		var bindname string
		err = rows.Scan(&bindtype, &bindname)
		if err != nil {
			return "", []structs.Bindspec{}, err
		}
		if bindtype == "config" {
			configset = bindname
		}
		if bindtype != "config" {
			services = append(services, structs.Bindspec{App: app, Space: space, Bindtype: bindtype, Bindname: bindname})
		}
	}
	err = rows.Err()
	if err != nil {
		return "", []structs.Bindspec{}, err
	}

	return configset, services, nil
}
