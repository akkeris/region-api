package monitor

import (
	"database/sql"
	"fmt"
	structs "region-api/structs"
	utils "region-api/utils"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"bytes"
	"encoding/json"
	"net/http"
)

func DeleteCallback(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["app"]
	space := params["space"]
	tag := params["tag"]
	method := params["method"]

	stmt, err := db.Prepare("DELETE from callbacks  where space = $1 and appname = $2 and tag = $3 and method = $4")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	res, err := stmt.Exec(space, appname, tag, method)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	_, err = res.RowsAffected()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	var message structs.Messagespec
	message.Status = 200
	message.Message = tag + "/" + method + " deleted from " + space + "/" + appname
	r.JSON(200, message)
}

func CreateCallback(db *sql.DB, spec structs.Callbackspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	_, err := json.Marshal(spec)
	if err != nil {
		fmt.Println("Error preparing request")
		utils.ReportError(err, r)
		return
	}
	var appname string
	inserterr := db.QueryRow("INSERT INTO callbacks (space,appname,tag,method,url) VALUES($1,$2,$3,$4,$5) returning appname;", spec.Space, spec.Appname, spec.Tag, spec.Method, spec.Url).Scan(&appname)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	var message structs.Messagespec
	message.Status = 201
	message.Message = "added " + spec.Method + "/ " + spec.Tag + " to " + spec.Appname + " in space " + spec.Space
	r.JSON(201, message)
}

func Callback(db *sql.DB, spec structs.NagiosAlert, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	str, err := json.Marshal(spec)
	if err != nil {
		fmt.Println("Error preparing request")
		utils.ReportError(err, r)
		return
	}
	jsonStr := []byte(string(str))
	client := &http.Client{}
	callbacks, err := getCallbacks(db, spec.Space, spec.Appname)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	for _, element := range callbacks {
		fmt.Println(element.Url)
		req, err := http.NewRequest(element.Method, element.Url, bytes.NewBuffer(jsonStr))
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			utils.ReportError(err, r)
			return
		}
		defer resp.Body.Close()

	}
	var msg structs.Messagespec
	msg.Status = 200
	msg.Message = "Callback Initiated"
	r.JSON(msg.Status, msg)
}

func GetCallbacks(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["app"]
	space := params["space"]
	var callbacks []structs.Callbackspec
	callbacks, err := getCallbacks(db, space, appname)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, callbacks)
}

func getCallbacks(db *sql.DB, space string, appname string) (cb []structs.Callbackspec, err error) {
	stmt, err := db.Prepare("select tag, method, url from callbacks where space = $1 and appname = $2")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(space, appname)
	defer rows.Close()
	var url string
	var method string
	var tag string
	var callbacks []structs.Callbackspec
	for rows.Next() {
		err := rows.Scan(&tag, &method, &url)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		var callback structs.Callbackspec
		callback.Appname = appname
		callback.Space = space
		callback.Tag = tag
		callback.Method = method
		callback.Url = url
		callbacks = append(callbacks, callback)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return callbacks, nil
}
