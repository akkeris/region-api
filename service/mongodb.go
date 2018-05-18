package service

/*
 * Project: alamo-api
 * Package: service
 * Module: mongodb
 *
 * Author:  ned.hanks
 *
 */

import (
    "net/http"
    "os"
    "strings"
    "bytes"
    "encoding/json"

    "../structs"
    "../utils"

    "github.com/bitly/go-simplejson"
    "github.com/go-martini/martini"
    "github.com/martini-contrib/render"
    "github.com/martini-contrib/binding"
    "log"
)

func setspecname(dburl string) (string) {
    parts := strings.Split(dburl, "/")
    specname := "mongodb:" + parts[len(parts)-1]
    return specname
}

func GetmongodbplansV1(_ martini.Params, r render.Render) {
    var plans []structs.Planspec

    client := &http.Client{}
    req, err := http.NewRequest("GET", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/plans", nil)
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
    r.JSON(http.StatusOK, plans)
}

func GetmongodburlV1(params martini.Params, r render.Render) {
    var mongodb structs.Mongodbspec

    servicename := params["servicename"]
    client := &http.Client{}
    req, err := http.NewRequest("GET", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/url/"+servicename, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    bodyj, _ := simplejson.NewFromReader(resp.Body)
    databaseurl, _ := bodyj.Get("DATABASE_URL").String()

    mongodb.DatabaseUrl = databaseurl
    mongodb.Spec = setspecname(databaseurl)

    r.JSON(200, mongodb)
}

func ProvisionmongodbV1(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
    var mongodb structs.Mongodbspec

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

    req, err := http.NewRequest("POST", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/instance", bytes.NewBuffer(jsonStr))
    req.Header.Set("Content-Type", "application/json")
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        var message structs.Messagespec
        message.Status = resp.StatusCode
        message.Message = err.Error()
        r.JSON(resp.StatusCode, message)
        return
    }

    bodyj, _ := simplejson.NewFromReader(resp.Body)
    databaseurl, _ := bodyj.Get("DATABASE_URL").String()

    mongodb.DatabaseUrl = databaseurl
    mongodb.Spec = setspecname(databaseurl)
    r.JSON(http.StatusCreated, mongodb)
}

//Deletepostgresonprem  centralized
func DeletemongodbV1(params martini.Params, r render.Render) {
    servicename := params["servicename"]
    client := &http.Client{}
    req, err := http.NewRequest("DELETE", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/instance/"+servicename, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        var message structs.Messagespec
        message.Status = resp.StatusCode
        message.Message = err.Error()
        r.JSON(resp.StatusCode, message)
        return
    }

    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(200, bodyj)
}

func GetmongodbV1(params martini.Params, r render.Render) {
    var message structs.Messagespec

    client := &http.Client{}
    req, err := http.NewRequest("GET", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/" + params["servicename"], nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    log.Printf("(service.GetmongodbV1) resp.StatusCode: %d\n", resp.StatusCode)
    if resp.StatusCode != http.StatusOK {
        log.Printf("(service.GetmongodbV1) send error %s\n", resp.Status)
        message.Status = resp.StatusCode
        message.Message = resp.Status
        r.JSON(resp.StatusCode, message)
        return
    }

    bodyj, _ := simplejson.NewFromReader(resp.Body)

    r.JSON(http.StatusOK, bodyj)
}


func Getmongodbvars(servicename string) (m map[string]string, e error) {
    m = make(map[string]string)
    client := &http.Client{}
    req, err := http.NewRequest("GET", "http://"+os.Getenv("MONGODB_BROKER_URL")+"/v1/mongodb/" + servicename, nil)
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("(service.Getmongodbvars) send error %v\n", err)
        return m , err
    }
    defer resp.Body.Close()

    log.Printf("(service.Getmongodbvars) resp.StatusCode: %d\n", resp.StatusCode)
    if resp.StatusCode != http.StatusOK {
        log.Printf("(service.Getmongodbvars) send error %v\n", resp.Status)
        return m, err
    }

    bodyj, _ := simplejson.NewFromReader(resp.Body)
    m["DATABASE_URL"], err =bodyj.Get("DATABASE_URL").String()
    if err != nil {
      log.Printf("(service.Getmongodbvars) send error %v\n", err)
      return m, err
    }
    return m, nil
}
