package app

import (
	"bytes"
	"net/http"
	"os"
	"region-api/config"
	"region-api/maintenance"
	"region-api/space"
	"region-api/structs"
	"region-api/utils"
	"region-api/service"
	"testing"
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
	"net/http/httptest"
	"strings"
	"log"
	"strconv"
)

func ServerAppConfig() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Post("/v1/app", binding.Json(structs.Appspec{}), Createapp)  //createapp.go
	m.Patch("/v1/app", binding.Json(structs.Appspec{}), Updateapp) //updateapp.go
	m.Get("/v1/apps", Listapps)                                    //listapps.go
	m.Get("/v1/app/:appname", Describeapp)                         //describeapp.go
	m.Delete("/v1/app/:appname", Deleteapp)                        //deleteapp.go
	m.Get("/v1/apps/plans", GetPlans)                              //plan.go

	m.Post("/v1/app/deploy", binding.Json(structs.Deployspec{}), Deployment) //deployment.go

	m.Post("/v1/app/bind", binding.Json(structs.Bindspec{}), Createbind)                       //createbind.go
	m.Delete("/v1/app/:appname/bind/:bindspec", Unbindapp)                                     //unbindapp.go
	m.Post("/v1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), Createbind) //createbind.go
	m.Delete("/v1/space/:space/app/:appname/bind/**", Unbindapp)                               //unbindapp.go

	m.Get("/v1/space/:space/app/:app/instance", GetInstances)                   //instance.go
	m.Get("/v1/space/:space/app/:appname/instance/:instanceid/log", GetAppLogs) //instance.go
	m.Delete("/v1/space/:space/app/:app/instance/:instanceid", DeleteInstance)  //instance.go

	m.Post("/v1/space/:space/app/:app/rollback/:revision", Rollback) //rollback.,go

	m.Get("/v1/space/:space/apps", Describespace)              //describeapp.go
	m.Get("/v1/space/:space/app/:appname", DescribeappInSpace) //describeapp.go

	m.Get("/v1/space/:space/app/:appname/configvars", GetAllConfigVars)
	m.Get("/v1/space/:space/app/:appname/deployments", GetDeployments) //replicasets.go
	m.Post("/v1/space/:space/app/:appname/restart", Restart)           //restart.go
	m.Get("/v1/space/:space/app/:app/status", Spaceappstatus)          //status.go
	m.Get("/v1/kube/podstatus/:space/:app", PodStatus)                 //status.go

	m.Get("/v1/space/:space/app/:app/subscribers", GetSubscribersDB)                                             //subscriber.go
	m.Delete("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), RemoveSubscriberDB) //subscriber.go
	m.Post("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), AddSubscriberDB)      //subscriber.go

	//Helper endpoints for creating an app in a space these are not tested here
	m.Delete("/v1/space/:space/app/:app", space.DeleteApp)
	m.Post("/v1/space", binding.Json(structs.Spacespec{}), space.Createspace)
	m.Put("/v1/space/:space/tags", binding.Json(structs.Spacespec{}), space.UpdateSpaceTags)
	m.Put("/v1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), space.AddApp)
	m.Put("/v1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), space.UpdateAppHealthCheck)
	m.Delete("/v1/space/:space/app/:app/healthcheck", space.DeleteAppHealthCheck)
	m.Put("/v1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), space.UpdateAppPlan)
	m.Put("/v1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), space.ScaleApp)

	m.Get("/v1/space/:space/app/:app/maintenance", maintenance.MaintenancePageStatus)
	m.Post("/v1/space/:space/app/:app/maintenance", maintenance.EnableMaintenancePage)
	m.Delete("/v1/space/:space/app/:app/maintenance", maintenance.DisableMaintenancePage)

	m.Post("/v1/config/set", binding.Json(structs.Setspec{}), config.Createset)
	m.Post("/v1/config/set/configvar", binding.Json([]structs.Varspec{}), config.Addvars)
	m.Delete("/v1/config/set/:setname", config.Deleteset)

	m.Post("/v2/services/postgres", binding.Json(structs.Provisionspec{}), service.ProvisionPostgresV2)
	m.Delete("/v2/services/postgres/:servicename", service.DeletePostgresV2)
	m.Post("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", binding.Json(structs.Bindmapspec{}), Createbindmap)
	m.Get("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", Getbindmaps)
	m.Delete("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname/:mapid", Deletebindmap)
	return m
}

var debug bool = false
var httpServer *martini.ClassicMartini = nil

type Response struct {
	Body string
	Status string
	StatusCode int
}

func Request(method string, path string, payload interface{}) (r *Response, e error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(strings.ToUpper(method), path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-type", "application/json")

	if debug {
		log.Printf("-> test: %s %s with headers [%s] with payload [%s]\n", method, path, req.Header, body)
	}
	resp := httptest.NewRecorder()
	httpServer.ServeHTTP(resp, req)
	if debug {
		log.Printf("<- test: %s %s - %d\n", method, path, resp.Code)
	}
	return &Response{Body: resp.Body.String(), Status: strconv.Itoa(resp.Code), StatusCode: resp.Code}, nil
}

func InitAppConfig() {
	if os.Getenv("TEST_DEBUG") == "true" {
		debug = true
	}
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	httpServer = ServerAppConfig()
	httpServer.Map(pool)
}

func TestConfigVarMappings(t *testing.T) {
	InitAppConfig() 
	appname := "regionapitest"
	space := "default"
	Convey("Given we have an app with a database attached", t, func() {
		Request("DELETE", "/v1/space/" + space + "/app/" + appname, nil)
		Request("DELETE", "/v1/app/" + appname, nil)

		res, err := Request("POST", "/v1/app", structs.Appspec{Name: appname, Port: 8080})
		So(err, ShouldEqual, nil)
		So(res.StatusCode, ShouldEqual, http.StatusCreated)
		res, err = Request("PUT", "/v1/space/" + space + "/app/" + appname, structs.Spaceappspec{Appname: appname, Space: space, Instances: 1, Plan: "scout"})
		So(err, ShouldEqual, nil)
		So(res.StatusCode, ShouldEqual, http.StatusCreated)
		res, err = Request("POST", "/v1/app/deploy", structs.Deployspec{AppName: appname, Space: space, Image: "docker.io/akkeris/apachetest:latest"})
		So(err, ShouldEqual, nil)
		So(res.StatusCode, ShouldEqual, http.StatusCreated)
		res, err = Request("POST", "/v2/services/postgres", structs.Provisionspec{Plan: "micro", Billingcode: "test"})
		So(err, ShouldEqual, nil)
		So(res.StatusCode, ShouldEqual, http.StatusCreated)
		dbinstance := structs.Postgresspec{}
		err = json.Unmarshal([]byte(res.Body), &dbinstance)
		So(err, ShouldEqual, nil)

		spec := strings.Split(dbinstance.Spec, ":")
		So(len(spec), ShouldEqual, 2)
		res, err = Request("POST", "/v1/space/" + space + "/app/" + appname +"/bind", structs.Bindspec{Bindname: spec[1], Bindtype: "postgres", Space: space, App: appname})
		So(err, ShouldEqual, nil)
		So(res.StatusCode, ShouldEqual, http.StatusCreated)

		rename_map_id := ""
		copy_map_id := ""

		Convey("The config var DATABASE_URL should be present in config vars", func() {
			res, err = Request("GET", "/v1/space/" + space + "/app/" + appname + "/configvars", nil)
			So(err, ShouldEqual, nil)
			So(res.Body, ShouldContainSubstring, "DATABASE_URL")

			Convey("With a rename added, DATABASE_URL should not be present.", func() {
				res, err = Request("POST", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1], structs.Bindmapspec{App:appname, Space:space, Bindtype:spec[0], Bindname:spec[1], Action:"rename", VarName:"DATABASE_URL", NewName:"MY_URL"})
				So(err, ShouldEqual, nil)
				rename := structs.Messagespec{}
				err = json.Unmarshal([]byte(res.Body), &rename)
				So(err, ShouldEqual, nil)
				rename_map_id = rename.Message
				log.Println("rename_map_id: " + rename_map_id)
				So(rename_map_id, ShouldNotEqual, "")
				res, err = Request("GET", "/v1/space/" + space + "/app/" + appname + "/configvars", nil)
				So(err, ShouldEqual, nil)
				So(res.Body, ShouldNotContainSubstring, "DATABASE_URL")
				So(res.Body, ShouldContainSubstring, "MY_URL")

				Convey("With a copy added, DATABASE_URL should not be present but rename and copy should.", func() {
					res, err = Request("POST", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1], structs.Bindmapspec{App:appname, Space:space, Bindtype:spec[0], Bindname:spec[1], Action:"copy", VarName:"DATABASE_URL", NewName:"FOOFOO_URL"})
					So(err, ShouldEqual, nil)

					copys := structs.Messagespec{}
					err = json.Unmarshal([]byte(res.Body), &copys)
					So(err, ShouldEqual, nil)
					copy_map_id = copys.Message
					So(copy_map_id, ShouldNotEqual, "")
					res, err = Request("GET", "http://localhost:5000/v1/space/" + space + "/app/" + appname + "/configvars", nil)
					So(err, ShouldEqual, nil)
					So(res.Body, ShouldNotContainSubstring, "DATABASE_URL")
					So(res.Body, ShouldContainSubstring, "MY_URL")
					So(res.Body, ShouldContainSubstring, "FOOFOO_URL")
					Convey("With copy rename is removed DATABASE_URL returns to normal.", func() {
						res, err = Request("DELETE", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1] + "/" + rename_map_id, nil)
						So(err, ShouldEqual, nil)
						res, err = Request("DELETE", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1] + "/" + copy_map_id, nil)
						So(err, ShouldEqual, nil)
						res, err = Request("GET", "/v1/space/" + space + "/app/" + appname + "/configvars", nil)
						So(err, ShouldEqual, nil)
						So(res.Body, ShouldContainSubstring, "DATABASE_URL")
						So(res.Body, ShouldNotContainSubstring, "MY_URL")
						So(res.Body, ShouldNotContainSubstring, "FOOFOO_URL")
						Convey("With delete is added DATABASE_URL should not be there.", func() {
							res, err = Request("POST", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1], structs.Bindmapspec{App:appname, Space:space, Bindtype:spec[0], Bindname:spec[1], Action:"delete", VarName:"DATABASE_URL"})
							So(err, ShouldEqual, nil)
							delete_map := structs.Messagespec{}
							err = json.Unmarshal([]byte(res.Body), &delete_map)
							So(err, ShouldEqual, nil)
							delete_map_id := delete_map.Message
							So(delete_map_id, ShouldNotEqual, "")
							res, err = Request("GET", "/v1/space/" + space + "/app/" + appname + "/configvars", nil)
							So(err, ShouldEqual, nil)
							So(res.Body, ShouldNotContainSubstring, "DATABASE_URL")
							So(res.Body, ShouldNotContainSubstring, "MY_URL")
							So(res.Body, ShouldNotContainSubstring, "FOOFOO_URL")
							res, err = Request("DELETE", "/v1/space/" + space + "/app/" + appname + "/bindmap/" + spec[0] + "/" + spec[1] + "/" + delete_map_id, nil)
							So(err, ShouldEqual, nil)
							res, err = Request("GET", "/v1/space/" + space + "/app/" + appname + "/configvars", nil)
							So(err, ShouldEqual, nil)
							So(res.Body, ShouldContainSubstring, "DATABASE_URL")
							So(res.Body, ShouldNotContainSubstring, "MY_URL")
							So(res.Body, ShouldNotContainSubstring, "FOOFOO_URL")
						})
					})
				})	
			})
		})

		Reset(func() {
			Request("DELETE", "/v1/space/" + space + "/app/" + appname + "/bind/" + dbinstance.Spec, nil)
			Request("DELETE", "/v2/services/postgres/" + spec[1], nil)
			Request("DELETE", "/v1/space/" + space + "/app/" + appname, nil)
			Request("DELETE", "/v1/app/" + appname, nil)
		})
	})	
}






