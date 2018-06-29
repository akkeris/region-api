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
	"errors"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Getpostgresplans  centralized
func GetpostgresplansV1(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/postgres/plans", nil)
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

//Getpostgresurl  centralized
// ~~ DEPRECREATED ~~
func GetpostgresurlV1(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var postgres structs.Postgresspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/postgres/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	parts := strings.Split(databaseurl, "/")
	specname := "postgres:" + parts[len(parts)-1]
	postgres.DatabaseUrl = databaseurl
	postgres.Spec = specname
	r.JSON(200, postgres)
}

//Provisionpostgres  centralized
func ProvisionpostgresV1(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/postgres/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()

	parts := strings.Split(databaseurl, "/")
	specname := "postgres:" + parts[len(parts)-1]

	var postgres structs.Postgresspec
	postgres.DatabaseUrl = databaseurl
	postgres.Spec = specname
	r.JSON(201, postgres)
}

//Deletepostgres  centralized
func DeletepostgresV1(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/postgres/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}

//Tagpostgres  centralized
func TagpostgresV1(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/tag", bytes.NewBuffer(jsonStr))
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

//Getpostgresvars  centralized
func GetpostgresvarsV1(servicename string) map[string]interface{} {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v1/postgres/url/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return config
}

func GetPostgresPlansV2(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/plans", nil)
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

func GetPostgresUrlV2(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var postgres structs.Postgresspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	parts := strings.Split(databaseurl, "/")
	specname := "postgres:" + parts[len(parts)-1]
	postgres.DatabaseUrl = databaseurl
	postgres.Spec = specname
	r.JSON(200, postgres)
}

func ProvisionPostgresV2(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()

	parts := strings.Split(databaseurl, "/")
	specname := "postgres:" + parts[len(parts)-1]

	var postgres structs.Postgresspec
	postgres.DatabaseUrl = databaseurl
	postgres.Spec = specname
	r.JSON(201, postgres)
}

func DeletePostgresV2(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}

func TagPostgresV2(spec structs.Tagspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + spec.Resource + "/tags", bytes.NewBuffer(jsonStr))
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

func GetPostgresVarsV2(servicename string) (error, map[string]interface{}) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		return err, map[string]interface{}{}
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Failure to obtain MEMCACHED_URL"), map[string]interface{}{}
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	return nil, map[string]interface{}{"DATABASE_URL":databaseurl}
}

func ListPostgresBackupsV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/backups", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresBackupV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/backups/" + params["backup"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func CreatePostgresBackupV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/backups", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(201, bodyj)
}

func RestorePostgresBackupV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/backups/" + params["backup"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func ListPostgresLogsV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/logs", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresLogsV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/logs/" + params["dir"] + "/" + params["file"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()

	r.Text(200, string(body))
}

func RestartPostgresV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func CreatePostgresRoleV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/roles", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func DeletePostgresRoleV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/roles/" + params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func ListPostgresRolesV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/roles", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func RotatePostgresRoleV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/roles/" + params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresRoleV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"] + "/roles/" + params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresV2(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRES_BROKER_URL")+"/v2/postgres/" + params["servicename"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}
