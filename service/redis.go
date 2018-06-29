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

//Tagredis centralized
func Tagredis(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
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

//Deleteredis  centralized
func Deleteredis(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/redis/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

//Provisionredis  centralized
func Provisionredis(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/redis/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	redisurl, _ := bodyj.Get("REDIS_URL").String()
	location := strings.Replace(redisurl, "redis://", "", 1)
	parts := strings.Split(location, ".")
	name := parts[0]
	toreturn := make(map[string]interface{})
	toreturn["REDIS_URL"] = redisurl
	toreturn["spec"] = "redis:" + name

	r.JSON(201, toreturn)
}

//Getredisurl  centralized
func Getredisurl(params martini.Params, r render.Render) {
	var redis structs.Redisspec
	servicename := params["servicename"]

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/redis/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &redis)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	redis.Spec = "redis:" + servicename
	r.JSON(200, redis)
}

//Getredisplans  centralized
func Getredisplans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/redis/plans", nil)
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

//Getredisvars  centralized
func Getredisvars(servicename string) (error, map[string]interface{}) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("REDIS_BROKER_URL")+"/v1/redis/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, config
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Failure to obtain redis URL"), config
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}
