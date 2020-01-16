package router

import (
	"bytes"
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"os"
	"region-api/utils"
	"testing"
)

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := martini.Classic()
	m.Use(render.Renderer())
	AddToMartini(m)
	m.Map(pool)
	return m
}

func TestHandlers(t *testing.T) {
	testDomain := "gotestdomain.example.com"
	testapp := "oct-apitest"
	testspace := "deck1"
	m := Init() // intialize handlers (could pass in a mock db ind the future)

	Convey("Given we want to create a site/router", t, func() {
		Reset(func() {
			r, _ := http.NewRequest("DELETE", "/v1/router/"+testDomain, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
		})
		Convey("it should respond successful with a 201", func() {
			testRouter := Router{Domain: testDomain}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(testRouter); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v1/router", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			Convey("we should be able to add a path to it", func() {
				path := "/health"
				replacepath := "/octhc"
				var testPath Route
				testPath.Domain = testDomain
				testPath.Path = path
				testPath.ReplacePath = replacepath
				testPath.App = testapp
				testPath.Space = testspace
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(testPath); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("POST", "/v1/router/"+testDomain+"/path", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusCreated)
				Convey("we should be able to push the router", func() {
					r, _ := http.NewRequest("PUT", "/v1/router/"+testDomain, nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					Convey("and see the router in the list of routers", func() {
						r, _ := http.NewRequest("GET", "/v1/routers", nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						var response []Router
						So(w.Code, ShouldEqual, http.StatusOK)
						decoder := json.NewDecoder(w.Body)
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						var exists bool
						var haspath bool
						exists = false
						haspath = false
						for _, element := range response {
							if element.Domain == testDomain {
								exists = true
								if element.Paths[0].Path == "/health" && element.Paths[0].ReplacePath == "/octhc" {
									haspath = true
								}
							}
						}
						So(exists, ShouldBeTrue)
						So(haspath, ShouldBeTrue)
					})
					Convey("and describe it", func() {
						r, _ := http.NewRequest("GET", "/v1/router/"+testDomain, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						var response Router
						So(w.Code, ShouldEqual, http.StatusOK)
						decoder := json.NewDecoder(w.Body)
						if err := decoder.Decode(&response); err != nil {
							panic(err)
						}
						var exists bool
						var haspath bool
						exists = false
						haspath = false
						if response.Domain == testDomain {
							exists = true
						}
						if response.Paths[0].Path == "/health" && response.Paths[0].ReplacePath == "/octhc" {
							haspath = true
						}
						So(exists, ShouldBeTrue)
						So(haspath, ShouldBeTrue)
						Convey("and remove the path", func() {
							path := "/health"
							var testPath Route
							testPath.Path = path
							b := new(bytes.Buffer)
							if err := json.NewEncoder(b).Encode(testPath); err != nil {
								panic(err)
							}
							r, _ := http.NewRequest("DELETE", "/v1/router/"+testDomain+"/path", b)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							So(w.Code, ShouldEqual, http.StatusOK)
						})
					})
				})
			})
		})
	})
}
