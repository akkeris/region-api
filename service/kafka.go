package service

import (
	"bytes"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
)

func ProvisionTopicV1(spec structs.KafkaTopic, params martini.Params, berr binding.Errors, r render.Render) {
    cluster := params["cluster"]

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
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster +"/topic", bytes.NewBuffer(jsonStr))
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetTopicsV1(r render.Render) {
    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/topics", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetTopicV1(params martini.Params, r render.Render) {
    topic := params["topic"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/topics/"+topic, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}
