package templates

import (
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

func Server() *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())
	m.Get("/v1/utils/urltemplates", GetURLTemplates)
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

func TestGetVaultVariables(t *testing.T) {
	m := Init()
	internaltemplate := "https://{name}-{space}.unittesti.example.com/"
	externaltemplate := "https://{name}-{space}.unittest.example.com/"
	os.Setenv("ALAMO_INTERNAL_URL_TEMPLATE", internaltemplate)
	os.Setenv("ALAMO_URL_TEMPLATE", externaltemplate)
	Convey("When getting templates with correct env vars", t, func() {
		r, _ := http.NewRequest("GET", "/v1/utils/urltemplates", nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusOK)
		var urltemplates structs.URLTemplates
		decoder := json.NewDecoder(w.Body)
		if err := decoder.Decode(&urltemplates); err != nil {
			panic(err)
		}
		So(urltemplates.Internal, ShouldEqual, internaltemplate)
		So(urltemplates.External, ShouldEqual, externaltemplate)

	})
	Convey("When getting templates with blank internal env var", t, func() {
		internaltemplateblank := ""
		os.Setenv("ALAMO_INTERNAL_URL_TEMPLATE", internaltemplateblank)
		os.Setenv("ALAMO_URL_TEMPLATE", externaltemplate)
		r, _ := http.NewRequest("GET", "/v1/utils/urltemplates", nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusInternalServerError)

	})
	Convey("When getting templates with blank external env var", t, func() {

		externaltemplateblank := ""
		os.Setenv("ALAMO_INTERNAL_URL_TEMPLATE", internaltemplate)
		os.Setenv("ALAMO_URL_TEMPLATE", externaltemplateblank)
		r, _ := http.NewRequest("GET", "/v1/utils/urltemplates", nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		So(w.Code, ShouldEqual, http.StatusInternalServerError)

	})
}
