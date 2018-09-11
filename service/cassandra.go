package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

func GetCassandraPlans(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("CASSANDRA_BROKER_URL")+"/v1/cassandra/plans", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
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

func GetCassandraURL(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var cassandra structs.Cassandraspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("CASSANDRA_BROKER_URL")+"/v1/cassandra/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}
	bodybytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = json.Unmarshal(bodybytes, &cassandra)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	cassandra.Spec = "cassandra:" + cassandra.Keyspace
	r.JSON(200, cassandra)
}

func ProvisionCassandra(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("CASSANDRA_BROKER_URL")+"/v1/cassandra/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	var cassandra structs.Cassandraspec
	bodybytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = json.Unmarshal(bodybytes, &cassandra)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	cassandra.Spec = "cassandra:" + cassandra.Keyspace
	r.JSON(201, cassandra)
}

func DeleteCassandra(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("CASSANDRA_BROKER_URL")+"/v1/cassandra/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		utils.ReportError(errors.New(resp.Status), r)
		return
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}


func GetCassandraVars(servicename string) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("CASSANDRA_BROKER_URL")+"/v1/cassandra/url/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return nil, errors.New(resp.Status)
	}

	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return config, nil
}
