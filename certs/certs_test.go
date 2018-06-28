package certs

import (
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
	"region-api/structs"
	"region-api/utils"
	"testing"
)

var pool *sql.DB

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())
	m.Post("/v1/certs", binding.Json(structs.CertificateRequestSpec{}), CertificateRequest)
	m.Get("/v1/certs", GetCerts)
	m.Get("/v1/certs/:id", GetCertStatus)
	return m
}

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool = utils.GetDB(pitdb)
	utils.InitAuth()
	m := Server()
	m.Map(pool)
	return m
}

func TestHandlers(t *testing.T) {
	m := Init()

	Convey("start\n", t, func() {
		Convey("Given we want to make a request for a new certificate\n", func() {
			Reset(func() {

				stmt, _ := pool.Prepare("delete from certs where request=$1")
				_, err := stmt.Exec("1313855")
				fmt.Println(err)

			})

			var request structs.CertificateRequestSpec
			request.CN = "apitest.example.com"
			request.SAN = []string{"apitest.qa.example.com", "apitest.dev.example.com"}
			cr := new(bytes.Buffer)
			if err := json.NewEncoder(cr).Encode(request); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v1/certs", cr)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			Convey("We should be able to get a list of cert requests\n", func() {

				r, _ := http.NewRequest("GET", "/v1/certs", cr)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				var certs []structs.CertificateRequestSpec
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&certs); err != nil {
					panic(err)
				}
				fmt.Println(certs)
				var certid string
				for _, cert := range certs {
					if cert.CN == "apitest.example.com" {
						certid = cert.ID
					}
				}
				fmt.Println("Cert ID is: " + certid)

				Convey("and we should be able to get the details on one cert\n", func() {
					r, _ := http.NewRequest("GET", "/v1/certs/"+certid, cr)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					var cert structs.CertificateRequestSpec
					decoder := json.NewDecoder(w.Body)
					if err := decoder.Decode(&cert); err != nil {
						panic(err)
					}
					fmt.Println(cert)
					So(cert.RequestStatus, ShouldEqual, "rejected")
				})
			})
		})
	})
}
