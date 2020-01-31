package space

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"os"
	"region-api/app"
	"region-api/config"
	"region-api/structs"
	"region-api/utils"
	"strings"
	"testing"
)

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Post("/v1/app", binding.Json(structs.Appspec{}), app.Createapp)  //createapp.go
	m.Patch("/v1/app", binding.Json(structs.Appspec{}), app.Updateapp) //updateapp.go
	m.Get("/v1/apps", app.Listapps)                                    //listapps.go
	m.Get("/v1/app/:appname", app.Describeapp)                         //describeapp.go
	m.Delete("/v1/app/:appname", app.Deleteapp)                        //deleteapp.go
	m.Get("/v1/apps/plans", app.GetPlans)                              //plan.go

	m.Post("/v1/app/deploy", binding.Json(structs.Deployspec{}), app.Deployment) //deployment.go

	m.Post("/v1/app/bind", binding.Json(structs.Bindspec{}), app.Createbind)                       //createbind.go
	m.Delete("/v1/app/:appname/bind/:bindspec", app.Unbindapp)                                     //unbindapp.go
	m.Post("/v1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), app.Createbind) //createbind.go
	m.Delete("/v1/space/:space/app/:appname/bind/**", app.Unbindapp)                               //unbindapp.go

	m.Get("/v1/space/:space/app/:app/instance", app.GetInstances)                   //instance.go
	m.Get("/v1/space/:space/app/:appname/instance/:instanceid/log", app.GetAppLogs) //instance.go
	m.Delete("/v1/space/:space/app/:app/instance/:instanceid", app.DeleteInstance)  //instance.go

	m.Post("/v1/space/:space/app/:app/rollback/:revision", app.Rollback) //rollback.,go

	m.Get("/v1/space/:space/apps", app.Describespace)              //describeapp.go
	m.Get("/v1/space/:space/app/:appname", app.DescribeappInSpace) //describeapp.go

	m.Post("/v1/space/:space/app/:appname/restart", app.Restart)           //restart.go
	m.Get("/v1/space/:space/app/:app/status", app.Spaceappstatus)          //status.go
	m.Get("/v1/kube/podstatus/:space/:app", app.PodStatus)                 //status.go

	m.Get("/v1/space/:space/app/:app/subscribers", app.GetSubscribersDB)                                             //subscriber.go
	m.Delete("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), app.RemoveSubscriberDB) //subscriber.go
	m.Post("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), app.AddSubscriberDB)      //subscriber.go

	//Helper endpoints for creating an app in a space these are not tested here
	m.Delete("/v1/space/:space/app/:app", DeleteApp)
	m.Post("/v1/space", binding.Json(structs.Spacespec{}), Createspace)
	m.Get("/v1/spaces", Listspaces)
	m.Get("/v1/space/:space", Space)

	m.Delete("/v1/space/:space", binding.Json(structs.Spacespec{}), Deletespace)
	m.Put("/v1/space/:space/tags", binding.Json(structs.Spacespec{}), UpdateSpaceTags)
	m.Put("/v1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), AddApp)
	m.Put("/v1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), UpdateAppHealthCheck)
	m.Delete("/v1/space/:space/app/:app/healthcheck", DeleteAppHealthCheck)
	m.Put("/v1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), UpdateAppPlan)
	m.Put("/v1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), ScaleApp)

	m.Post("/v1/config/set", binding.Json(structs.Setspec{}), config.Createset)
	m.Post("/v1/config/set/configvar", binding.Json([]structs.Varspec{}), config.Addvars)
	m.Delete("/v1/config/set/:setname", config.Deleteset)

	return m
}

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	httpMartini := Server()
	httpMartini.Map(pool)
	return httpMartini
}

func sendRequest(m *martini.ClassicMartini, method string, url string, payload interface{}) *httptest.ResponseRecorder {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(payload); err != nil {
		panic(err)
	}
	r, _ := http.NewRequest(strings.ToUpper(method), url, b)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	return w
}

func TestCreatingDeletingSpaces(t *testing.T) {
	m := Init()
	Convey("Given we need a space", t, func() {
		Convey("Ensure an incorrect space returns a 404", func() {
			w := sendRequest(m, "get", "/v1/space/foobardoesnotexistextan", nil)
			So(w.Code, ShouldEqual, http.StatusNotFound)
			So(w.Body.String(), ShouldContainSubstring, "The specified space does not exist")
		})
		Convey("Ensure we can create a space", func() {
			w := sendRequest(m, "post", "/v1/space", structs.Spacespec{Name: "alamotestspace", Internal: false, ComplianceTags: "", Stack: "ds1"})
			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w.Body.String(), ShouldContainSubstring, "space created")
		})
		Convey("Ensure we cant create a space twice", func() {
			w := sendRequest(m, "post", "/v1/space", structs.Spacespec{Name: "alamotestspace", Internal: false, ComplianceTags: "", Stack: "ds1"})
			So(w.Code, ShouldEqual, http.StatusBadRequest)
			So(w.Body.String(), ShouldContainSubstring, "The specified space is already taken.")
		})
		Convey("We should be able to see the spaces", func() {
			w := sendRequest(m, "get", "/v1/spaces", nil)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "alamotestspace")
		})
		Convey("We should be able to get the test space", func() {
			w := sendRequest(m, "get", "/v1/space/alamotestspace", nil)
			So(w.Code, ShouldEqual, http.StatusOK)
			fmt.Println(w.Body.String())
		})
		Convey("Given we should be able to delete a space", func() {
			w := sendRequest(m, "delete", "/v1/space/alamotestspace", nil)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "space deleted")
		})
	})
}
