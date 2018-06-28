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

	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

//Getpostgresonpremplans  centralized
func GetpostgresonpremplansV1(params martini.Params, r render.Render) {
	var plans []structs.Planspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/plans", nil)
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

//Getpostgresonpremurl  centralized
func GetpostgresonpremurlV1(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	var postgresonprem structs.Postgresspec
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/url/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	parts := strings.Split(databaseurl, "/")
	specname := "postgresonprem:" + parts[len(parts)-1]
	postgresonprem.DatabaseUrl = databaseurl
	postgresonprem.Spec = specname
	r.JSON(200, postgresonprem)
}

//Provisionpostgresonprem  centralized
func ProvisionpostgresonpremV1(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
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
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/instance", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()

	parts := strings.Split(databaseurl, "/")
	specname := "postgresonprem:" + parts[len(parts)-1]

	var postgresonprem structs.Postgresspec
	postgresonprem.DatabaseUrl = databaseurl
	postgresonprem.Spec = specname
	r.JSON(201, postgresonprem)
}

//Deletepostgresonprem  centralized
func DeletepostgresonpremV1(params martini.Params, r render.Render) {
	servicename := params["servicename"]
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/instance/"+servicename, nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	r.JSON(200, bodyj)
}

//Getpostgresonpremvars  centralized
func GetpostgresonpremvarsV1(servicename string) map[string]interface{} {
	config := make(map[string]interface{})
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/url/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return config
}

func GetPostgresonpremVarsV1(servicename string) map[string]interface{} {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+servicename, nil)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	databaseurl, _ := bodyj.Get("DATABASE_URL").String()
	return map[string]interface{}{"DATABASE_URL": databaseurl}
}

func CreatePostgresonpremRoleV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/roles", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(201, bodyj)
}

func DeletePostgresonpremRoleV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/roles/"+params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func ListPostgresonpremRolesV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/roles", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func RotatePostgresonpremRoleV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/roles/"+params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresonpremRoleV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/roles/"+params["role"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func GetPostgresonpremV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(200, bodyj)
}

func ListPostgresonpremBackupsV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/backups", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(resp.StatusCode, bodyj)
}

func GetPostgresonpremBackupV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/backups/"+params["backup"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(resp.StatusCode, bodyj)
}

func CreatePostgresonpremBackupV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/backups", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(201, bodyj)
}

func RestorePostgresonpremBackupV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/backups/"+params["backup"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(resp.StatusCode, bodyj)
}

func ListPostgresonpremLogsV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/logs", nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(resp.StatusCode, bodyj)
}

func GetPostgresonpremLogsV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"]+"/logs/"+params["dir"]+"/"+params["file"], nil)
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

	r.Text(resp.StatusCode, string(body))
}

func RestartPostgresonpremV1(params martini.Params, r render.Render) {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://"+os.Getenv("POSTGRESONPREM_BROKER_URL")+"/v1/postgresonprem/"+params["servicename"], nil)
	resp, err := client.Do(req)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)

	r.JSON(resp.StatusCode, bodyj)
}
