package service

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
	"region-api/structs"
	"region-api/utils"
	"strings"
	"testing"
)

func ServerOnPrem() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/v1/service/postgresonprem/plans", GetpostgresonpremplansV1)
	m.Get("/v1/service/postgresonprem/url/:servicename", GetpostgresonpremurlV1)
	m.Post("/v1/service/postgresonprem/instance", binding.Json(structs.Provisionspec{}), ProvisionpostgresonpremV1)
	m.Delete("/v1/service/postgresonprem/instance/:servicename", DeletepostgresonpremV1)
	m.Post("/v1/service/postgresonprem/:servicename/roles", CreatePostgresonpremRoleV1)
	m.Delete("/v1/service/postgresonprem/:servicename/roles/:role", DeletePostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:servicename/roles", ListPostgresonpremRolesV1)
	m.Put("/v1/service/postgresonprem/:servicename/roles/:role", RotatePostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:servicename/roles/:role", GetPostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:ame", GetPostgresonpremV1)

	m.Get("/v1/service/postgresonprem/:servicename/backups", ListPostgresonpremBackupsV1)
	return m
}

func InitOnPrem() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := ServerOnPrem()
	m.Map(pool)
	return m
}

func TestPostgresonpremService(t *testing.T) {
	m := InitOnPrem()

	var postgresonpremdb structs.Postgresspec
	var postgresonpremname string

	Convey("Given we want to provision a service\n", t, func() {
		b := new(bytes.Buffer)
		payload := structs.Provisionspec{Plan: "shared", Billingcode: "test"}
		if err := json.NewEncoder(b).Encode(payload); err != nil {
			panic(err)
		}
		r, _ := http.NewRequest("POST", "/v1/service/postgresonprem/instance", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusCreated)
		decoder := json.NewDecoder(w.Body)
		if err := decoder.Decode(&postgresonpremdb); err != nil {
			panic(err)
		}

		So(postgresonpremdb.DatabaseUrl, ShouldStartWith, "postgres://")
		So(postgresonpremdb.Spec, ShouldStartWith, "postgresonprem:")
		instance := strings.Split(postgresonpremdb.Spec, ":")[1]
		postgresonpremname = strings.Split(postgresonpremdb.Spec, ":")[1]
		Convey("When we want the postgresonprem url\n", func() {
			r, _ := http.NewRequest("GET", "/v1/service/postgresonprem/url/"+postgresonpremname, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "DATABASE_URL")
			So(w.Body.String(), ShouldContainSubstring, postgresonpremdb.DatabaseUrl)

			Convey("When we want the postgresonprem database info\n", func() {
				r, _ := http.NewRequest("GET", "/v1/service/postgresonprem/url/"+postgresonpremname, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, "DATABASE_URL")
				So(w.Body.String(), ShouldContainSubstring, postgresonpremdb.DatabaseUrl)

				Convey("Create Extra Role\n", func() {
					req, _ := http.NewRequest("POST", "/v1/service/postgresonprem/"+instance+"/roles", nil)
					resp := httptest.NewRecorder()
					m.ServeHTTP(resp, req)
					So(resp.Code, ShouldEqual, http.StatusCreated)
					decoder := json.NewDecoder(resp.Body)
					var response map[string]string
					if err := decoder.Decode(&response); err != nil {
						panic(err)
					}
					So(response["username"], ShouldNotEqual, "")
					So(response["password"], ShouldNotEqual, "")
					So(response["endpoint"], ShouldNotEqual, "")
					role := response["username"]
					password := response["password"]
					Convey("Get the role\n", func() {
						req, _ := http.NewRequest("GET", "/v1/service/postgresonprem/"+instance+"/roles/"+role, nil)
						resp := httptest.NewRecorder()
						m.ServeHTTP(resp, req)
						So(resp.Code, ShouldEqual, http.StatusOK)
						decoder := json.NewDecoder(resp.Body)
						var response map[string]string
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						fmt.Println(response)
						So(response["username"], ShouldNotEqual, "")
						So(response["password"], ShouldNotEqual, "")
						So(response["endpoint"], ShouldNotEqual, "")
						So(response["username"], ShouldEqual, role)
						Convey("Get the roles\n", func() {
							req, _ := http.NewRequest("GET", "/v1/service/postgresonprem/"+instance+"/roles", nil)
							resp := httptest.NewRecorder()
							m.ServeHTTP(resp, req)
							So(resp.Code, ShouldEqual, http.StatusOK)
							decoder := json.NewDecoder(resp.Body)
							var response []map[string]string
							if err := decoder.Decode(&response); err != nil {
								panic(err)
							}
							fmt.Println(response)
							So(response[0]["username"], ShouldNotEqual, "")
							So(response[0]["password"], ShouldNotEqual, "")
							So(response[0]["endpoint"], ShouldNotEqual, "")
							So(response[0]["username"], ShouldEqual, role)
							Convey("Rotate Extra Role\n", func() {
								req, _ := http.NewRequest("PUT", "/v1/service/postgresonprem/"+instance+"/roles/"+role, nil)
								resp := httptest.NewRecorder()
								m.ServeHTTP(resp, req)
								So(resp.Code, ShouldEqual, http.StatusOK)
								decoder := json.NewDecoder(resp.Body)
								var response map[string]string
								if err := decoder.Decode(&response); err != nil {
									panic(err)
								}
								So(response["username"], ShouldNotEqual, "")
								So(response["password"], ShouldNotEqual, "")
								So(response["endpoint"], ShouldNotEqual, "")
								So(response["username"], ShouldEqual, role)
								So(response["password"], ShouldNotEqual, password)
								Convey("Delete Role\n", func() {
									req, _ := http.NewRequest("DELETE", "/v1/service/postgresonprem/"+instance+"/roles/"+role, nil)
									resp := httptest.NewRecorder()
									m.ServeHTTP(resp, req)
									So(resp.Code, ShouldEqual, http.StatusOK)
									Convey("Get the roles - Should be empty\n", func() {
										req, _ := http.NewRequest("GET", "/v1/service/postgresonprem/"+instance+"/roles", nil)
										resp := httptest.NewRecorder()
										m.ServeHTTP(resp, req)
										So(resp.Code, ShouldEqual, http.StatusOK)
										decoder := json.NewDecoder(resp.Body)
										var response []map[string]string
										if err := decoder.Decode(&response); err != nil {
											panic(err)
										}
										fmt.Println(response)
										So(len(response), ShouldEqual, 0)

										Convey("Get the backups\n", func() {
											req, _ := http.NewRequest("GET", "/v1/service/postgresonprem/"+instance+"/backups", nil)
											resp := httptest.NewRecorder()
											m.ServeHTTP(resp, req)
											So(resp.Code, ShouldEqual, 422)
											decoder := json.NewDecoder(resp.Body)
											var response map[string]string
											if err := decoder.Decode(&response); err != nil {
												panic(err)
											}
											fmt.Println(response)
											So(response["error"], ShouldEqual, "Not available for this service")

											Convey("When we want to remove a database\n", func() {
												r, _ := http.NewRequest("DELETE", "/v1/service/postgresonprem/instance/"+postgresonpremname, nil)
												w := httptest.NewRecorder()
												m.ServeHTTP(w, r)
												So(w.Code, ShouldEqual, http.StatusOK)
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
