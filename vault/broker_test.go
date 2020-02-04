package vault

import (
	structs "region-api/structs"
	"region-api/utils"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"encoding/json"
	"os"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	. "github.com/smartystreets/goconvey/convey"
)

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())
	AddToMartini(m)
	return m
}

func Init() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	utils.InitAuth()
	m := Server()
	GetVaultListPeriodic()
	m.Map(pool)
	return m
}

func lookupCred(creds []structs.Creds, cred string) structs.Creds {
	r := structs.Creds{}

	for _, e := range creds {
		if e.Key == cred {
			return e
		}
	}
	return r
}

func TestGetVaultVariables(t *testing.T) {
	testSecretQa := "secret/qa/db/perf"
	testSecretStage := "secret/stage/db/perf"

	var secretQa []structs.Creds
	var secretStage []structs.Creds

	m := Init()

	Convey("When listing secrets from vault", t, func() {

		b := new(bytes.Buffer)
		r, _ := http.NewRequest("GET", "/v1/service/vault/plans", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)

		So(w.Code, ShouldEqual, http.StatusOK)

		var response [1000]string
		d := json.NewDecoder(w.Body)
		if err := d.Decode(&response); err != nil {
			panic(err)
		}
		So(response, ShouldNotBeEmpty)
		So(response, ShouldContain, "vault:"+testSecretQa)
	})

	Convey("When getting QA variables from vault", t, func() {
		secretQa = GetVaultVariables(testSecretQa)
		So(lookupCred(secretQa, "OCT_VAULT_DB_PERF_USERNAME"), ShouldNotBeNil)
	})

	Convey("When getting STAGE variables from vault", t, func() {
		secretStage = GetVaultVariables(testSecretStage)
		So(lookupCred(secretStage, "OCT_VAULT_DB_PERF_USERNAME").Value, ShouldContainSubstring, "p42")
	})

	Convey("When displaying creds from vault", t, func() {
		b := new(bytes.Buffer)
		r, _ := http.NewRequest("GET", "/v1/service/vault/credentials/secret/stage/db/perf", b)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)

		So(w.Code, ShouldEqual, http.StatusOK)

		var response [100]structs.Creds
		d := json.NewDecoder(w.Body)
		if err := d.Decode(&response); err != nil {
			panic(err)
		}

		var pass string

		for _, c := range response {
			if c.Key == "OCT_VAULT_DB_PERF_PASSWORD" {
				pass = c.Value
			}
		}

		So(pass, ShouldContainSubstring, "redacted")
	})
	Convey("When getting vault paths from SECRETS", t, func() {
		paths := GetVaultPaths()
		fmt.Println(paths)
		So(paths, ShouldContain, "secret/qa")

		So(paths[0], ShouldContainSubstring, "dev")
	})
}