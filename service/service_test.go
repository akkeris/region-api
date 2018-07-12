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

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/v2/services/postgres/plans", GetPostgresPlansV2)
	m.Get("/v2/services/postgres/:servicename/url", GetPostgresUrlV2)
	m.Post("/v2/services/postgres", binding.Json(structs.Provisionspec{}), ProvisionPostgresV2)
	m.Delete("/v2/services/postgres/:servicename", DeletePostgresV2)
	m.Post("/v2/services/postgres/:servicename/tags", binding.Json(structs.Tagspec{}), TagPostgresV2)
	m.Get("/v2/services/postgres/:servicename/backups", ListPostgresBackupsV2)
	m.Get("/v2/services/postgres/:servicename/backups/:backup", GetPostgresBackupV2)
	m.Put("/v2/services/postgres/:servicename/backups", CreatePostgresBackupV2)
	m.Put("/v2/services/postgres/:servicename/backups/:backup", RestorePostgresBackupV2)
	m.Get("/v2/services/postgres/:servicename/logs", ListPostgresLogsV2)
	m.Get("/v2/services/postgres/:servicename/logs/:dir/:file", GetPostgresLogsV2)
	m.Put("/v2/services/postgres/:servicename", RestartPostgresV2)
	m.Post("/v2/services/postgres/:servicename/roles", CreatePostgresRoleV2)
	m.Delete("/v2/services/postgres/:servicename/roles/:role", DeletePostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename/roles", ListPostgresRolesV2)
	m.Put("/v2/services/postgres/:servicename/roles/:role", RotatePostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename/roles/:role", GetPostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename", GetPostgresV2)

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

func TestPostgresServiceV2(t *testing.T) {
	m := Init()
	var postgresdb structs.Postgresspec
	var postgresname string

	Convey("Given we want to provision a service", t, func() {
		b := new(bytes.Buffer)
		payload := structs.Provisionspec{Plan: "micro", Billingcode: "test"}
		if err := json.NewEncoder(b).Encode(payload); err != nil {
			panic(err)
		}
		r, _ := http.NewRequest("POST", "/v2/services/postgres", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusCreated)
		decoder := json.NewDecoder(w.Body)

		if err := decoder.Decode(&postgresdb); err != nil {
			panic(err)
		}
		fmt.Println(postgresdb)
		So(postgresdb.DatabaseUrl, ShouldStartWith, "postgres://")
		So(postgresdb.Spec, ShouldStartWith, "postgres:")

		postgresname = strings.Split(postgresdb.Spec, ":")[1]
		Convey("When we want the postgres url", func() {
			r, _ := http.NewRequest("GET", "/v2/services/postgres/"+postgresname+"/url", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "DATABASE_URL")
			So(w.Body.String(), ShouldContainSubstring, postgresdb.DatabaseUrl)

			Convey("When we want the postgres database info", func() {
				r, _ := http.NewRequest("GET", "/v2/services/postgres/"+postgresname, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, "DATABASE_URL")
				So(w.Body.String(), ShouldContainSubstring, postgresdb.DatabaseUrl)

				Convey("When we want to remove a database", func() {
					r, _ := http.NewRequest("DELETE", "/v2/services/postgres/"+postgresname, nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
				})
			})
		})
	})

}
