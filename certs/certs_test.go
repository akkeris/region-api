package certs

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/go-martini/martini"
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
	AddToMartini(m)
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
				stmt.Exec("1313855")
				issuer, _ := GetIssuer(pool, "cert-manager")
				issuer.DeleteCertificate("apitest.example.com")
			})
			var request structs.CertificateOrder
			request.CommonName = "apitest.example.com"
			request.SubjectAlternativeNames = []string{"apitest.qa.example.com", "apitest.dev.example.com"}
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
				var certs []structs.CertificateOrder
				decoder := json.NewDecoder(w.Body)
				if err := decoder.Decode(&certs); err != nil {
					panic(err)
				}
				var certid string = ""
				for _, cert := range certs {
					if cert.CommonName == "apitest.example.com" {
						certid = cert.Id
					}
				}
				So(certid, ShouldNotEqual, "")
				Convey("and we should be able to get the details on one cert\n", func() {
					r, _ := http.NewRequest("GET", "/v1/certs/"+certid, cr)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					var cert structs.CertificateOrder
					decoder := json.NewDecoder(w.Body)
					if err := decoder.Decode(&cert); err != nil {
						panic(err)
					}
					So(cert.Status, ShouldEqual, "pending")
				})
				Reset(func() {
					stmt, _ := pool.Prepare("delete from certs where request=$1")
					stmt.Exec("1313855")
					issuer, _ := GetIssuer(pool, "cert-manager")
					issuer.DeleteCertificate("apitest.example.com")
				})
			})
		})
	})
}
