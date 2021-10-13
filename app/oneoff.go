package app

import (
	"database/sql"
	"fmt"
	"net/http"
	config "region-api/config"
	runtime "region-api/runtime"
	service "region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
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
	if oneoff1.Image == "" {
		utils.ReportInvalidRequest("Image must be specified", r)
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

	appname := name
	appimage := repo
	apptag := tag

	// Get app bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, appname)

	// Get memory limits
	memorylimit, memoryrequest, err := GetMemoryLimits(db, oneoff1.Plan)

	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	internal, err := utils.IsInternalSpace(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if oneoff1.Labels == nil {
		oneoff1.Labels = make(map[string]string)
	}

	plantype, err := GetPlanType(db, oneoff1.Plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if plantype != nil && *plantype != "" {
		oneoff1.Labels["akkeris.io/plan-type"] = *plantype
	}
	oneoff1.Labels["akkeris.io/plan"] = oneoff1.Plan
	oneoff1.Labels["akkeris.io/oneoff"] = "true"
	if internal {
		oneoff1.Labels["akkeris.io/internal"] = "true"
	} else {
		oneoff1.Labels["akkeris.io/internal"] = "false"
	}

	// Assembly config
	elist := AddAkkerisConfigVars(appname, space)
	// add user specific vars
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}
	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, appname, space, appbindings)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	for _, e := range servicevars {
		elist = append(elist, e)
	}

	// Override config vars with any one-off env overrides
	for _, override := range oneoff1.Env {
		found := false
		for _, env := range elist {
			if env.Name == override.Name {
				env.Value = override.Value
				found = true
			}
		}
		if !found {
			elist = append(elist, override)
		}
	}

	// Create deployment
	var deployment structs.Deployment
	deployment.Space = space
	deployment.App = appname
	// deployment.Amount = instances
	deployment.ConfigVars = elist
	// deployment.HealthCheck = healthcheck
	deployment.MemoryRequest = memoryrequest
	deployment.MemoryLimit = memorylimit
	deployment.Image = appimage
	deployment.Tag = apptag

	if len(oneoff1.Command) > 0 {
		deployment.Command = oneoff1.Command
	}

	if oneoff1.RunID != "" {
		deployment.Annotations = make(map[string]string)
		deployment.Annotations["logtrain.akkeris.io/drains"] = "persistent://" + oneoff1.RunID
	}

	err = rt.CreateOneOffPod(&deployment)
	if err != nil {
		fmt.Println("Error creating a one off pod")
		utils.ReportError(err, r)
	}

	r.JSON(http.StatusCreated, map[string]string{"Status": "Created"})
}

func StopOneOffPod(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	oneoff := params["oneoff"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if rt.OneOffExists(space, oneoff) {
		rt.DeletePod(space, oneoff)
	}

	r.JSON(http.StatusOK, map[string]string{"Status": "OK"})
}
