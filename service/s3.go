package service

import (
	structs "../structs"
	utils "../utils"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"errors"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Tags3 centralized
func Tags3(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
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

//Deletes3  centralized
func Deletes3(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/s3/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

//Provisions3  centralized
func Provisions3(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/s3/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	s3url, _ := bodyj.Get("S3_LOCATION").String()
	s3bucket, _ := bodyj.Get("S3_BUCKET").String()
	s3accesskey, _ := bodyj.Get("S3_ACCESS_KEY").String()
	s3secretkey, _ := bodyj.Get("S3_SECRET_KEY").String()
	toreturn := make(map[string]interface{})
	toreturn["S3_LOCATION"] = s3url
	toreturn["S3_ACCESS_KEY"] = s3accesskey
	toreturn["S3_SECRET_KEY"] = s3secretkey
	toreturn["S3_BUCKET"] = s3bucket
	toreturn["spec"] = "s3:" + s3bucket

	r.JSON(201, toreturn)
}

//Gets3url  centralized
func Gets3url(params martini.Params, r render.Render) {
	var s3 structs.S3spec
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/s3/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &s3)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	s3.Spec = "s3:" + servicename
	r.JSON(200, s3)
}

//Gets3plans  centralized
func Gets3plans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/s3/plans", nil)
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

//Gets3vars  centralized
func Gets3vars(servicename string) (error, map[string]interface{}) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("S3_BROKER_URL")+"/v1/s3/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, map[string]interface{}{}
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Cannot obtain S3_BROKER_URL from downstream broker."), config
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}
