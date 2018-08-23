package app

import (
	"database/sql"
	"fmt"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	config "region-api/config"
	runtime "region-api/runtime"
	service "region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

func OneOffDeployment(db *sql.DB, oneoff1 structs.OneOffSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	var name string
	var repo string
	var tag string

	if oneoff1.Podname == "" {
		utils.ReportInvalidRequest("Application Name can not be blank", r)
		return
	}
	if oneoff1.Space == "" {
		utils.ReportInvalidRequest("Space Name can not be blank", r)
		return
	}
	if oneoff1.Image == "" {
		utils.ReportInvalidRequest("Image must be specified", r)
		return
	}
	if oneoff1.Image != "" && !(strings.Contains(oneoff1.Image, ":")) {
		utils.ReportInvalidRequest("Image must contain tag", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, oneoff1.Space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if rt.OneOffExists(oneoff1.Space, oneoff1.Podname) {
		rt.DeletePod(oneoff1.Space, oneoff1.Podname)
	}

	name = oneoff1.Podname
	space := oneoff1.Space
	if oneoff1.Image != "" {
		repo = strings.Split(oneoff1.Image, ":")[0]
		tag = strings.Split(oneoff1.Image, ":")[1]
	}
	var (
		appport     int
		instances   int
		plan        string
		healthcheck string
	)

	err = db.QueryRow("SELECT apps.port,spacesapps.instances,COALESCE(spacesapps.plan,'noplan') AS plan,COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from apps,spacesapps where apps.name=$1 and apps.name=spacesapps.appname and spacesapps.space=$2", name, space).Scan(&appport, &instances, &plan, &healthcheck)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	appname := name
	appimage := repo
	apptag := tag

	// Get app bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, appname)

	// Get memory limits
	memorylimit, memoryrequest, err := GetMemoryLimits(db, plan)

	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Assembly config
	elist := AddAlamoConfigVars(appname, space)
	// add user specific vars
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}
	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, appbindings)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	for _, e := range servicevars {
		elist = append(elist, e)
	}
	// Create deployment
	var deployment structs.Deployment
	deployment.Space = space
	deployment.App = appname
	deployment.Amount = instances
	deployment.ConfigVars = elist
	deployment.HealthCheck = healthcheck
	deployment.MemoryRequest = memoryrequest
	deployment.MemoryLimit = memorylimit
	deployment.Image = appimage
	deployment.Tag = apptag

	err = rt.CreateOneOffPod(&deployment)
	if err != nil {
		fmt.Println("Error creating a one off pod")
		utils.ReportError(err, r)
	}

	r.JSON(http.StatusCreated, map[string]string{"Status": "Created"})
}
