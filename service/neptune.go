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
	"strings"

	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

func GetNeptunePlans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/plans", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

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

func GetNeptuneURL(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var neptune structs.Neptunespec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("NEPTUNE_DATABASE_URL").String()
	accesskey, _ := bodyj.Get("NEPTUNE_ACCESS_KEY").String()
	secretkey, _ := bodyj.Get("NEPTUNE_SECRET_KEY").String()
	region, _ := bodyj.Get("NEPTUNE_REGION").String()

	splitURL := strings.Split(databaseurl, ".")
	specname := "neptune:" + splitURL[0]
	neptune.Spec = specname
	neptune.NeptuneDatabaseURL = databaseurl
	neptune.NeptuneAccessKey = accesskey
	neptune.NeptuneSecretKey = secretkey
	neptune.NeptuneRegion = region
	r.JSON(200, neptune)
}

func ProvisionNeptune(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("NEPTUNE_DATABASE_URL").String()
	accesskey, _ := bodyj.Get("NEPTUNE_ACCESS_KEY").String()
	secretkey, _ := bodyj.Get("NEPTUNE_SECRET_KEY").String()
	region, _ := bodyj.Get("NEPTUNE_REGION").String()
	splitURL := strings.Split(databaseurl, ".")
	specname := "neptune:" + splitURL[0]

	var neptune structs.Neptunespec
	neptune.Spec = specname
	neptune.NeptuneDatabaseURL = databaseurl
	neptune.NeptuneAccessKey = accesskey
	neptune.NeptuneSecretKey = secretkey
	neptune.NeptuneRegion = region
	r.JSON(201, neptune)
}

func DeleteNeptune(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}

func TagNeptune(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/tag", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	var brokerresponse structs.Brokerresponse
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		bb, _ := ioutil.ReadAll(resp.Body)
		_ = json.Unmarshal(bb, &brokerresponse)
		message.Status = 201
		message.Message = brokerresponse.Response
		r.JSON(message.Status, message)
	}
}

func GetNeptuneVars(servicename string) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("NEPTUNE_BROKER_URL")+"/v1/neptune/url/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return config, nil
}
