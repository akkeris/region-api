package service

import (
	structs "../structs"
	utils "../utils"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"errors"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

func Getauroramysqlplans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/aurora-mysql/plans", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	bodymap, maperr := bodyj.Map()
	if maperr != nil {
		utils.ReportError(maperr, r)
		return
	}
	for k, v := range bodymap {
		var plan structs.Planspec
		plan.Size = k
		plan.Description = v.(string)
		plans = append(plans, plan)
	}
	r.JSON(200, plans)
}

func Getauroramysqlurl(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var auroramysql structs.Auroramysqlspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/aurora-mysql/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	database_readonly_url, _ := bodyj.Get("DATABASE_READONLY_URL").String()
	parts := strings.Split(databaseurl, "/")
	specname := "auroramysql:" + parts[len(parts)-1]
	auroramysql.DatabaseUrl = databaseurl
	auroramysql.DatabaseReadonlyUrl = database_readonly_url
	auroramysql.Spec = specname
	r.JSON(200, auroramysql)
}

func Provisionauroramysql(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	client := &http.Client{}
	str, err := json.Marshal(spec)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	jsonStr := []byte(string(str))
	req, err := http.NewRequest("POST", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/aurora-mysql/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	database_readonly_url, _ := bodyj.Get("DATABASE_READONLY_URL").String()

	parts := strings.Split(databaseurl, "/")
	specname := "auroramysql:" + parts[len(parts)-1]

	var auroramysql structs.Auroramysqlspec
	auroramysql.DatabaseUrl = databaseurl
	auroramysql.DatabaseReadonlyUrl = database_readonly_url
	auroramysql.Spec = specname
	r.JSON(201, auroramysql)
}

func Deleteauroramysql(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/aurora-mysql/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}

func Tagauroramysql(spec structs.Tagspec, berr binding.Errors, r render.Render) {
	var message structs.Messagespec
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	client := &http.Client{}
	str, err := json.Marshal(spec)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	jsonStr := []byte(string(str))
	req, err := http.NewRequest("POST", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	var brokerresponse structs.Brokerresponse
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		bb, _ := ioutil.ReadAll(resp.Body)
		_ = json.Unmarshal(bb, &brokerresponse)
		message.Status = 201
		message.Message = brokerresponse.Response
		r.JSON(message.Status, message)
	}
}

func Getauroramysqlvars(servicename string) (error, map[string]interface{}) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("AURORAMYSQL_BROKER_URL")+"/v1/aurora-mysql/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, map[string]interface{}{}
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Cannot obtain AURORAMYSQL_BROKER_URL from broker."), map[string]interface{}{}
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}
