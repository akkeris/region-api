package deployment

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

	m.Get("/v2beta1/apps", ListAppsV2)
	m.Get("/v2beta1/app/:appid", DescribeAppV2)

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

func TestDeploymentsV2(t *testing.T) {
	if os.Getenv("ENABLE_V2_ENDPOINTS") == "" || strings.ToLower(os.Getenv("ENABLE_V2_ENDPOINTS")) != "true" {
		t.Skip("V2 endpoints not enabled, skipping TestDeploymentsV2")
	}
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
					// Should probably change this to unmarshal JSON and check actual returned object
					So(w.Body.String(), ShouldContainSubstring, `"healthcheck":"/test"`)
				})
				Convey("it should be able to delete the healthcheck", func() {
					r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName+"/healthcheck", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					// Should probably change this to unmarshal JSON and check actual returned object
					So(w.Body.String(), ShouldContainSubstring, `"healthcheck":"tcp"`)
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
					// Should probably change this to unmarshal JSON and check actual returned object
					So(w.Body.String(), ShouldContainSubstring, `"plan":"constellation"`)
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
					// Should probably change this to unmarshal JSON and check actual returned object
					So(w.Body.String(), ShouldContainSubstring, `"instances":2`)
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

func TestAppsV2(t *testing.T) {
	if os.Getenv("ENABLE_V2_ENDPOINTS") == "" || strings.ToLower(os.Getenv("ENABLE_V2_ENDPOINTS")) != "true" {
		t.Skip("V2 endpoints not enabled, skipping TestAppsV2")
	}

	// DescribeAppV2

	testDeploymentName := "gotest"
	testDeploymentName2 := "gotest2"
	testDeploymentSpace := "gotest"
	testDeploymentID := structs.PrettyNullString{sql.NullString{
		String: "12345678-1234-1234-1234-123456789abc",
		Valid:  true,
	}}
	testInstances := structs.PrettyNullInt64{sql.NullInt64{
		Int64: 1,
		Valid: true,
	}}
	m := Init()

	Convey("Given we have a Deployment", t, func() {
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

			Convey("it should exist in a list of all deployments", func() {
				r, _ := http.NewRequest("GET", "/v2beta1/apps", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				type appInfo struct {
					ID          string   `json:"id"`          // App ID
					Deployments []string `json:"deployments"` // List of deployments associated with the app
				}
				var response []appInfo
				So(w.Code, ShouldEqual, http.StatusOK)
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(
					response,
					func(actual interface{}, expected ...interface{}) string {
						for _, v := range response {
							for _, w := range v.Deployments {
								if w == testDeploymentName+"-"+testDeploymentSpace {
									return ""
								}
							}
						}
						return "Deployment was not found in list of deployments for the given ID."
					},
				)
			})

			Convey("it should exist in a list of deployments for a specific app", func() {
				r, _ := http.NewRequest("GET", "/v2beta1/app/"+testDeploymentID.String, nil)
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
						return "Deployment was not found in list of deployments for the " + testDeploymentID.String + " app."
					},
				)
			})
		})
		Reset(func() {
			r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
		})
	})

	Convey("Given we have two deployments", t, func() {
		Convey("When both of the deployments are successfully created", func() {
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

			testDeployment2 := structs.AppDeploymentSpec{
				AppID:     testDeploymentID,
				Name:      testDeploymentName2,
				Space:     testDeploymentSpace,
				Instances: testInstances,
				Plan:      "scout",
			}
			b2 := new(bytes.Buffer)
			if err2 := json.NewEncoder(b2).Encode(testDeployment2); err2 != nil {
				panic(err2)
			}
			r2, _ := http.NewRequest("POST", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName2, b2)
			w2 := httptest.NewRecorder()
			m.ServeHTTP(w2, r2)

			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w2.Code, ShouldEqual, http.StatusCreated)

			Convey("they should both exist in a list of all deployments", func() {
				r, _ := http.NewRequest("GET", "/v2beta1/apps", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				type appInfo struct {
					ID          string   `json:"id"`          // App ID
					Deployments []string `json:"deployments"` // List of deployments associated with the app
				}
				var response []appInfo
				So(w.Code, ShouldEqual, http.StatusOK)
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(
					response,
					func(actual interface{}, expected ...interface{}) string {
						found1 := false
						found2 := false
						for _, v := range response {
							for _, w := range v.Deployments {
								if w == testDeploymentName+"-"+testDeploymentSpace {
									found1 = true
									continue
								}
								if w == testDeploymentName2+"-"+testDeploymentSpace {
									found2 = true
									continue
								}
							}
						}
						if found1 && found2 {
							return ""
						}
						return "One or more deployments were not found in the list of deployments for the given ID."
					},
				)
			})

			Convey("they should both exist alongside each other", func() {
				r, _ := http.NewRequest("GET", "/v2beta1/app/"+testDeploymentID.String, nil)
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
						found1 := false
						found2 := false
						for _, v := range response {
							if v.AppID.String == testDeploymentID.String &&
								v.Name == testDeploymentName &&
								v.Space == testDeploymentSpace &&
								v.Instances.Int64 == testInstances.Int64 &&
								v.Plan == "scout" {
								found1 = true
								continue
							}
							if v.AppID.String == testDeploymentID.String &&
								v.Name == testDeploymentName2 &&
								v.Space == testDeploymentSpace &&
								v.Instances.Int64 == testInstances.Int64 &&
								v.Plan == "scout" {
								found2 = true
								continue
							}
						}
						if found1 && found2 {
							return ""
						}
						return "One or more deployments were not found in the list of deployments for the " + testDeploymentID.String + " app."
					},
				)
			})
		})

		Reset(func() {
			r, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)

			r2, _ := http.NewRequest("DELETE", "/v2beta1/space/"+testDeploymentSpace+"/deployment/"+testDeploymentName2, nil)
			w2 := httptest.NewRecorder()
			m.ServeHTTP(w2, r2)
			So(w2.Code, ShouldEqual, http.StatusOK)
		})
	})
}
