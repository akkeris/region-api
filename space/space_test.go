package space

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"region-api/app"
	"region-api/config"
	"region-api/structs"
	"region-api/utils"
	"strings"
	"testing"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
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

	m.Post("/v1/space/:space/app/:appname/restart", app.Restart)  //restart.go
	m.Get("/v1/space/:space/app/:app/status", app.Spaceappstatus) //status.go
	m.Get("/v1/kube/podstatus/:space/:app", app.PodStatus)        //status.go

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

	// V2 ENDPOINTS
	m.Get("/v2beta1/space/:space/deployments", DescribeSpaceV2)
	m.Get("/v2beta1/space/:space/deployment/:deployment", DescribeDeploymentV2)
	m.Get("/v2beta1/space/:space/deployment/:deployment/configvars", GetAllConfigVarsV2)
	m.Post("/v2beta1/space/:space/deployment/:deployment", binding.Json(structs.AppDeploymentSpec{}), AddDeploymentV2)
	m.Put("/v2beta1/space/:space/deployment/:deployment/deploy", binding.Json(structs.DeploySpecV2{}), DeploymentV2Handler)
	m.Delete("/v2beta1/space/:space/deployment/:deployment", DeleteDeploymentV2Handler)
	m.Patch("/v2beta1/space/:space/deployment/:deployment/healthcheck", binding.Json(structs.AppDeploymentSpec{}), UpdateDeploymentHealthCheckV2)
	m.Delete("/v2beta1/space/:space/deployment/:deployment/healthcheck", DeleteDeploymentHealthCheckV2)
	m.Patch("/v2beta1/space/:space/deployment/:deployment/plan", binding.Json(structs.AppDeploymentSpec{}), UpdateDeploymentPlanV2)
	m.Patch("/v2beta1/space/:space/deployment/:deployment/scale", binding.Json(structs.AppDeploymentSpec{}), ScaleDeploymentV2)

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

// V2

func TestDeploymentsV2(t *testing.T) {
	testDeploymentName := "gotest"
	testDeploymentSpace := "gotest"
	testDeploymentID := structs.PrettyNullString{sql.NullString{
		String: "12345678-1234-1234-1234-123456789abc",
		Valid:  true,
	}}
	testInstances := structs.PrettyNullInt64{sql.NullInt64{
		Int64: 1,
		Valid: true,
	}}
	testHealthcheck := structs.PrettyNullString{sql.NullString{
		String: "/",
		Valid:  true,
	}}
	m := Init()

	// Given we want a deployment
	// When it is successfully created
	// 		It should exist in a list of deployments
	// 		It should have valid info
	//		It shouldn't collide with an already created app
	// When we update the app
	//		- update port
	//		-

	Convey("Given we have a Deployment", t, func() {
		Convey("When a deployment is invalid", func() {
			Convey("it should require a name", func() {
				testDeployment := structs.DeploySpecV2{Space: testDeploymentSpace, Image: "docker.io/akkeris/apachetest:latest"}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/deploy", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Deployment Name can not be blank")
			})
			Convey("it should require a space", func() {
				testDeployment := structs.DeploySpecV2{Name: testDeploymentName, Image: "docker.io/akkeris/apachetest:latest"}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/deploy", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Space Name can not be blank")
			})
			Convey("it should require an image", func() {
				testDeployment := structs.DeploySpecV2{Name: testDeploymentName, Space: testDeploymentSpace}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/deploy", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Image must be specified")
			})
			Convey("it should require an image with a tag", func() {
				testDeployment := structs.DeploySpecV2{Name: testDeploymentName, Space: testDeploymentSpace, Image: "docker.io/akkeris/apachetest"}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/deploy", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Image must contain tag")
			})
		})

		Convey("When the deployment is successfully created", func() {
			testDeployment := structs.AppDeploymentSpec{
				AppID:     testDeploymentID,
				Name:      testDeploymentName,
				Space:     testDeploymentSpace,
				Instances: testInstances,
				Plan:      "scout",
			}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w.Body.String(), ShouldContainSubstring, "Deployment record created")

			Convey("it should exist in a list of deployments", func() {
				r, _ := http.NewRequest("GET", "/v2beta1/space/"+testDeploymentSpace+"/deployments", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				var response []structs.AppDeploymentSpec
				So(w.Code, ShouldEqual, http.StatusOK)
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(
					response,
					func(actual interface{}, expected ...interface{}) string {
						for _, v := range response {
							if v.AppID.String == testDeploymentID.String &&
								v.Name == testDeploymentName &&
								v.Space == testDeploymentSpace &&
								v.Instances.Int64 == testInstances.Int64 &&
								v.Plan == "scout" {
								return ""
							}
						}
						return "Deployment was not found in list of deployments for the " + testDeploymentSpace + " space."
					},
				)
			})

			Convey("it shouldn't collide with a new deployment", func() {
				testDeployment := structs.AppDeploymentSpec{
					AppID:     testDeploymentID,
					Name:      testDeploymentName,
					Space:     testDeploymentSpace,
					Instances: testInstances,
					Plan:      "scout",
				}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("POST", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Deployment already exists")
			})

			Convey("when we update the deployment", func() {
				Convey("it should be able to update the healthcheck", func() {
					testDeployment := structs.AppDeploymentSpec{Healthcheck: structs.PrettyNullString{sql.NullString{Valid: true, String: "/test"}}}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("PATCH", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/healthcheck", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					So(w.Body.String(), ShouldContainSubstring, "updated to use /test healthcheck")
				})
				Convey("it should be able to delete the healthcheck", func() {
					r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/healthcheck", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					So(w.Body.String(), ShouldContainSubstring, "healthcheck removed")
				})
				Convey("it should be able to update the plan", func() {
					testDeployment := structs.AppDeploymentSpec{Plan: "constellation"}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("PATCH", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/plan", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					So(w.Body.String(), ShouldContainSubstring, "updated to use constellation plan")
				})
			})

			Reset(func() {
				r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
			})
		})

		Convey("Given we have an app in a space with a healthcheck", func() {
			testDeployment := structs.AppDeploymentSpec{
				AppID:       testDeploymentID,
				Name:        testDeploymentName,
				Space:       testDeploymentSpace,
				Instances:   testInstances,
				Plan:        "scout",
				Healthcheck: testHealthcheck,
			}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w.Body.String(), ShouldContainSubstring, "Deployment record created")

			Convey("when a web deployment is created", func() {
				testDeployment := structs.DeploySpecV2{Name: testDeploymentName, Space: testDeploymentSpace, Image: "docker.io/akkeris/apachetest:latest", Port: 8080}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/deploy", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusCreated)
				So(w.Body.String(), ShouldContainSubstring, "Deployment Created")

				Convey("it should have info in a space", func() {
					r, _ := http.NewRequest("GET", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					var response structs.AppDeploymentSpec
					So(w.Code, ShouldEqual, http.StatusOK)
					decoder := json.NewDecoder(w.Body)
					if err := decoder.Decode(&response); err != nil {
						panic(err)
					}
					So(response.Healthcheck.String, ShouldEqual, "/")
				})
				Convey("it should be able to scale", func() {
					testDeployment := structs.AppDeploymentSpec{Instances: structs.PrettyNullInt64{sql.NullInt64{Valid: true, Int64: 2}}}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testDeployment); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("PATCH", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/scale", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusAccepted)
					So(w.Body.String(), ShouldContainSubstring, "Instances updated")
				})
			})

			Reset(func() {
				r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
			})
		})
	})
}
