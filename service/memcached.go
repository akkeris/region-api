package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
	"errors"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Tagmemcached centralized
func Tagmemcached(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
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

//Deletememcached  centralized
func Deletememcached(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

//Provisionmemcached  centralized
func Provisionmemcached(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	memcachedurl, _ := bodyj.Get("MEMCACHED_URL").String()
	location := memcachedurl
	parts := strings.Split(location, ".")
	name := parts[0]
	toreturn := make(map[string]interface{})
	toreturn["MEMCACHED_URL"] = memcachedurl
	toreturn["spec"] = "memcached:" + name

	r.JSON(201, toreturn)
}

//Getmemcachedurl  centralized
func Getmemcachedurl(params martini.Params, r render.Render) {
	var memcached structs.Memcachedspec
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	uerr := json.Unmarshal(bodybytes, &memcached)
	if uerr != nil {
		utils.ReportError(uerr, r)
		return
	}
	memcached.Spec = "memcached:" + servicename
	r.JSON(200, memcached)
}

//Getmemcachedplans  centralized
func Getmemcachedplans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/plans", nil)
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

//Getmemcachedvars  centralized
func Getmemcachedvars(servicename string) (error, map[string]interface{}) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, config
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Failure to obtain MEMCACHED_URL"), config
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}

func FlushMemcached(params martini.Params, r render.Render) {
	_, err := flushmemcached(params["name"])
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	var msg structs.Messagespec
	msg.Status = 200
	msg.Message = "OK"
	r.JSON(msg.Status, msg)
}

func flushmemcached(name string) (r string, e error) {
	var toreturn string
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/operations/cache/"+name, nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	return string(bodybytes), nil
}

func GetMemcachedStats(params martini.Params, r render.Render) {

	stats, err := getstats(params["name"])
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return

	}
	r.JSON(200, stats)
}

func getstats(name string) (s []structs.KV, e error) {
	var stats []structs.KV
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("MEMCACHED_BROKER_URL")+"/v1/memcached/operations/stats/"+name, nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return stats, err
	}
	defer resp.Body.Close()
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bodybytes, &stats)
	if err != nil {
		fmt.Println(err)
		return stats, err
	}
	return stats, nil
}
