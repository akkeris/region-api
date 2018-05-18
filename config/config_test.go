package config

import (
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
)

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())
	m.Get("/v1/config/sets", Listsets)
	m.Post("/v1/config/set", binding.Json(structs.Setspec{}), Createset)
	m.Post("/v1/config/set/configvar", binding.Json([]structs.Varspec{}), Addvars)
	m.Post("/v1/config/set/configvaradd", binding.Json(structs.Varspec{}), Addvar)
	m.Patch("/v1/config/set/configvar", binding.Json(structs.Varspec{}), Updatevar)
	m.Delete("/v1/config/set/:setname/configvar/:varname", Deletevar)
	m.Get("/v1/config/set/:setname", Dumpset)
	m.Delete("/v1/config/set/:setname", Deleteset)
	m.Post("/v1/config/set/:parent/include/:child", Includeset)
	m.Delete("/v1/config/set/:parent/include/:child", Deleteinclude)
	m.Get("/v1/config/set/:setname/configvar/:varname", Getvar)
	return m
}

var pool *sql.DB

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool = utils.GetDB(pitdb)
	utils.InitAuth()
	m := Server()
	m.Map(pool)
	return m
}

func TestHandlers(t *testing.T) {
	testapp := "oct-apitest"
	//    testspace := "deck1"

	m := Init()
	//r,_ := http.NewRequest("DELETE", "/v1/config/set/testsetname", nil)
	//w := httptest.NewRecorder()
	//m.ServeHTTP(w,r)
	//fmt.Println(r)
	Convey("start", t, func() {
		Convey("Given we want to get an error when creating a set", func() {

			b := new(bytes.Buffer)
			b.Write([]byte("Merp Derp"))
			r, _ := http.NewRequest("POST", "/v1/config/set", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			fmt.Println(w.Code)
			So(w.Code, ShouldEqual, http.StatusBadRequest)
		})
		Convey("Give we want to get an error when adding a variable", func() {
			b := new(bytes.Buffer)
			b.Write([]byte("Merp Derp"))
			r, _ := http.NewRequest("POST", "/v1/config/set/configvaradd", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			fmt.Println(w.Code)
			So(w.Code, ShouldEqual, http.StatusBadRequest)
		})
		Convey("Give we want to get an error when adding variables", func() {
			b := new(bytes.Buffer)
			b.Write([]byte("Merp Derp"))
			r, _ := http.NewRequest("POST", "/v1/config/set/configvar", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			fmt.Println(w.Code)
			So(w.Code, ShouldEqual, http.StatusBadRequest)
		})
		Convey("Give we want to get an error when updating variables", func() {
			b := new(bytes.Buffer)
			b.Write([]byte("Merp Derp"))
			r, _ := http.NewRequest("PATCH", "/v1/config/set/configvar", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			fmt.Println(w.Code)
			So(w.Code, ShouldEqual, http.StatusBadRequest)
		})
		Convey("Given we want a set", func() {
			var fullset map[string]string
			fullset, _ = GetConfigVars(pool, "oct-apitest-cs")
			fmt.Println(fullset)
			So(fullset, ShouldNotBeEmpty)
		})
		Convey("Given that we want to see a list of config sets", func() {
			r, _ := http.NewRequest("GET", "/v1/config/sets", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			var sets []structs.Setspec
			decoder := json.NewDecoder(w.Body)
			if err := decoder.Decode(&sets); err != nil {
				panic(err)
			}
			fmt.Println(sets)
			var found bool
			found = false
			for _, element := range sets {
				setname := element.Setname
				fmt.Println(setname)
				if setname == testapp+"-cs" {
					found = true
				}
			}
			So(found, ShouldBeTrue)
		})
		Convey("Given that we want a parent configset and a child config set to include", func() {
			Reset(func() {
				rd, _ := http.NewRequest("DELETE", "/v1/config/set/parentset/include/childset", nil)
				wd := httptest.NewRecorder()
				m.ServeHTTP(wd, rd)
				fmt.Println(rd)
				rp, _ := http.NewRequest("DELETE", "/v1/config/set/parentset", nil)
				wp := httptest.NewRecorder()
				m.ServeHTTP(wp, rp)
				fmt.Println(rp)
				rc, _ := http.NewRequest("DELETE", "/v1/config/set/childset", nil)
				wc := httptest.NewRecorder()
				m.ServeHTTP(wc, rc)
				fmt.Println(rc)
			})
			var parentset structs.Setspec
			parentset.Setname = "parentset"
			parentset.Settype = "testsettype"
			bp := new(bytes.Buffer)
			if err := json.NewEncoder(bp).Encode(parentset); err != nil {
				panic(err)
			}
			rp, _ := http.NewRequest("POST", "/v1/config/set", bp)
			wp := httptest.NewRecorder()
			m.ServeHTTP(wp, rp)
			So(wp.Code, ShouldEqual, http.StatusCreated)
			var childset structs.Setspec
			childset.Setname = "childset"
			childset.Settype = "testsettype"
			bc := new(bytes.Buffer)
			if err := json.NewEncoder(bc).Encode(childset); err != nil {
				panic(err)
			}
			rc, _ := http.NewRequest("POST", "/v1/config/set", bc)
			wc := httptest.NewRecorder()
			m.ServeHTTP(wc, rc)
			So(wp.Code, ShouldEqual, http.StatusCreated)
			Convey("We should be able to include one in the other", func() {
				r, _ := http.NewRequest("POST", "/v1/config/set/parentset/include/childset", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				fmt.Println(r)
				So(w.Code, ShouldEqual, http.StatusCreated)
				Convey("We should be able to delete the include", func() {
					r, _ := http.NewRequest("DELETE", "/v1/config/set/parentset/include/childset", nil)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					fmt.Println(r)
					So(w.Code, ShouldEqual, http.StatusOK)
				})
			})

		})
		Convey("Given that we want a new config set", func() {
			Reset(func() {
				r, _ := http.NewRequest("DELETE", "/v1/config/set/testsetname", nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				fmt.Println(r)
			})
			var set structs.Setspec
			set.Setname = "testsetname"
			set.Settype = "testsettype"
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(set); err != nil {
				panic(err)
			}
			r, _ := http.NewRequest("POST", "/v1/config/set", b)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusCreated)
			Convey("We should be able to add two variables to it", func() {
				var vars []structs.Varspec
				var var1 structs.Varspec
				var var2 structs.Varspec
				var1.Setname = set.Setname
				var1.Varname = "var1name"
				var1.Varvalue = "var1value"
				var2.Setname = set.Setname
				var2.Varname = "var2name"
				var2.Varvalue = "var2value"
				vars = append(vars, var1)
				vars = append(vars, var2)
				b := new(bytes.Buffer)
				if err := json.NewEncoder(b).Encode(vars); err != nil {
					panic(err)
				}
				r, _ := http.NewRequest("POST", "/v1/config/set/configvar", b)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusCreated)
				Convey("We should be able to update a value", func() {
					var var1new structs.Varspec
					var1new.Setname = set.Setname
					var1new.Varname = "var1name"
					var1new.Varvalue = "var1valuenew"
					b := new(bytes.Buffer)
					if err := json.NewEncoder(b).Encode(var1new); err != nil {
						panic(err)
					}
					r, _ := http.NewRequest("PATCH", "/v1/config/set/configvar", b)
					w := httptest.NewRecorder()
					m.ServeHTTP(w, r)
					So(w.Code, ShouldEqual, http.StatusOK)
					Convey("We should be able to delete a variable", func() {
						r, _ := http.NewRequest("DELETE", "/v1/config/set/testsetname/configvar/var2name", nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusOK)
						Convey("And dump the set with the correct variable and value", func() {
							var vars []structs.Varspec
							r, _ := http.NewRequest("GET", "/v1/config/set/testsetname", nil)
							w := httptest.NewRecorder()
							m.ServeHTTP(w, r)
							decoder := json.NewDecoder(w.Body)
							if err := decoder.Decode(&vars); err != nil {
								panic(err)
							}
							fmt.Println(vars)
							var found bool
							found = false
							fmt.Println(vars)
							fmt.Println(len(vars))
							if vars[0].Setname == "testsetname" && vars[0].Varname == "var1name" && vars[0].Varvalue == "var1valuenew" && len(vars) == 1 {
								found = true
							}
							So(found, ShouldBeTrue)
							Convey("And also add a single variable", func() {
								var varadd structs.Varspec
								varadd.Setname = "testsetname"
								varadd.Varname = "varaddname"
								varadd.Varvalue = "varaddvalue"
								b := new(bytes.Buffer)
								if err := json.NewEncoder(b).Encode(varadd); err != nil {
									panic(err)
								}
								r, _ := http.NewRequest("POST", "/v1/config/set/configvaradd", b)
								w := httptest.NewRecorder()
								m.ServeHTTP(w, r)
								fmt.Println(r)
								So(w.Code, ShouldEqual, http.StatusCreated)
								Convey("And then get that var", func() {
									r, _ := http.NewRequest("GET", "/v1/config/set/testsetname/configvar/varaddname", nil)
									w := httptest.NewRecorder()
									m.ServeHTTP(w, r)
									var addedvar structs.Varspec
									decoder := json.NewDecoder(w.Body)
									if err := decoder.Decode(&addedvar); err != nil {
										panic(err)
									}
									So(addedvar.Varvalue, ShouldEqual, "varaddvalue")
								})
							})
						})
					})

				})

			})
		})
	})

}
