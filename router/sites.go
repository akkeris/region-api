package router

import (
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"os"
	"regexp"
	utils "region-api/utils"
	"strings"
	"sync"
	"time"
)
func HttpGetDomains(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domains()
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	

	r.JSON(http.StatusOK, domains)
}
