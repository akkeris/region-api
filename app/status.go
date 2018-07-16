package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"
)

func Spaceappstatus(params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	appspace := app + "-" + space

	if space == "default" {
		appspace = app
	}
	service := os.Getenv("NAGIOS_ADDRESS")
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		utils.ReportError(err, r)
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		utils.ReportError(err, r)
	}
	_, err = conn.Write([]byte("GET services\nColumns: display_name state plugin_output execution_time long_plugin_output last_check\nFilter: display_name = alamo-" + appspace + "\n"))
	conn.CloseWrite()
	if err != nil {
		utils.ReportError(err, r)
	}
	var resulta []string
	result, err := ioutil.ReadAll(conn)
	if err != nil {
		utils.ReportError(err, r)
	}
	//fmt.Println(string(result))
	resulta = strings.Split(string(result), "\n")
	var resultso structs.SpaceAppStatus
	for _, element := range resulta {
		if element != "" {
			var s structs.SpaceAppStatus
			parts := strings.Split(element, ";")
			s.App = app
			s.Space = space
			statusint, _ := strconv.Atoi(parts[1])
			s.Status = statusint
			s.Output = parts[2]
			s.ExecutionTime = parts[3]
			s.ExtendedOutput = parts[4]
			checktime, _ := strconv.Atoi(parts[5])
			s.LastCheckTime = checktime
			resultso = s
		}
	}
	r.JSON(http.StatusOK, resultso)
}

func PodStatus(db *sql.DB, params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, rt.GetPodStatus(space, app))
}
