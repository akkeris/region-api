package service

import (
	structs "../structs"
	utils "../utils"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Tagrabbitmq centralized
func Tagrabbitmq(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
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

//Deleterabbitmq  centralized
func Deleterabbitmq(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/rabbitmq/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

//Provisionrabbitmq  centralized
func Provisionrabbitmq(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/rabbitmq/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	rabbitmqurl, _ := bodyj.Get("RABBITMQ_URL").String()
	location := rabbitmqurl
	parts := strings.Split(location, "/")
	name := parts[len(parts)-1]
	toreturn := make(map[string]interface{})
	toreturn["RABBITMQ_URL"] = rabbitmqurl
	toreturn["spec"] = "rabbitmq:" + name

	r.JSON(201, toreturn)
}

//Getrabbitmqurl  centralized
func Getrabbitmqurl(params martini.Params, r render.Render) {
	var rabbitmq structs.Rabbitmqspec
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/rabbitmq/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &rabbitmq)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	rabbitmq.Spec = "rabbitmq:" + servicename
	r.JSON(200, rabbitmq)
}

//Getrabbitmqplans  centralized
func Getrabbitmqplans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/rabbitmq/plans", nil)
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

//Getrabbitmqvars  centralized
func Getrabbitmqvars(servicename string) map[string]interface{} {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("RABBITMQ_BROKER_URL")+"/v1/rabbitmq/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return config
}
