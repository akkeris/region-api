package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"region-api/structs"
	"region-api/utils"
	"strings"
	"testing"
)

func ServerInflux() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/v1/service/influxdb/plans", GetInfluxdbPlans)
	m.Get("/v1/service/influxdb/url/:servicename", GetInfluxdbURL)
	m.Post("/v1/service/influxdb/instance", binding.Json(structs.Provisionspec{}), ProvisionInfluxdb)
	m.Delete("/v1/service/influxdb/instance/:servicename", DeleteInfluxdb)

	return m
}

func InitInflux() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := ServerInflux()
	m.Map(pool)
	return m
}

func TestPostgresonpremService(t *testing.T) {
	m := InitInflux()

	var influxdb structs.Influxdbspec
	var influxdbname string

	Convey("Given we want to provision a service\n", t, func() {
		b := new(bytes.Buffer)
		payload := structs.Provisionspec{Plan: "shared", Billingcode: "test"}
		if err := json.NewEncoder(b).Encode(payload); err != nil {
			panic(err)
		}
		r, _ := http.NewRequest("POST", "/v1/service/influxdb/instance", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusCreated)
		decoder := json.NewDecoder(w.Body)
		if err := decoder.Decode(&influxdb); err != nil {
			panic(err)
		}

		So(influxdb.Url, ShouldStartWith, "https://")
		So(influxdb.Spec, ShouldStartWith, "influxdb:")
		influxdbname = strings.Split(influxdb.Spec, ":")[1]
		Convey("When we want the influxdb url\n", func() {
			r, _ := http.NewRequest("GET", "/v1/service/influxdb/url/"+influxdbname, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "INFLUX_URL")
			So(w.Body.String(), ShouldContainSubstring, influxdb.Url)

			Convey("When we want the influxdb database info\n", func() {
				r, _ := http.NewRequest("GET", "/v1/service/influxdb/url/"+influxdbname, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, "INFLUX_URL")
				So(w.Body.String(), ShouldContainSubstring, influxdb.Url)
				Convey("When we want to make sure we can get to the new db\n", func() {
					results, err := hitDB(influxdb.Url, influxdb.Name, influxdb.Username, influxdb.Password)
					fmt.Println(err)
					fmt.Println(results)
                                        So(err, ShouldBeNil)
					So(results, ShouldEqual, "{\"results\":[{\"statement_id\":0}]}\n")
					Convey("When we want to remove a database\n", func() {
						r, _ := http.NewRequest("DELETE", "/v1/service/influxdb/instance/"+influxdbname, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusOK)
					})
				})
			})
		})
	})
}

func hitDB(endpoint string, db string, username string, password string) (r string, e error) {
	var toreturn string
	t := url.URL{Path: "q=show measurements"}
	estring := t.String()
	req, err := http.NewRequest("GET", endpoint+"/query?db="+db+"&"+estring, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	fmt.Println(resp)
	defer resp.Body.Close()
	bodybytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	toreturn = string(bodybytes)
	return toreturn, nil
}
