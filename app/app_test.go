package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"region-api/config"
	"region-api/maintenance"
	"region-api/space"
	"region-api/structs"
	"region-api/utils"
	"testing"

	"encoding/json"

	"fmt"
	"time"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
)

func Server() *martini.ClassicMartini {
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

	m.Post("/v1/space/:space/app/:appname/restart", Restart)  //restart.go
	m.Get("/v1/space/:space/app/:app/status", Spaceappstatus) //status.go
	m.Get("/v1/kube/podstatus/:space/:app", PodStatus)        //status.go

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

	// V2 Endpoints
	m.Get("/v2beta1/apps", ListAppsV2)
	m.Get("/v2beta1/app/:appid", DescribeAppV2)

	return m
}

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := Server()
	m.Map(pool)
	return m
}

func TestAppHandlers(t *testing.T) {
	testAppName := "gotest"
	m := Init() // intialize handlers (could pass in a mock db ind the future)
	Convey("Given we want an App", t, func() {
		Convey("When the app is successfully created", func() {
			testApp := structs.Appspec{Name: testAppName, Port: 8080}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(testApp); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v1/app", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w.Body.String(), ShouldContainSubstring, "App Created with ID")

			Convey("it should exist in a list of apps", func() {
				r, _ := http.NewRequest("GET", "/v1/apps", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				var response struct {
					Apps []string `json:"apps"`
				}
				So(w.Code, ShouldEqual, http.StatusOK)
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(response.Apps, ShouldContain, testAppName)

				Convey("it should have valid info", func() {
					r, _ := http.NewRequest("GET", "/v1/app/"+testAppName, nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					var response structs.Appspec
					decoder := json.NewDecoder(w.Body)
					if err := decoder.Decode(&response); err != nil {
						panic(err)
					}
					So(response.Name, ShouldContainSubstring, testAppName)

					Convey("it shouldn't be able to collide with an already created app", func() {
						testApp := structs.Appspec{Name: testAppName, Port: 8080}
						b := new(bytes.Buffer)
						if err := json.NewEncoder(b).Encode(testApp); err != nil {
							panic(err)
						}
						r, _ := http.NewRequest("POST", "/v1/app", b)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusInternalServerError)
						So(w.Body.String(), ShouldContainSubstring, "pq: duplicate key")
					})
				})
			})

			Convey("When we update the app", func() {
				Convey("it should fail without a name", func() {
					testApp := structs.Appspec{Port: 9000}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testApp); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("PATCH", "/v1/app", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusBadRequest)
					So(w.Body.String(), ShouldContainSubstring, "Name Cannot be blank")

					Convey("it should fail without a port", func() {
						testApp := structs.Appspec{Name: testAppName}
						b := new(bytes.Buffer)
						if err := json.NewEncoder(b).Encode(testApp); err != nil {
							panic(err)
						}
						r, _ := http.NewRequest("PATCH", "/v1/app", b)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusBadRequest)
						So(w.Body.String(), ShouldContainSubstring, "Port Cannot be blank")
						Convey("it should update the port", func() {
							testApp := structs.Appspec{Name: testAppName, Port: 9000}
							b := new(bytes.Buffer)
							if err := json.NewEncoder(b).Encode(testApp); err != nil {
								panic(err)
							}
							r, _ := http.NewRequest("PATCH", "/v1/app", b)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							So(w.Code, ShouldEqual, http.StatusCreated)                               //Should be StatusOK
							So(w.Body.String(), ShouldContainSubstring, "App "+testAppName+"Updated") //should update space

						})
					})
				})
			})

			Convey("When the app is deleted", func() {
				r, _ := http.NewRequest("DELETE", "/v1/app/"+testAppName, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, testAppName+" deleted")

				Convey("it should not exist in a list of apps", func() {
					r, _ := http.NewRequest("GET", "/v1/apps", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					var response struct {
						Apps []string `json:"apps"`
					}
					So(w.Code, ShouldEqual, http.StatusOK)
					decoder := json.NewDecoder(w.Body)
					if err := decoder.Decode(&response); err != nil {
						panic(err)
					}
					So(response.Apps, ShouldNotContain, testAppName)

					Convey("it should not have any info", func() {
						r, _ := http.NewRequest("GET", "/v1/app/"+testAppName, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusOK)
						var response structs.Appspec
						decoder := json.NewDecoder(w.Body)
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						So(response.Name, ShouldBeEmpty) // 404?
					})
				})
			})

			Reset(func() {
				r, _ := http.NewRequest("DELETE", "/v1/app/"+testAppName, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
			})
		})
		Convey("Given we send invalid app info", func() {
			Convey("it should fail with no name", func() {
				testApp := structs.Appspec{Port: 8080}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testApp); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("POST", "/v1/app", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldContainSubstring, "Name Cannot be blank")
				Convey("it should fail with no port", func() {
					testApp := structs.Appspec{Name: testAppName}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testApp); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("POST", "/v1/app", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusBadRequest)
					So(w.Body.String(), ShouldContainSubstring, "Port Cannot be blank")
				})
			})
		})
	})
}

func TestDeployments(t *testing.T) {
	testAppName := "gotest"
	testAppSpace := "gotest"
	testBindSetName := "gotest-gotest"
	m := Init()

	Convey("Given we have an App", t, func() {
		Convey("When the app is successfully created", func() {
			testApp := structs.Appspec{Name: testAppName, Port: 8080}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(testApp); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v1/app", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			So(w.Body.String(), ShouldContainSubstring, "App Created with ID")

			Convey("Given we have an app in a space", func() { //THIS DOES NOT TEST SPACEAPP FUNCTIONALITY, THATS IN SPACE PACKAGE
				testSpaceApp := structs.Spaceappspec{Appname: testAppName, Space: testAppSpace, Instances: 1, Plan: "scout"}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testSpaceApp); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v1/space/"+testAppSpace+"/app/"+testAppName, b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusCreated)

				Convey("When we have config sets", func() {
					//configset
					testBindSet := structs.Setspec{Setname: testBindSetName, Settype: "app"}
					bc := new(bytes.Buffer)
					if err := json.NewEncoder(bc).Encode(testBindSet); err != nil {
						panic(err)
					}
					rc, _ := http.NewRequest("POST", "/v1/config/set", bc)
					wc := httptest.NewRecorder()
					m.ServeHTTP(wc, rc)
					So(wc.Code, ShouldEqual, http.StatusCreated)

					//configvars to set
					var configset []structs.Varspec
					configvar1 := structs.Varspec{Setname: testBindSetName, Varname: "MERP", Varvalue: "derp"}
					configvar2 := structs.Varspec{Setname: testBindSetName, Varname: "TEST", Varvalue: "app"}
					configset = append(configset, configvar1)
					configset = append(configset, configvar2)
					bs := new(bytes.Buffer)
					if err := json.NewEncoder(bs).Encode(configset); err != nil {
						panic(err)
					}
					rs, _ := http.NewRequest("POST", "/v1/config/set/configvar", bs)
					ws := httptest.NewRecorder()
					m.ServeHTTP(ws, rs)
					So(ws.Code, ShouldEqual, http.StatusCreated)

					Convey("when we attempt to bind an app with invalid information", func() {
						Convey("it should require an app name", func() {
							testBind := structs.Bindspec{Bindname: testBindSetName, Bindtype: "config", Space: testAppSpace}
							b := new(bytes.Buffer)
							if err := json.NewEncoder(b).Encode(testBind); err != nil {
								panic(err)
							}
							r, _ := http.NewRequest("POST", "/v1/app/bind", b)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							So(w.Code, ShouldEqual, http.StatusBadRequest)
							So(w.Body.String(), ShouldContainSubstring, "Application Name can not be blank")

							Convey("it should require a space", func() {
								testBind := structs.Bindspec{Bindname: testBindSetName, Bindtype: "config", App: testAppName}
								b := new(bytes.Buffer)
								if err := json.NewEncoder(b).Encode(testBind); err != nil {
									panic(err)
								}
								r, _ := http.NewRequest("POST", "/v1/app/bind", b)
								w := httptest.NewRecorder()
								m.ServeHTTP(w, r)
								So(w.Code, ShouldEqual, http.StatusBadRequest)
								So(w.Body.String(), ShouldContainSubstring, "Space Name can not be blank")

								Convey("it should require a bind type", func() {
									testBind := structs.Bindspec{Bindname: testBindSetName, Space: testAppSpace, App: testAppName}
									b := new(bytes.Buffer)
									if err := json.NewEncoder(b).Encode(testBind); err != nil {
										panic(err)
									}
									r, _ := http.NewRequest("POST", "/v1/app/bind", b)
									w := httptest.NewRecorder()
									m.ServeHTTP(w, r)
									So(w.Code, ShouldEqual, http.StatusBadRequest)
									So(w.Body.String(), ShouldContainSubstring, "Bind Type can not be blank")

									Convey("it should require a bind type", func() {
										testBind := structs.Bindspec{Bindtype: "config", Space: testAppSpace, App: testAppName}
										b := new(bytes.Buffer)
										if err := json.NewEncoder(b).Encode(testBind); err != nil {
											panic(err)
										}
										r, _ := http.NewRequest("POST", "/v1/app/bind", b)
										w := httptest.NewRecorder()
										m.ServeHTTP(w, r)
										So(w.Code, ShouldEqual, http.StatusBadRequest)
										So(w.Body.String(), ShouldContainSubstring, "Bind Name can not be blank")
									})
								})
							})
						})
					})

					Convey("it should bind an app to the configset", func() {
						testBind := structs.Bindspec{Bindname: testBindSetName, Bindtype: "config", Space: testAppSpace, App: testAppName}
						b := new(bytes.Buffer)
						if err := json.NewEncoder(b).Encode(testBind); err != nil {
							panic(err)
						}
						r, _ := http.NewRequest("POST", "/v1/app/bind", b)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusCreated)

						Convey("it should exist in a list of apps in a space", func() {
							r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/apps", nil)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							var response []structs.Spaceappspec
							So(w.Code, ShouldEqual, http.StatusOK)
							decoder := json.NewDecoder(w.Body)
							if err := decoder.Decode(&response); err != nil {
								panic(err)
							}
							So(len(response), ShouldBeGreaterThan, 0)
							So(response[0].Appname, ShouldEqual, testAppName)

							Convey("it should have info in a space", func() {
								r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/app/"+testAppName, nil)
								w := httptest.NewRecorder()
								m.ServeHTTP(w, r)
								var response structs.Spaceappspec
								So(w.Code, ShouldEqual, http.StatusOK)
								decoder := json.NewDecoder(w.Body)
								if err := decoder.Decode(&response); err != nil {
									panic(err)
								}
								So(response.Appname, ShouldEqual, testAppName)

								Convey("it's parent app should have info about it's space", func() {
									r, _ := http.NewRequest("GET", "/v1/app/"+testAppName, nil)
									w := httptest.NewRecorder()
									m.ServeHTTP(w, r)
									So(w.Code, ShouldEqual, http.StatusOK)
									var response structs.Appspec
									decoder := json.NewDecoder(w.Body)
									if err := decoder.Decode(&response); err != nil {
										panic(err)
									}
									So(response.Name, ShouldContainSubstring, testAppName)
									So(response.Spaces[0].Space, ShouldEqual, testAppSpace)
								})
							})
						})
						//})

						Convey("when a web deployment is created", func() {
							testApp := structs.Deployspec{AppName: testAppName, Space: testAppSpace, Image: "docker.io/akkeris/apachetest:latest"}
							b := new(bytes.Buffer)
							if err := json.NewEncoder(b).Encode(testApp); err != nil {
								panic(err)
							}
							r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							So(w.Code, ShouldEqual, http.StatusCreated)
							So(w.Body.String(), ShouldContainSubstring, "Deployment Created")

							Convey("it should have an instance", func() {
								r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/instance", nil)
								w := httptest.NewRecorder()
								m.ServeHTTP(w, r)
								var response []structs.Instance
								So(w.Code, ShouldEqual, http.StatusOK)
								decoder := json.NewDecoder(w.Body)
								if err := decoder.Decode(&response); err != nil {
									panic(err)
								}
								So(len(response), ShouldBeGreaterThan, 0)
								instance := response[0].InstanceID

								Convey("the instance should have a status", func() {
									time.Sleep(time.Second * 240)
									r, _ := http.NewRequest("GET", "/v1/kube/podstatus/"+testAppSpace+"/"+testAppName, nil)
									w := httptest.NewRecorder()
									m.ServeHTTP(w, r)
									response := []structs.SpaceAppStatus{}
									decoder := json.NewDecoder(w.Body)
									if err := decoder.Decode(&response); err != nil {
										panic(err)
									}
									fmt.Println(response)
									So(w.Code, ShouldEqual, http.StatusOK)
									So(response[0].Output, ShouldContainSubstring, "Running")

									Convey("the spaceapp should have a status", func() {
										r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/status", nil)
										w := httptest.NewRecorder()
										m.ServeHTTP(w, r)
										response := structs.SpaceAppStatus{}
										decoder := json.NewDecoder(w.Body)
										if err := decoder.Decode(&response); err != nil {
											panic(err)
										}
										fmt.Println(response)
										So(w.Code, ShouldEqual, http.StatusOK)

										Convey("the instance should have logs", func() {
											r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/instance/"+instance+"/log", nil)
											w := httptest.NewRecorder()
											m.ServeHTTP(w, r)
											var response struct {
												Logs string `json:"logs"`
											}
											decoder := json.NewDecoder(w.Body)
											if err := decoder.Decode(&response); err != nil {
												panic(err)
											}
											So(w.Code, ShouldEqual, http.StatusOK)
											So(response.Logs, ShouldNotBeEmpty)

											Convey("it should be able to be updated", func() {
												testApp := structs.Deployspec{AppName: testAppName, Space: testAppSpace, Image: "docker.io/akkeris/apachetest:latest", Port: 8080}
												b := new(bytes.Buffer)
												if err := json.NewEncoder(b).Encode(testApp); err != nil {
													panic(err)
												}
												r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
												w := httptest.NewRecorder()
												m.ServeHTTP(w, r)
												So(w.Code, ShouldEqual, http.StatusCreated)
												So(w.Body.String(), ShouldContainSubstring, "Deployment Updated")

												Convey("the instance can be deleted", func() {
													r, _ := http.NewRequest("DELETE", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/instance/"+instance, nil)
													w := httptest.NewRecorder()
													m.ServeHTTP(w, r)
													So(w.Code, ShouldEqual, http.StatusOK)
													So(w.Body.String(), ShouldContainSubstring, "Deleted "+instance)

													Convey("the deployment can be restarted", func() {
														r, _ := http.NewRequest("POST", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/restart", nil)
														w := httptest.NewRecorder()
														m.ServeHTTP(w, r)
														So(w.Code, ShouldEqual, http.StatusOK)
														So(w.Body.String(), ShouldContainSubstring, "Restart Submitted")

														Convey("the deployment can be rolled back", func() {
															r, _ := http.NewRequest("POST", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/rollback/1", nil)
															w := httptest.NewRecorder()
															m.ServeHTTP(w, r)
															So(w.Code, ShouldEqual, http.StatusOK)
															So(w.Body.String(), ShouldContainSubstring, "rolled back to 1")
														})
													})
												})
											})
										})
									})
								})
							})
						})

						Reset(func() {
							r, _ := http.NewRequest("DELETE", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/bind/config:"+testBindSetName, nil)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							fmt.Println(w.Body.String())
							So(w.Code, ShouldEqual, http.StatusOK)

						})
					})

					Reset(func() {
						r, _ := http.NewRequest("DELETE", "/v1/config/set/"+testBindSetName, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusOK)
					})
				})

				Convey("when a deployment is invalid", func() {
					Convey("it should require a name", func() {
						testApp := structs.Deployspec{Space: testAppSpace, Image: "docker.io/akkeris/apachetest:latest"}
						b := new(bytes.Buffer)
						if err := json.NewEncoder(b).Encode(testApp); err != nil {
							panic(err)
						}
						r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusBadRequest)
						So(w.Body.String(), ShouldContainSubstring, "Application Name can not be blank")

						Convey("it should require a space", func() {
							testApp := structs.Deployspec{AppName: testAppName, Image: "docker.io/akkeris/apachetest:latest"}
							b := new(bytes.Buffer)
							if err := json.NewEncoder(b).Encode(testApp); err != nil {
								panic(err)
							}
							r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							So(w.Code, ShouldEqual, http.StatusBadRequest)
							So(w.Body.String(), ShouldContainSubstring, "Space Name can not be blank")

							Convey("it should require an image", func() {
								testApp := structs.Deployspec{AppName: testAppName, Space: testAppSpace}
								b := new(bytes.Buffer)
								if err := json.NewEncoder(b).Encode(testApp); err != nil {
									panic(err)
								}
								r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
								w := httptest.NewRecorder()
								m.ServeHTTP(w, r)
								So(w.Code, ShouldEqual, http.StatusBadRequest)
								So(w.Body.String(), ShouldContainSubstring, "Image must be specified")

								Convey("it should require an image with a tag", func() {
									testApp := structs.Deployspec{AppName: testAppName, Space: testAppSpace, Image: "docker.io/akkeris/apachetest"}
									b := new(bytes.Buffer)
									if err := json.NewEncoder(b).Encode(testApp); err != nil {
										panic(err)
									}
									r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
									w := httptest.NewRecorder()
									m.ServeHTTP(w, r)
									So(w.Code, ShouldEqual, http.StatusBadRequest)
									So(w.Body.String(), ShouldContainSubstring, "Image must contain tag")
								})
							})
						})
					})
				})

				Convey("it should not be deleted by main app", func() {
					r, _ := http.NewRequest("DELETE", "/v1/app/"+testAppName, nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusBadRequest)
					So(w.Body.String(), ShouldContainSubstring, "application still exists in spaces:")
				})

				Reset(func() {
					r, _ := http.NewRequest("DELETE", "/v1/space/"+"gotest"+"/app/"+"gotest", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
				})
			})

			Convey("Given we have an app in a space with a healthcheck", func() { //THIS DOES NOT TEST SPACEAPP FUNCTIONALITY, THATS IN SPACE PACKAGE
				testSpaceApp := structs.Spaceappspec{Appname: testAppName, Space: testAppSpace, Instances: 1, Plan: "scout", Healthcheck: "/"}
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testSpaceApp); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("PUT", "/v1/space/"+testAppSpace+"/app/"+testAppName, b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusCreated)

				Convey("when a web deployment is created", func() {
					testApp := structs.Deployspec{AppName: testAppName, Space: testAppSpace, Image: "docker.io/akkeris/apachetest:latest"}
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(testApp); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("POST", "/v1/app/deploy", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusCreated)
					So(w.Body.String(), ShouldContainSubstring, "Deployment Created")

					Convey("it should have info in a space", func() {
						r, _ := http.NewRequest("GET", "/v1/space/"+testAppSpace+"/app/"+testAppName, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						var response structs.Spaceappspec
						So(w.Code, ShouldEqual, http.StatusOK)
						decoder := json.NewDecoder(w.Body)
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						So(response.Healthcheck, ShouldEqual, "/")
					})
				})

				Reset(func() {
					r, _ := http.NewRequest("DELETE", "/v1/space/"+"gotest"+"/app/"+"gotest", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
				})
			})

			Reset(func() {
				r, _ := http.NewRequest("DELETE", "/v1/app/"+testAppName, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
			})
		})
	})
}

func TestQoSHandlers(t *testing.T) {
	m := Init() // intialize handlers (could pass in a mock db ind the future)
	Convey("Given we want to know the QoS details", t, func() {
		Convey("it should show us a list of plans", func() {
			r, _ := http.NewRequest("GET", "/v1/apps/plans", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			var response []structs.QoS
			So(w.Code, ShouldEqual, http.StatusOK)
			decoder := json.NewDecoder(w.Body)
			if err := decoder.Decode(&response); err != nil {
				panic(err)
			}
			So(len(response), ShouldBeGreaterThan, 0)
			So(response[0].Name, ShouldNotBeEmpty)
		})
	})
}

//TODO need to setup and teardown app beforehand instead of using unknown state app
func TestMaintenanceHandlers(t *testing.T) {
	m := Init()
	Convey("Given we want the maintenance status", t, func() {
		Convey("it should show that the maintenenace is disabled", func() {
			r, _ := http.NewRequest("GET", "/v1/space/default/app/api/maintenance", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			var response structs.Maintenancespec
			So(w.Code, ShouldEqual, http.StatusOK)
			decoder := json.NewDecoder(w.Body)
			if err := decoder.Decode(&response); err != nil {
				panic(err)
			}
			So(response.Status, ShouldEqual, "off")
		})
	})

	Convey("Given we want to put an app into maintenance mode", t, func() {
		Convey("it should respond successful with a 201", func() {
			r, _ := http.NewRequest("POST", "/v1/space/deck1/app/oct-apitest/maintenance", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)

			Convey("it should show that the maintenenace is enabled", func() {
				r, _ := http.NewRequest("GET", "/v1/space/deck1/app/oct-apitest/maintenance", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				var response structs.Maintenancespec
				So(w.Code, ShouldEqual, http.StatusOK)
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(response.Status, ShouldEqual, "on")

				Convey("we should be able to take it out of maintenance mode ", func() {
					r, _ := http.NewRequest("DELETE", "/v1/space/deck1/app/oct-apitest/maintenance", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)

					Convey("it should show that the maintenence is disabled", func() {
						r, _ := http.NewRequest("GET", "/v1/space/deck1/app/oct-apitest/maintenance", nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						var response structs.Maintenancespec
						So(w.Code, ShouldEqual, http.StatusOK)
						decoder := json.NewDecoder(w.Body)
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						So(response.Status, ShouldEqual, "off")
					})
				})
			})
		})
	})
}
