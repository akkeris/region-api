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
	"testing"
)

func ServerES() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/v1/service/es/plans", Getesplans)
	m.Get("/v1/service/es/url/:servicename", Getesurl)
	m.Get("/v1/service/es/instance/:servicename/status", Getesstatus)
	m.Post("/v1/service/es/instance/tag", binding.Json(structs.Tagspec{}), Tages)

	return m
}

func InitES() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := ServerES()
	m.Map(pool)
	return m
}

func TestESService(t *testing.T) {
	m := InitES()

	Convey("Given we want the plans\n", t, func() {
		r, _ := http.NewRequest("GET", "/v1/service/es/plans", nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusOK)
		fmt.Println(w)
		decoder := json.NewDecoder(w.Body)
		var response []map[string]string
		if err := decoder.Decode(&response); err != nil {
			panic(err)
		}
		So(response[0]["size"], ShouldNotEqual, "")
		So(response[0]["description"], ShouldNotEqual, "")
		Convey("To get the url\n", func() {
			r, _ := http.NewRequest("GET", "/v1/service/es/url/merpderp", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
			fmt.Println(w)
			decoder := json.NewDecoder(w.Body)
			var response map[string]string
			if err := decoder.Decode(&response); err != nil {
				panic(err)
			}
			So(response["ES_URL"], ShouldEqual, "https://vpc-merpderp-266sk4si4qhodjybbn3gbkznpq.us-west-2.es.amazonaws.com")
			So(response["KIBANA_URL"], ShouldEqual, "https://vpc-merpderp-266sk4si4qhodjybbn3gbkznpq.us-west-2.es.amazonaws.com/_plugin/kibana")
			Convey("To get the status\n", func() {
				r, _ := http.NewRequest("GET", "/v1/service/es/instance/merpderp/status", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				fmt.Println(w)
				decoder := json.NewDecoder(w.Body)
				var response map[string]string
				if err := decoder.Decode(&response); err != nil {
					panic(err)
				}
				So(response["ES_URL"], ShouldEqual, "https://vpc-merpderp-266sk4si4qhodjybbn3gbkznpq.us-west-2.es.amazonaws.com")
				So(response["KIBANA_URL"], ShouldEqual, "https://vpc-merpderp-266sk4si4qhodjybbn3gbkznpq.us-west-2.es.amazonaws.com/_plugin/kibana")
				Convey("Tag and instance", func() {
					b := new(bytes.Buffer)
					var payload structs.Tagspec
					payload.Resource = "merpderp"
					payload.Name = "unittestname"
					payload.Value = "unittestvalue"
					if err := json.NewEncoder(b).Encode(payload); err != nil {
						panic(err)
					}
					fmt.Println(b)
					req, _ := http.NewRequest("POST", "/v1/service/es/instance/tag", b)
					resp := httptest.NewRecorder()
					m.ServeHTTP(resp, req)
					fmt.Println(resp)
					So(resp.Code, ShouldEqual, http.StatusCreated)
					decoder := json.NewDecoder(resp.Body)
					var response map[string]interface{}
					if err := decoder.Decode(&response); err != nil {
						panic(err)
					}
					fmt.Println(response)
					So(response["message"], ShouldEqual, "tag added")
				})
			})
		})

	})
}
