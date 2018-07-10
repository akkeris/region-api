package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"

	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
)

//Tages centralized
func Tages(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/tag", bytes.NewBuffer(jsonStr))
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

//Deletees  centralized
func Deletees(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

//Provisiones  centralized
func Provisiones(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	toreturn := make(map[string]interface{})
	if resp.StatusCode != 201 {
		messagemap, _ := bodyj.Map()
		toreturn["message"] = messagemap["error"]
		toreturn["spec"] = ""
		r.JSON(resp.StatusCode, toreturn)
		return
	}
	if resp.StatusCode == 201 {
		messagemap, _ := bodyj.Map()
		toreturn["message"] = messagemap["message"]
		toreturn["spec"] = messagemap["spec"]
		r.JSON(resp.StatusCode, toreturn)
		return
	}
}

//Getesstatus  centralized
func Getesstatus(params martini.Params, r render.Render) {
	var es structs.Esspec
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/instance/"+servicename+"/status", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &es)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	es.Spec = "es:" + servicename
	r.JSON(resp.StatusCode, es)
}

//Getesurl  centralized
func Getesurl(params martini.Params, r render.Render) {
	var es structs.Esspec
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &es)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	es.Spec = "es:" + servicename
	if resp.StatusCode == 503 {
		if resp.Header.Get("x-ignore-errors") == "true" {
			r.Header().Add("x-ignore-errors", "true")
		}
	}
	r.JSON(resp.StatusCode, es)
}

//Getesplans  centralized
func Getesplans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/plans", nil)
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

//Getesvars  centralized
func Getesvars(servicename string) (error, map[string]interface{}) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("ES_BROKER_URL")+"/v1/es/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, config
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Cannot obtain ES_BROKER_URL from downstream broker."), config
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}
