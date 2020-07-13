package deployment

// V2 Functions

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"region-api/app"
	"region-api/config"
	ingress "region-api/router"
	runtime "region-api/runtime"
	"region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/lib/pq"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	uuid "github.com/nu7hatch/gouuid"
)

// Check to see if a given deployment exists in the database
func checkDeployment(db *sql.DB, name string, space string) (bool, error) {
	var exists bool
	query := "select exists(select 1 from v2.deployments where name = $1 and space = $2)"
	if err := db.QueryRow(query, name, space).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// UpdateDeploymentPlanV2 - V2 version of space.UpdateAppPlan
// (original: "space/app.go")
func UpdateDeploymentPlanV2(db *sql.DB, params martini.Params, deployment structs.AppDeploymentSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	name := params["deployment"]
	space := params["space"]

	if _, err := db.Exec("UPDATE v2.deployments SET plan=$1 WHERE name=$2 AND space=$3", deployment.Plan, name, space); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{
		Status:  http.StatusOK,
		Message: "Deployment: " + name + "-" + space + " updated to use " + deployment.Plan + " plan",
	})
}

// UpdateDeploymentHealthCheckV2 - V2 version of space.UpdateAppHealthCheck
// (original: "space/app.go")
func UpdateDeploymentHealthCheckV2(db *sql.DB, params martini.Params, deployment structs.AppDeploymentSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	if !deployment.Healthcheck.Valid || deployment.Healthcheck.String == "" {
		utils.ReportInvalidRequest("healthcheck required", r)
		return
	}

	name := params["deployment"]
	space := params["space"]

	_, err := db.Exec("UPDATE v2.deployments SET healthcheck=$1 WHERE name=$2 AND space=$3", deployment.Healthcheck.String, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{
		Status:  http.StatusOK,
		Message: "Deployment: " + name + "-" + space + " updated to use " + deployment.Healthcheck.String + " healthcheck",
	})
}

// DeleteDeploymentHealthCheckV2 - V2 version of space.DeleteAppHealthCheck
// (original: "space/app.go")
func DeleteDeploymentHealthCheckV2(db *sql.DB, params martini.Params, r render.Render) {
	name := params["deployment"]
	space := params["space"]

	if _, err := db.Exec("UPDATE v2.deployments SET healthcheck=NULL WHERE name=$1 AND space=$2", name, space); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Deployment: " + name + "-" + space + " healthcheck removed"})
}

// DeleteDeploymentV2Handler - HTTP Handler for DeleteDeploymentV2
func DeleteDeploymentV2Handler(db *sql.DB, params martini.Params, r render.Render) {
	name := params["deployment"]
	space := params["space"]

	responseCode, err := DeleteDeploymentV2(db, name, space)
	if err != nil {
		if responseCode == 400 {
			utils.ReportInvalidRequest(err.Error(), r)
		} else {
			utils.ReportError(err, r)
		}
		return
	}

	r.JSON(responseCode, structs.Messagespec{Status: responseCode, Message: name + "-" + space + " removed"})
}

// DeleteDeploymentV2 - V2 version of space.DeleteAppV2
// (original: "space/app.go")
func DeleteDeploymentV2(db *sql.DB, name string, space string) (int, error) {
	exists, err := checkDeployment(db, name, space)
	if err != nil {
		return 500, err
	}

	if exists == false {
		return 400, errors.New("Invalid app or space name")
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		return 500, err
	}

	if err = rt.DeleteService(space, name); err != nil {
		return 500, err
	}

	if err = rt.DeleteDeployment(space, name); err != nil {
		return 500, err
	}

	rslist, err := rt.GetReplicas(space, name)
	// in a previous iteration we ignored the error, i'll do so here
	// but i need to put an if around it to prevent the nil rslist from
	// bombing out everything
	if err != nil {
		fmt.Println(err)
	}
	if err == nil {
		for _, rs := range rslist {
			err = rt.DeleteReplica(space, name, rs)
			if err != nil {
				log.Println("Failed to remove replica set: ", err)
			}
		}
	} else {
		log.Println("Error getting replica sets:", err)
	}

	var podlist []string
	podlist, err = rt.GetPods(space, name)
	if err != nil {
		fmt.Println(err)
	}
	// in a previous iteration we ignored the error, i'll do so here
	// but i need to put an if around it to prevent the nil podlist from
	// bombing out everything
	if err == nil {
		for _, pod := range podlist {
			err = rt.DeletePod(space, pod)
			if err != nil {
				log.Println("Failed to remove pod: ", err)
			}
		}
	} else {
		log.Println("Error getting pods to remove:", err)
	}
	_, err = db.Exec("DELETE from appbindings where space=$1 and appname=$2", space, name)
	if err != nil {
		return 500, err
	}

	_, err = db.Exec("DELETE from v2.deployments where space=$1 and name=$2", space, name)
	if err != nil {
		return 500, err
	}

	return http.StatusOK, nil
}

// ScaleDeploymentV2 - V2 version of space.ScaleApp
// (original: "space/app.go")
func ScaleDeploymentV2(db *sql.DB, params martini.Params, deployment structs.AppDeploymentSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	name := params["deployment"]
	space := params["space"]

	var instances int
	if !deployment.Instances.Valid || deployment.Instances.Int64 < 0 {
		utils.ReportInvalidRequest("Instances missing or invalid", r)
		return
	}

	instances = int(deployment.Instances.Int64)

	if _, err := db.Exec("update v2.deployments set instances=$3 where space=$1 and name=$2", space, name, instances); err != nil {
		utils.ReportError(err, r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.Scale(space, name, instances); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusAccepted, structs.Messagespec{Status: http.StatusAccepted, Message: "Instances updated"})
}

// DeleteSpaceV2 - V2 version of space.Deletespace
// (original: "space/space.go")
func DeleteSpaceV2(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]

	if space == "" {
		utils.ReportInvalidRequest("The space was blank or invalid.", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	pods, err := rt.GetPodsBySpace(space)
	if err != nil && err.Error() != "space does not exist" {
		utils.ReportError(err, r)
		return
	} else if err == nil {
		if len(pods.Items) != 0 {
			r.JSON(http.StatusConflict, structs.Messagespec{Status: http.StatusConflict, Message: "The space cannot be deleted as it still has pods in it."})
			return
		}
	}

	var appsCount int
	if err = db.QueryRow("select count(*) from v2.deployments where space = $1", space).Scan(&appsCount); err != nil {
		utils.ReportError(err, r)
		return
	}

	if appsCount > 0 {
		r.JSON(http.StatusConflict, structs.Messagespec{Status: http.StatusConflict, Message: "The space cannot be deleted as it still has apps in it."})
		return
	}

	// this must happen after GetRuntimeFor.
	if _, err = db.Exec("delete from spaces where name = $1", space); err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.DeleteSpace(space); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "space deleted"})
}

// DescribeSpaceV2 - Get a list of all deployments for a space
func DescribeSpaceV2(db *sql.DB, params martini.Params, r render.Render) {
	var deployments []structs.AppDeploymentSpec
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	rows, err := db.Query("select name from v2.deployments where space = $1", space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			utils.ReportError(err, r)
			return
		}

		var deployment structs.AppDeploymentSpec

		deploymentQuery := "select appid, name, space, instances, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where name = $1 and space = $2"
		if err = db.QueryRow(deploymentQuery, name, space).Scan(
			&deployment.AppID,
			&deployment.Name,
			&deployment.Space,
			&deployment.Instances,
			&deployment.Plan,
			&deployment.Healthcheck,
		); err != nil {
			utils.ReportError(err, r)
			return
		}

		// Retrieve bindings for the deployment
		var bindings []structs.Bindspec
		var bindtype string
		var bindname string
		crows, err := db.Query("select bindtype, bindname from appbindings where appname=$1 and space=$2", name, space)
		defer crows.Close()
		for crows.Next() {
			if err := crows.Scan(&bindtype, &bindname); err != nil {
				utils.ReportError(err, r)
				return
			}
			bindings = append(bindings, structs.Bindspec{App: name, Bindtype: bindtype, Bindname: bindname, Space: space})
		}
		deployment.Bindings = bindings

		// Retrieve current image for deployment
		currentimage, err := rt.GetCurrentImage(deployment.Name, deployment.Space)
		if err != nil {
			if err.Error() == "deployment not found" {
				// if there has yet to be a deployment we'll get a not found error,
				// just set the image to blank and keep moving.
				currentimage = ""
			} else {
				utils.ReportError(err, r)
				return
			}
		}

		deployment.Image = structs.PrettyNullString{NullString: sql.NullString{
			String: currentimage,
			Valid:  currentimage != "",
		}}

		deployments = append(deployments, deployment)
	}

	r.JSON(http.StatusOK, deployments)
}

// DescribeDeploymentV2 - Get details about a specific deployment (based on name, space)
func DescribeDeploymentV2(db *sql.DB, params martini.Params, r render.Render) {
	name := params["deployment"]
	space := params["space"]

	exists, err := checkDeployment(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if exists == false {
		utils.ReportNotFoundError(r)
		return
	}

	var deployment structs.AppDeploymentSpec

	deploymentQuery := "select appid, name, space, instances, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where name = $1 and space = $2"
	if err = db.QueryRow(deploymentQuery, name, space).Scan(
		&deployment.AppID,
		&deployment.Name,
		&deployment.Space,
		&deployment.Instances,
		&deployment.Plan,
		&deployment.Healthcheck,
	); err != nil {
		utils.ReportError(err, r)
		return
	}

	// Retrieve bindings for the deployment
	var bindings []structs.Bindspec
	var bindtype string
	var bindname string
	crows, err := db.Query("select bindtype, bindname from appbindings where appname=$1 and space=$2", name, space)
	defer crows.Close()
	for crows.Next() {
		if err := crows.Scan(&bindtype, &bindname); err != nil {
			utils.ReportError(err, r)
			return
		}
		bindings = append(bindings, structs.Bindspec{App: name, Bindtype: bindtype, Bindname: bindname, Space: space})
	}
	deployment.Bindings = bindings

	// Retrieve current image for deployment
	rt, err := runtime.GetRuntimeFor(db, deployment.Space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	currentimage, err := rt.GetCurrentImage(deployment.Name, deployment.Space)
	if err != nil {
		if err.Error() == "deployment not found" {
			// if there has yet to be a deployment we'll get a not found error,
			// just set the image to blank and keep moving.
			currentimage = ""
		} else {
			utils.ReportError(err, r)
			return
		}
	}

	deployment.Image = structs.PrettyNullString{NullString: sql.NullString{
		String: currentimage,
		Valid:  currentimage != "",
	}}

	r.JSON(http.StatusOK, deployment)
}

func getConfigVars(db *sql.DB, appname string, space string) ([]structs.EnvVar, error) {
	// Get bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, appname)
	if err != nil {
		return nil, err
	}

	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		return nil, err
	}

	// Assemble config -- akkeris "built in config", "user defined config vars", "service configvars"
	elist := app.AddAkkerisConfigVars(appname, space)
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}

	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, appname, space, appbindings)
	if err != nil {
		return nil, err
	}

	for _, e := range servicevars {
		elist = append(elist, e)
	}
	return elist, nil
}

// GetAllConfigVarsV2 - Get all config vars for a deployment
func GetAllConfigVarsV2(db *sql.DB, params martini.Params, r render.Render) {
	name := params["deployment"]
	space := params["space"]

	exists, err := checkDeployment(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if exists == false {
		utils.ReportNotFoundError(r)
		return
	}

	configList, err := getConfigVars(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(201, configList)
}

// AddDeploymentV2 - V2 version of space.AddApp
// (original: "space/app.go")
func AddDeploymentV2(db *sql.DB, params martini.Params, deployment structs.AppDeploymentSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	if !deployment.AppID.Valid || deployment.AppID.String == "" {
		utils.ReportInvalidRequest("App ID is required", r)
		return
	}

	name := params["deployment"]
	space := params["space"]

	exists, err := checkDeployment(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if exists {
		utils.ReportInvalidRequest("Deployment already exists", r)
		return
	}

	var healthcheck *string
	if deployment.Healthcheck.String == "" {
		healthcheck = nil
	} else {
		healthcheck = &deployment.Healthcheck.String
	}

	insertQuery := "insert into v2.deployments(name, space, plan, instances, healthcheck, appid) values($1, $2, $3, $4, $5, $6) returning name"
	inserterr := db.QueryRow(insertQuery, name, space, deployment.Plan, int(deployment.Instances.Int64), healthcheck, deployment.AppID.String).Scan(&name)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}

	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "Deployment record created"})
}

// DeploymentV2Handler - HTTP handler for DeploymentV2
func DeploymentV2Handler(db *sql.DB, payload structs.DeploySpecV2, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	response, responseCode, err := DeploymentV2(db, payload)
	if err != nil {
		if responseCode == 400 {
			utils.ReportInvalidRequest(err.Error(), r)
		} else {
			utils.ReportError(err, r)
		}
		return
	}

	r.JSON(responseCode, response)
}

// DeploymentV2 - Deploy (or redeploy) a deployment
func DeploymentV2(db *sql.DB, payload structs.DeploySpecV2) (structs.Deployresponse, int, error) {
	var (
		deployresponse structs.Deployresponse
		nullPort       sql.NullInt64
		port           int
		instances      int
		plan           string
		healthcheck    string
	)

	isOneOff := payload.OneOff

	// Validate input
	if payload.Name == "" {
		return deployresponse, 400, errors.New("Deployment Name can not be blank")
	}
	if payload.Space == "" {
		return deployresponse, 400, errors.New("Space Name can not be blank")
	}
	if payload.Image == "" {
		return deployresponse, 400, errors.New("Image must be specified")
	}
	if !(strings.Contains(payload.Image, ":")) {
		return deployresponse, 400, errors.New("Image must contain tag")
	}

	rt, err := runtime.GetRuntimeFor(db, payload.Space)
	if err != nil {
		return deployresponse, 500, err
	}

	name := payload.Name
	space := payload.Space
	image := strings.Split(payload.Image, ":")[0]
	imageTag := strings.Split(payload.Image, ":")[1]

	deploymentQuery := "select instances, port, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where name = $1 and space = $2"
	if err = db.QueryRow(deploymentQuery, name, space).Scan(&instances, &nullPort, &plan, &healthcheck); err != nil {
		return deployresponse, 500, err
	}

	if nullPort.Valid {
		port = int(nullPort.Int64)
	} else {
		port = 0
	}

	// Get bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, name)
	if err != nil {
		return deployresponse, 500, err
	}

	// Get memory limits
	memorylimit, memoryrequest, err := app.GetMemoryLimits(db, plan)
	if err != nil {
		return deployresponse, 500, err
	}

	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		return deployresponse, 500, err
	}

	// Assemble config -- akkeris "built in config", "user defined config vars", "service configvars"
	elist := app.AddAkkerisConfigVars(name, space)
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}

	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, name, space, appbindings)
	if err != nil {
		return deployresponse, 500, err
	}
	for _, e := range servicevars {
		elist = append(elist, e)
	}

	// We have everything we need to create a one-off pod.
	if isOneOff {
		if rt.OneOffExists(payload.Space, payload.Name) {
			rt.DeletePod(payload.Space, payload.Name)
		}
		if err = rt.CreateOneOffPod(&structs.Deployment{
			Space:         space,
			App:           name,
			Amount:        instances,
			ConfigVars:    elist,
			HealthCheck:   healthcheck,
			MemoryRequest: memoryrequest,
			MemoryLimit:   memorylimit,
			Image:         image,
			Tag:           imageTag,
		}); err != nil {
			fmt.Println("Error creating a one off pod!")
			return deployresponse, 500, err
		}
		deployresponse.Controller = "Deployment Created"
		deployresponse.Service = "Service not required"
		return deployresponse, 201, nil
	}

	isInternal, err := utils.IsInternalSpace(db, space)
	if err != nil {
		return deployresponse, 500, err
	}

	if payload.Labels == nil {
		payload.Labels = make(map[string]string)
	}

	plantype, err := app.GetPlanType(db, plan)
	if err != nil {
		return deployresponse, 500, err
	}

	if plantype != nil && *plantype != "" {
		payload.Labels["akkeris.io/plan-type"] = *plantype
	}

	payload.Labels["akkeris.io/plan"] = plan
	payload.Labels["akkeris.io/internal"] = strconv.FormatBool(isInternal)
	payload.Labels["akkeris.io/http2"] = strconv.FormatBool(payload.Features.Http2EndToEndService)

	// Via heuristics and rules, determine and/or override ports and configvars
	var finalport int
	finalport = port
	if configvars["PORT"] != "" {
		holdport, _ := strconv.Atoi(configvars["PORT"])
		finalport = holdport
	}
	if payload.Port != 0 {
		finalport = payload.Port
		holdport := strconv.Itoa(payload.Port)
		configvars["PORT"] = holdport
	}
	if payload.Port == 0 && configvars["PORT"] == "" && port == 0 {
		finalport = 4747
		configvars["PORT"] = "4747"
	}

	// Set revision history limit
	var revisionhistorylimit int
	revisionhistorylimit = 10
	if os.Getenv("REVISION_HISTORY_LIMIT") != "" {
		revisionhistorylimit, err = strconv.Atoi(os.Getenv("REVISION_HISTORY_LIMIT"))
		if err != nil {
			log.Println("The env REVISION_HISTORY_LIMIT was set but was an invalid value, must be whole positive number.")
			log.Println(err)
			revisionhistorylimit = 10
		}
	}

	appIngress, err := ingress.GetAppIngress(db, isInternal)
	if err != nil {
		return deployresponse, 500, err
	}
	siteIngress, err := ingress.GetSiteIngress(db, isInternal)
	if err != nil {
		return deployresponse, 500, err
	}

	// Create deployment
	var deployment structs.Deployment
	deployment.Labels = payload.Labels
	deployment.Space = space
	deployment.App = name
	deployment.Port = finalport
	deployment.Amount = instances
	deployment.ConfigVars = elist
	deployment.HealthCheck = healthcheck
	deployment.MemoryRequest = memoryrequest
	deployment.MemoryLimit = memorylimit
	deployment.Image = image
	deployment.Tag = imageTag
	deployment.RevisionHistoryLimit = revisionhistorylimit
	if len(payload.Command) > 0 {
		deployment.Command = payload.Command

	}
	if (structs.Features{}) != payload.Features {
		deployment.Features = payload.Features
	}
	if len(payload.Filters) > 0 {
		// Inject istio sidecar for http filters
		deployment.Features.IstioInject = true
	}

	if plantype != nil && *plantype != "" && *plantype != "general" {
		deployment.PlanType = *plantype
	}

	deploymentExists, err := rt.DeploymentExists(space, name)
	if err != nil {
		return deployresponse, 500, err
	}
	serviceExists, err := rt.ServiceExists(space, name)
	if err != nil {
		return deployresponse, 500, err
	}

	// Do not write to cluster above this line, everything below should apply changes,
	// everything above should do sanity checks, this helps prevent "half" deployments
	// by minimizing resource after the first write

	if !deploymentExists {
		if err = rt.CreateDeployment(&deployment); err != nil {
			return deployresponse, 500, err
		}
	} else {
		if err = rt.UpdateDeployment(&deployment); err != nil {
			return deployresponse, 500, err
		}
	}

	// Any deployment features requiring istio transitioned ingresses should
	// be marked here. Only apply this to the web dyno types.
	appFQDN := name + "-" + space
	if space == "default" {
		appFQDN = name
	}
	if isInternal {
		appFQDN = appFQDN + "." + os.Getenv("INTERNAL_DOMAIN")
	} else {
		appFQDN = appFQDN + "." + os.Getenv("EXTERNAL_DOMAIN")
	}

	// Create/update service
	if finalport != -1 {
		if !serviceExists {
			if err := rt.CreateService(space, name, finalport, payload.Labels, payload.Features); err != nil {
				return deployresponse, 500, err
			}
		} else {
			if err := rt.UpdateService(space, name, finalport, payload.Labels, payload.Features); err != nil {
				return deployresponse, 500, err
			}
		}

		// Apply the HTTP filters
		foundJwtFilter := false
		foundCorsFilter := false
		foundCspFilter := false
		for _, filter := range payload.Filters {
			if filter.Type == "jwt" {
				issuer := ""
				jwksURI := ""
				audiences := make([]string, 0)
				excludes := make([]string, 0)
				includes := make([]string, 0)
				if val, ok := filter.Data["issuer"]; ok {
					issuer = val
				}
				if val, ok := filter.Data["jwks_uri"]; ok {
					jwksURI = val
				}
				if val, ok := filter.Data["audiences"]; ok {
					if val != "" {
						audiences = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["excludes"]; ok {
					if val != "" {
						excludes = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["includes"]; ok {
					if val != "" {
						includes = strings.Split(val, ",")
					}
				}
				if jwksURI == "" {
					fmt.Printf("WARNING: Invalid jwt configuration, uri was not valid: %s\n", jwksURI)
				} else {
					foundJwtFilter = true
					if err := appIngress.InstallOrUpdateJWTAuthFilter(name, space, appFQDN, int64(finalport), issuer, jwksURI, audiences, excludes, includes); err != nil {
						fmt.Printf("WARNING: There was an error installing or updating JWT Auth filter: %s\n", err.Error())
					}
				}
			} else if filter.Type == "cors" {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Adding CORS filter %#+v\n", filter)
				}
				allowOrigin := make([]string, 0)
				allowMethods := make([]string, 0)
				allowHeaders := make([]string, 0)
				exposeHeaders := make([]string, 0)
				maxAge := time.Second * 86400
				allowCredentials := false
				if val, ok := filter.Data["allow_origin"]; ok {
					if val != "" {
						allowOrigin = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["allow_methods"]; ok {
					if val != "" {
						allowMethods = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["allow_headers"]; ok {
					if val != "" {
						allowHeaders = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["expose_headers"]; ok {
					if val != "" {
						exposeHeaders = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["max_age"]; ok {
					age, err := strconv.ParseInt(val, 10, 32)
					if err == nil {
						maxAge = time.Second * time.Duration(age)
					} else {
						fmt.Printf("WARNING: Unable to convert max_age to value %s\n", val)
					}
				}
				if val, ok := filter.Data["allow_credentials"]; ok {
					if val == "true" {
						allowCredentials = true
					} else {
						allowCredentials = false
					}
				}
				if err := appIngress.InstallOrUpdateCORSAuthFilter(name+"-"+space, "/", allowOrigin, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials); err != nil {
					fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
				} else {
					foundCorsFilter = true
				}
				routes, err := ingress.GetPathsByApp(db, name, space)
				if err == nil {
					for _, route := range routes {
						if err := siteIngress.InstallOrUpdateCORSAuthFilter(route.Domain, route.Path, allowOrigin, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials); err != nil {
							fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter on site: %s: %s\n", route.Domain, err.Error())
						}
					}
				} else {
					fmt.Printf("WARNING: There was an error trying to pull the routes for an app to install the CORS auth filters on: %s\n", err.Error())
				}
			} else if filter.Type == "csp" {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Adding CSP filter %#+v\n", filter)
				}
				policy := ""
				if val, ok := filter.Data["policy"]; ok {
					policy = val
				}
				if policy != "" {
					if err := appIngress.InstallOrUpdateCSPFilter(name+"-"+space, "/", policy); err != nil {
						fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
					} else {
						foundCspFilter = true
					}
					routes, err := ingress.GetPathsByApp(db, name, space)
					if err == nil {
						for _, route := range routes {
							if err := siteIngress.InstallOrUpdateCSPFilter(route.Domain, route.Path, policy); err != nil {
								fmt.Printf("WARNING: There was an error installing or updating CSP filter on site: %s: %s\n", route.Domain, err.Error())
							}
						}
					} else {
						fmt.Printf("WARNING: There was an error trying to pull the routes for an app to install the CSP filters on: %s\n", err.Error())
					}
				}
			} else {
				fmt.Printf("WARNING: Unknown filter type: %s\n", filter.Type)
			}
		}

		// If we don't have a CORS filter remove it from the app and any sites it may be associated with.
		// this is effectively a no-op if there is no CORS auth filter in the first place
		if !foundCorsFilter {
			if err := appIngress.DeleteCORSAuthFilter(name+"-"+space, "/"); err != nil {
				fmt.Printf("WARNING: There was an error removing the CORS auth filter from the app: %s\n", err.Error())
			}
			routes, err := ingress.GetPathsByApp(db, name, space)
			if err == nil {
				for _, route := range routes {
					if err := siteIngress.DeleteCORSAuthFilter(route.Domain, route.Path); err != nil {
						fmt.Printf("WARNING: There was an error removing CORS Auth filter on site: %s: %s\n", route.Domain, err.Error())
					}
				}
			} else {
				fmt.Printf("WARNING: There was an error trying to pull the routes for an app to install the CORS auth filters on: %s\n", err.Error())
			}
		}
		if !foundCspFilter {
			if err := appIngress.DeleteCSPFilter(name+"-"+space, "/"); err != nil {
				fmt.Printf("WARNING: There was an error removing the CSP filter from the app: %s\n", err.Error())
			}
			routes, err := ingress.GetPathsByApp(db, name, space)
			if err == nil {
				for _, route := range routes {
					if err := siteIngress.DeleteCSPFilter(route.Domain, route.Path); err != nil {
						fmt.Printf("WARNING: There was an error removing CSP filter on site: %s: %s\n", route.Domain, err.Error())
					}
				}
			} else {
				fmt.Printf("WARNING: There was an error trying to pull the routes for an app to install the CSP filters on: %s\n", err.Error())
			}
		}
		if !foundJwtFilter {
			if err := appIngress.DeleteJWTAuthFilter(name, space, appFQDN, int64(finalport)); err != nil {
				fmt.Printf("WARNING: There was an error removing the JWT auth filter: %s\n", err.Error())
			}
		}
	}

	// Prepare the response back
	if !deploymentExists {
		if finalport != -1 {
			deployresponse.Service = "Service Created"
		}
		if port == -1 {
			deployresponse.Service = "Service not required"
		}
		deployresponse.Controller = "Deployment Created"
	} else {
		if finalport != -1 {
			deployresponse.Service = "Service Created"
		}
		if finalport == -1 {
			deployresponse.Service = "Service not required"
		}
		deployresponse.Controller = "Deployment Updated"
	}
	return deployresponse, 201, nil
}

// deploymentExists - Returns whether or not a deployment with a given name and space exists
func deploymentExists(db *sql.DB, name string, space string) (bool, error) {
	deploymentExistsQuery := "select exists(select from v2.deployments where name = $1 and space = $2)"

	var exists bool
	if err := db.QueryRow(deploymentExistsQuery, name, space).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// // deleteDeploymentV2 - Copy of function in space package. !!Needs to be moved to a common package!!
// func deleteDeploymentV2(db *sql.DB, name string, space string) (int, error) {
// 	rt, err := runtime.GetRuntimeFor(db, space)
// 	if err != nil {
// 		return 500, err
// 	}

// 	if err = rt.DeleteService(space, name); err != nil {
// 		return 500, err
// 	}

// 	if err = rt.DeleteDeployment(space, name); err != nil {
// 		return 500, err
// 	}

// 	rslist, err := rt.GetReplicas(space, name)
// 	// in a previous iteration we ignored the error, i'll do so here
// 	// but i need to put an if around it to prevent the nil rslist from
// 	// bombing out everything
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	if err == nil {
// 		for _, rs := range rslist {
// 			err = rt.DeleteReplica(space, name, rs)
// 			if err != nil {
// 				log.Println("Failed to remove replica set: ", err)
// 			}
// 		}
// 	} else {
// 		log.Println("Error getting replica sets:", err)
// 	}

// 	var podlist []string
// 	podlist, err = rt.GetPods(space, name)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	// in a previous iteration we ignored the error, i'll do so here
// 	// but i need to put an if around it to prevent the nil podlist from
// 	// bombing out everything
// 	if err == nil {
// 		for _, pod := range podlist {
// 			err = rt.DeletePod(space, pod)
// 			if err != nil {
// 				log.Println("Failed to remove pod: ", err)
// 			}
// 		}
// 	} else {
// 		log.Println("Error getting pods to remove:", err)
// 	}
// 	_, err = db.Exec("DELETE from appbindings where space=$1 and appname=$2", space, name)
// 	if err != nil {
// 		return 500, err
// 	}

// 	_, err = db.Exec("DELETE from v2.deployments where space=$1 and name=$2", space, name)
// 	if err != nil {
// 		return 500, err
// 	}

// 	return http.StatusOK, nil
// }

// DeleteAppV2 - V2 version of app.Deleteapp
// (original: "app/deleteapp.go")
// This should delete ALL deployments with a given app ID
func DeleteAppV2(db *sql.DB, params martini.Params, r render.Render) {
	appid := params["appid"]

	// Make sure that UUID is valid and we have deployments for it
	if _, err := uuid.ParseHex(appid); err != nil {
		utils.ReportInvalidRequest("Invalid app UUID", r)
	}

	var count int
	if err := db.QueryRow("select count(*) from v2.deployments where appid = $1", appid).Scan(&count); err != nil {
		utils.ReportError(err, r)
		return
	}

	if count < 1 {
		utils.ReportNotFoundError(r)
		return
	}

	// Get all deployments for the app ID
	rows, err := db.Query("select name, space from v2.deployments where appid = $1", appid)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()

	type deletionError struct {
		DeploymentName  string `json:"deploymentName"`
		DeploymentSpace string `json:"deploymentSpace"`
		Error           string `json:"error"`
	}

	var deletionErrors []deletionError

	for rows.Next() {
		var dName string
		var dSpace string
		if err = rows.Scan(&dName, &dSpace); err != nil {
			utils.ReportError(err, r)
			return
		}

		_, err := DeleteDeploymentV2(db, dName, dSpace)
		if err != nil {
			deletionErrors = append(deletionErrors, deletionError{
				DeploymentName:  dName,
				DeploymentSpace: dSpace,
				Error:           err.Error(),
			})
		} else {
			_, err2 := db.Exec("delete from v2.deployments where name = $1 and space = $2", dName, dSpace)
			if err2 != nil {
				deletionErrors = append(deletionErrors, deletionError{
					DeploymentName:  dName,
					DeploymentSpace: dSpace,
					Error:           err2.Error(),
				})
			}
		}
	}

	if len(deletionErrors) > 0 {
		r.JSON(http.StatusInternalServerError, struct {
			Status  int             `json:"status"`
			Message string          `json:"message"`
			Errors  []deletionError `json:"errors"`
		}{
			http.StatusInternalServerError,
			"One or more deployments for the specified app could not be deleted",
			deletionErrors,
		})
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "All deployments for the specified app were successfully deleted."})
}

// getAppIDFromName - Given a deployment name and space, get the associated app ID
func getAppIDFromName(db *sql.DB, name string, space string) (string, error) {
	appidFromNameQuery := "select appid from v2.deployments where name = $1 and space = $2 and appid is not null"

	var appid sql.NullString
	if err := db.QueryRow(appidFromNameQuery, name, space).Scan(&appid); err != nil {
		if err.Error() == "sql: no rows in result set" {
			// Apps created with v1 schema might not have an appid set
			return "", errors.New("Invalid app or space name, or created with v1 schema")
		}
		return "", err
	}
	return appid.String, nil
}

func getBindings(db *sql.DB, appname string, space string) (b []structs.Bindspec, err error) {
	var bindings []structs.Bindspec
	var bindtype string
	var bindname string
	crows, err := db.Query("select bindtype, bindname from appbindings where appname=$1 and space=$2", appname, space)
	defer crows.Close()
	for crows.Next() {
		err := crows.Scan(&bindtype, &bindname)
		if err != nil {
			utils.LogError("", err)
			return bindings, err
		}
		bindings = append(bindings, structs.Bindspec{App: appname, Bindtype: bindtype, Bindname: bindname, Space: space})
	}
	return bindings, nil
}

// GetDeployments - Given an app ID, return information about all deployments for that app
func GetDeployments(db *sql.DB, appid string, rt runtime.Runtime) (ads []structs.AppDeploymentSpec, err error) {
	deploymentsQuery := "select appid, name, space, instances, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where appid = $1"

	var deployments []structs.AppDeploymentSpec
	results, err := db.Query(deploymentsQuery, appid)
	if err != nil {
		utils.LogError("", err)
		return nil, err
	}
	defer results.Close()

	// Assemble all neccesary data for each deployment
	for results.Next() {
		var deployment structs.AppDeploymentSpec
		err := results.Scan(&deployment.AppID, &deployment.Name, &deployment.Space, &deployment.Instances, &deployment.Plan, &deployment.Healthcheck)
		if err != nil {
			utils.LogError("", err)
			return nil, err
		}
		// Retrieve bindings for the deployment
		bindings, err := getBindings(db, deployment.Name, deployment.Space)
		if err != nil {
			utils.LogError("", err)
			return nil, err
		}
		deployment.Bindings = bindings

		// Retrieve current image for deployment
		if rt == nil {
			rt, err = runtime.GetRuntimeFor(db, deployment.Space)
			if err != nil {
				return nil, err
			}
		}

		currentimage, err := rt.GetCurrentImage(deployment.Name, deployment.Space)
		if err != nil {
			if err.Error() == "deployment not found" {
				// if there has yet to be a deployment we'll get a not found error,
				// just set the image to blank and keep moving.
				currentimage = ""
			} else {
				return nil, err
			}
		}

		deployment.Image = structs.PrettyNullString{NullString: sql.NullString{
			String: currentimage,
			Valid:  currentimage != "",
		}}
		deployments = append(deployments, deployment)
	}
	return deployments, nil
}

// DescribeAppV2 - Get all deployments for an app based on app ID
// Todo: Move this back to app package?
func DescribeAppV2(db *sql.DB, params martini.Params, r render.Render) {
	appid := params["appid"]
	var err error

	if _, err := uuid.ParseHex(appid); err != nil {
		utils.ReportInvalidRequest("Invalid app UUID", r)
		return
	}

	deployments, err := GetDeployments(db, appid, nil)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, deployments)
}

// ListAppsV2 - Get a list of all Akkeris apps and their associated deployments
// Todo: Move this back to app package?
func ListAppsV2(db *sql.DB, params martini.Params, r render.Render) {
	rows, err := db.Query("select coalesce(appid,'00000000-0000-0000-0000-000000000000') as appid, array_agg(name || '-' || space) deployments from v2.deployments group by appid")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()

	type appInfo struct {
		ID          string   `json:"id"`          // App ID
		Deployments []string `json:"deployments"` // List of deployments associated with the app
	}

	var appList []appInfo

	for rows.Next() {
		var app appInfo
		if err = rows.Scan(&app.ID, pq.Array(&app.Deployments)); err != nil {
			utils.ReportError(err, r)
			return
		}
		appList = append(appList, app)
	}

	if err = rows.Err(); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, appList)
}

// RenameAppV2 - Rename all deployments for an app
func RenameAppV2(db *sql.DB, params martini.Params, renamespec structs.AppRenameSpec, r render.Render) {
	// function stub
}
