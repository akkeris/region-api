package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAppApi(t *testing.T) {
	m := TestInit()
	Convey("Given that user hits the healthcheck endpoint", t, func() {
		Convey("it should return OK", func() {
			r, _ := http.NewRequest("GET", "/v1/octhc/kube", nil)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r)
			So(w.Code, ShouldEqual, 200)
		})
	})
}
