package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	gocql "github.com/gocql/gocql"
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
	"time"
)

func ServerCassandra() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/v1/service/cassandra/plans", GetCassandraPlans)
	m.Get("/v1/service/cassandra/url/:servicename", GetCassandraURL)
	m.Post("/v1/service/cassandra/instance", binding.Json(structs.Provisionspec{}), ProvisionCassandra)
	m.Delete("/v1/service/cassandra/instance/:servicename", DeleteCassandra)

	return m
}

func InitCassandra() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := ServerCassandra()
	m.Map(pool)
	return m
}

func TestCassandraService(t *testing.T) {
	m := InitCassandra()

	var cassandra structs.Cassandraspec
	var cassandraname string

	Convey("Given we want to provision a service\n", t, func() {
		b := new(bytes.Buffer)
		payload := structs.Provisionspec{Plan: "shared", Billingcode: "test"}
		if err := json.NewEncoder(b).Encode(payload); err != nil {
			panic(err)
		}
		r, _ := http.NewRequest("POST", "/v1/service/cassandra/instance", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusCreated)
		decoder := json.NewDecoder(w.Body)
		if err := decoder.Decode(&cassandra); err != nil {
			panic(err)
		}

		So(cassandra.Location, ShouldStartWith, "dse")
		So(cassandra.Spec, ShouldStartWith, "cassandra:")
		cassandraname = strings.Split(cassandra.Spec, ":")[1]
		Convey("When we want the cassandra url\n", func() {
			r, _ := http.NewRequest("GET", "/v1/service/cassandra/url/"+cassandraname, nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Body.String(), ShouldContainSubstring, "CASSANDRA_LOCATION")
			So(w.Body.String(), ShouldContainSubstring, cassandra.Keyspace)

			Convey("When we want the cassandra database info\n", func() {
				r, _ := http.NewRequest("GET", "/v1/service/cassandra/url/"+cassandraname, nil)
				w := httptest.NewRecorder()
				m.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, "CASSANDRA_LOCATION")
				So(w.Body.String(), ShouldContainSubstring, cassandra.Keyspace)
				Convey("When we want to make sure we can get to the new db\n", func() {
					results, err := hitCassandraDB(cassandra.Location, cassandra.Keyspace, cassandra.Username, cassandra.Password)
					fmt.Println(err)
					fmt.Println(results)
					So(err, ShouldBeNil)
					So(results, ShouldEqual, cassandra.Keyspace)
					Convey("When we want to remove a database\n", func() {
						r, _ := http.NewRequest("DELETE", "/v1/service/cassandra/instance/"+cassandraname, nil)
						w := httptest.NewRecorder()
						m.ServeHTTP(w, r)
						So(w.Code, ShouldEqual, http.StatusOK)
					})
				})
			})
		})
	})
}

func hitCassandraDB(cassandra_url string, keyspace string, username string, password string) (r string, e error) {
	cluster := gocql.NewCluster(cassandra_url)
	pass := gocql.PasswordAuthenticator{username, password}
	cluster.IgnorePeerAddr = true
	cluster.ProtoVersion = 4
	cluster.CQLVersion = "3.0.0"
	cluster.Keyspace = keyspace
	cluster.Authenticator = pass
	duration := 10 * time.Second
	cluster.Timeout = duration
	cluster.ConnectTimeout = duration
	cluster.Consistency = gocql.One
	cluster.Port = 9042
	cluster.NumConns = 1
	var err error
	s, err := cluster.CreateSession()
	if err != nil {
		fmt.Println("got session error")
		fmt.Println(err)
		return "", err
	}
	ksmd, err := s.KeyspaceMetadata(keyspace)
	if err != nil {
		fmt.Println("got session error")
		fmt.Println(err)
		return "", err
	}
	keyspacename := ksmd.Name
	return keyspacename, nil
}
