package service

import (
    "database/sql"
    "github.com/lib/pq"
	"bytes"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"os"
	"fmt"
	"errors"
    "io/ioutil"
	structs "region-api/structs"
	utils "region-api/utils"
)

type Acl struct {
    Id            string `json:"id"`
    Cluster       string `json:"cluster"`
    Topic         string `json:"topic"`
    User          string `json:"user"`
    Space         string `json:"space,omitempty"`
    Appname       string `json:"app,omitempty"`
    Role          string `json:"role"`
}

type AclsResponse struct {
    Acls  []Acl `json:"acls"`
}

func ProvisionKafkaV1(spec structs.Provisionspec, berr binding.Errors, r render.Render) {
    if berr != nil {
        utils.ReportInvalidRequest(berr[0].Message, r)
        return
    }

    client := &http.Client{}
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+spec.Plan +"/user", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    defer resp.Body.Close()

    var kafka structs.Kafkaspec
    var creds structs.KafkaAclCredentials

    bodybytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    err = json.Unmarshal(bodybytes, &creds)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    kafka.Spec = "kafka:" + creds.AclCredentials.Username
    r.JSON(201, kafka)
}

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

func GetConfigsV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/configs", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetConfigV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    name := params["name"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/configs/"+name, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}


func GetSchemasV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/schemas", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetSchemaV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    schema := params["schema"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/schemas/"+schema, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func CreateTopicKeyMappingV1(spec structs.TopicKeyMapping, params martini.Params, berr binding.Errors, r render.Render) {
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
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/topic-key-mapping", bytes.NewBuffer(jsonStr))
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

func CreateTopicSchemaMappingV1(spec structs.TopicSchemaMapping, params martini.Params, berr binding.Errors, r render.Render) {
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
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/topic-schema-mapping", bytes.NewBuffer(jsonStr))
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

func CreateAclV1(db *sql.DB, request structs.AclRequest, params martini.Params, berr binding.Errors, r render.Render) {
    cluster := params["cluster"]

    if berr != nil {
        utils.ReportInvalidRequest(berr[0].Message, r)
        return
    }

    username := getUserName(db, request.Appname, request.Space)
    if(username == "") {
        utils.ReportInvalidRequest("Application does not have a bind to any kafka instance.", r)
        return
    }

    request.User = username
    client := &http.Client{}
    str, err := json.Marshal(request)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    jsonStr := []byte(string(str))
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/acls", bytes.NewBuffer(jsonStr))
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

func getUserName(db *sql.DB, app string, space string) string {
    var username string
    stmt, dberr := db.Prepare("select  bindname from appbindings where bindtype = $1 and appname = $2 and space = $3")
    if dberr != nil {
        fmt.Println(dberr)
        return username
    }
    defer stmt.Close()
    rows, err := stmt.Query("kafka", app, space)

    defer rows.Close()

    for rows.Next() {
        err := rows.Scan(&username)
        if err != nil {
            fmt.Println(err)
            return username
        }
    }
    err = rows.Err()
    if err != nil {
        fmt.Println(err)
        return username
    }

    return username
}

func DeleteAclV1(params martini.Params, r render.Render) {
    id := params["id"]

    client := &http.Client{}
    req, err := http.NewRequest("DELETE", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/acls/"+id, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetAclsV1(db *sql.DB, request *http.Request, params martini.Params, r render.Render) {
    cluster := params["cluster"]
    topic := request.URL.Query().Get("topic")
    var aclsResponse AclsResponse

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/acls?topic="+topic, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    if resp.StatusCode > 299 || resp.StatusCode < 200 {
        bodyj, _ := simplejson.NewFromReader(resp.Body)
        r.JSON(resp.StatusCode, bodyj)
        return
    }

    bodybytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    err = json.Unmarshal(bodybytes, &aclsResponse)
    if err != nil {
        utils.ReportError(err, r)
        return
    }
    getAppBindings(db, aclsResponse.Acls)
    r.JSON(resp.StatusCode, aclsResponse)
}

func getAppBindings(db *sql.DB, acls []Acl) {
    aclIndexMap := make(map[string]int)
    users := make([]string, len(acls))

    for i := 0; i < len(acls); i +=1 {
        aclIndexMap[acls[i].User] = i
        users[i] = acls[i].User
    }

    stmt, dberr := db.Prepare("select bindname, appname, space from appbindings where bindtype = $1 and bindname = Any($2)")
    if dberr != nil {
        fmt.Println(dberr)
        return
    }
    defer stmt.Close()
    rows, err := stmt.Query("kafka", pq.Array(users))
    if err != nil {
        fmt.Println(err)
        return
    }

    defer rows.Close()

    for rows.Next() {
        var user string
        var appname string
        var space string
        err := rows.Scan(&user, &appname, &space)
        if err != nil {
            fmt.Println(err)
            return
        }

        if(appname != "") {
            i := aclIndexMap[user]
            acls[i].Appname = appname
            acls[i].Space = space
        }
    }

}

func GetKafkaPlansV1(params martini.Params, r render.Render) {
    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/clusters", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func DeleteTopicV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    topic := params["topic"]

    client := &http.Client{}
    req, err := http.NewRequest("DELETE", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/topics/"+topic, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func DeleteKafkaV1(params martini.Params, r render.Render) {
    servicename := params["servicename"]
    client := &http.Client{}
    req, err := http.NewRequest("DELETE", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/user/"+servicename, nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

//Getkafkavars  centralized
func Getkafkavars(db *sql.DB, appname string, space string) (error, map[string]interface{}) {
    username := getUserName(db, appname, space)
    config := make(map[string]interface{})
    if(username == "") {
        return errors.New("Failure to obtain kafka username binding"), config
    }

	client := &http.Client{}
	req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/credentials/"+username, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err, config
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New("Failure to obtain kafka URL"), config
	}
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	config, _ = bodyj.Map()
	return nil, config
}

func GetConsumerGroupsV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/consumer-groups", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetConsumerGroupOffsetsV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    consumerGroup := params["consumerGroupName"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/consumer-groups/"+consumerGroup+"/offsets", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func GetConsumerGroupMembersV1(params martini.Params, r render.Render) {
    cluster := params["cluster"]
    consumerGroup := params["consumerGroupName"]

    client := &http.Client{}
    req, err := http.NewRequest("GET", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/consumer-groups/"+consumerGroup+"/members", nil)
    resp, err := client.Do(req)
    if err != nil {
        utils.ReportError(err, r)
        return
    }

    defer resp.Body.Close()
    bodyj, _ := simplejson.NewFromReader(resp.Body)
    r.JSON(resp.StatusCode, bodyj)
}

func SeekConsumerGroupV1(spec structs.KafkaConsumerGroupSeekRequest, params martini.Params, berr binding.Errors, r render.Render) {
    cluster := params["cluster"]
    consumerGroup := params["consumerGroupName"]

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
    req, err := http.NewRequest("POST", os.Getenv("KAFKA_BROKER_URL")+"/v1/kafka/cluster/"+cluster+"/consumer-groups/"+consumerGroup+"/seek", bytes.NewBuffer(jsonStr))
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
