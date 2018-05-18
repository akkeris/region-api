package app

import (
	"../config"
	"../space"
	"../structs"
	"../utils"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var pool *sql.DB

func ServerOneOff() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Post("/v1/app", binding.Json(structs.Appspec{}), Createapp)  //createapp.go
	m.Patch("/v1/app", binding.Json(structs.Appspec{}), Updateapp) //updateapp.go
	m.Get("/v1/apps", Listapps)                                    //listapps.go
	m.Get("/v1/app/:appname", Describeapp)                         //describeapp.go
	m.Delete("/v1/app/:appname", Deleteapp)                        //deleteapp.go
	m.Put("/v1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), space.AddApp)
	m.Get("/v1/space/:space/apps", Describespace)              //describeapp.go
	m.Get("/v1/space/:space/app/:appname", DescribeappInSpace) //describeapp.go

	m.Post("/v1/app/deploy/oneoff", binding.Json(structs.OneOffSpec{}), OneOffDeployment) //deployment.go

	m.Post("/v1/app/bind", binding.Json(structs.Bindspec{}), Createbind)                       //createbind.go
	m.Delete("/v1/app/:appname/bind/:bindspec", Unbindapp)                                     //unbindapp.go
	m.Post("/v1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), Createbind) //createbind.go
	m.Delete("/v1/space/:space/app/:appname/bind/**", Unbindapp)                               //unbindapp.go

	m.Get("/v1/space/:space/app/:app/instance", GetInstances)                   //instance.go
	m.Get("/v1/space/:space/app/:appname/instance/:instanceid/log", GetAppLogs) //instance.go
	m.Delete("/v1/space/:space/app/:app/instance/:instanceid", DeleteInstance)  //instance.go

	m.Post("/v1/config/set", binding.Json(structs.Setspec{}), config.Createset)
	m.Post("/v1/config/set/configvar", binding.Json([]structs.Varspec{}), config.Addvars)
	m.Delete("/v1/config/set/:setname", config.Deleteset)

	m.Get("/v1/kube/podstatus/:space/:app", PodStatus)        //status.go
	m.Get("/v1/space/:space/app/:app/status", Spaceappstatus) //status.go

	return m
}

func InitOneOff() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool = utils.GetDB(pitdb)
	utils.InitAuth()
	m := ServerOneOff()
	m.Map(pool)
	return m
}

func TestOneoffs(t *testing.T) {
	testAppName := "gotestoneoff"
	testAppSpace := "gotest"
	testBindSetName := "gotestoneoff-gotest"
	m := InitOneOff()

	Convey("Given we have an App", t, func() {
		Reset(func() {

			stmt, _ := pool.Prepare("delete from apps where name=$1")
			_, err := stmt.Exec(testAppName)
			fmt.Println(err)

			stmt, _ = pool.Prepare("delete from spacesapps where space=$1 and appname=$2")
			_, err = stmt.Exec(testAppSpace, testAppName)
			fmt.Println(err)

			stmt, _ = pool.Prepare("delete from sets where name=$1")
			_, err = stmt.Exec(testBindSetName)
			fmt.Println(err)

			stmt, _ = pool.Prepare("delete from configvars where setname=$1")
			_, err = stmt.Exec(testBindSetName)
			fmt.Println(err)

			stmt, _ = pool.Prepare("delete from appbindings where appname=$1")
			_, err = stmt.Exec(testAppName)
			fmt.Println(err)

		})
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
					configvar1 := structs.Varspec{Setname: testBindSetName, Varname: "EXIT_CODE", Varvalue: "0"}
                                        configvar2 := structs.Varspec{Setname: testBindSetName, Varname: "SLEEP_SECONDS", Varvalue: "5"}
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

									Convey("when a oneoff is created", func() {
										testApp := structs.OneOffSpec{Podname: testAppName, Space: testAppSpace, Image: "docker.io/akkeris/jobtest:latest"}
										b := new(bytes.Buffer)
										if err := json.NewEncoder(b).Encode(testApp); err != nil {
											panic(err)
										}
										r, _ := http.NewRequest("POST", "/v1/app/deploy/oneoff", b)
										w := httptest.NewRecorder()
										m.ServeHTTP(w, r)
										So(w.Code, ShouldEqual, http.StatusCreated)
										So(w.Body.String(), ShouldContainSubstring, "Created")
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
												time.Sleep(time.Second * 120)
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
												So(response[0].Output, ShouldNotBeNil)

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
                                                                                                                fmt.Println(response.Logs) 
														So(response.Logs, ShouldNotBeEmpty)
       
														Convey("the instance can be deleted", func() {
															r, _ := http.NewRequest("DELETE", "/v1/space/"+testAppSpace+"/app/"+testAppName+"/instance/"+instance, nil)
															w := httptest.NewRecorder()
															m.ServeHTTP(w, r)
															So(w.Code, ShouldEqual, http.StatusOK)
															So(w.Body.String(), ShouldContainSubstring, "Deleted "+instance)

														})
													})
												})
											})
										})
									})
								})

							})
						})
					})
				})
			})
		})
	})
}
