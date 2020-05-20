package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	config "region-api/config"
	ingress "region-api/router"
	runtime "region-api/runtime"
	service "region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

// DeleteAppV2 - V2 version of app.Deleteapp
// (original: "app/deleteapp.go")
func DeleteAppV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	var space string
	err := db.QueryRow("select space from spacesapps where appname = $1", appname).Scan(&space)
	if err == nil && space != "" {
		utils.ReportInvalidRequest("application still exists in spaces: "+space, r)
		return
	}
	_, err = db.Exec("DELETE from apps where name=$1", appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: appname + " deleted"})
}

// DeploymentV2 - V2 version of app.Deployment
// (original: "app/deployment.go")
func DeploymentV2(db *sql.DB, deploy1 structs.Deployspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	var repo string
	var tag string

	if deploy1.AppName == "" {
		utils.ReportInvalidRequest("Application Name can not be blank", r)
		return
	}
	if deploy1.Space == "" {
		utils.ReportInvalidRequest("Space Name can not be blank", r)
		return
	}
	if deploy1.Image == "" {
		utils.ReportInvalidRequest("Image must be specified", r)
		return
	}
	if deploy1.Image != "" && !(strings.Contains(deploy1.Image, ":")) {
		utils.ReportInvalidRequest("Image must contain tag", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, deploy1.Space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	appname := deploy1.AppName
	space := deploy1.Space
	repo = strings.Split(deploy1.Image, ":")[0]
	tag = strings.Split(deploy1.Image, ":")[1]

	var (
		appport     int
		instances   int
		plan        string
		healthcheck string
	)

	err = db.QueryRow("SELECT apps.port,spacesapps.instances,COALESCE(spacesapps.plan,'noplan') AS plan,COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from apps,spacesapps where apps.name=$1 and apps.name=spacesapps.appname and spacesapps.space=$2", appname, space).Scan(&appport, &instances, &plan, &healthcheck)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	appimage := repo
	apptag := tag

	// Get bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Get memory limits
	memorylimit, memoryrequest, err := GetMemoryLimits(db, plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

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

	if deploy1.Labels == nil {
		deploy1.Labels = make(map[string]string)
	}
	plantype, err := GetPlanType(db, plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if plantype != nil && *plantype != "" {
		deploy1.Labels["akkeris.io/plan-type"] = *plantype
	}
	deploy1.Labels["akkeris.io/plan"] = plan
	if internal {
		deploy1.Labels["akkeris.io/internal"] = "true"
	} else {
		deploy1.Labels["akkeris.io/internal"] = "false"
	}

	if deploy1.Features.Http2EndToEndService {
		deploy1.Labels["akkeris.io/http2"] = "true"
	} else {
		deploy1.Labels["akkeris.io/http2"] = "false"
	}

	// Via heuristics and rules, determine and/or override ports and configvars
	var finalport int
	finalport = appport
	if configvars["PORT"] != "" {
		holdport, _ := strconv.Atoi(configvars["PORT"])
		finalport = holdport
	}
	if deploy1.Port != 0 {
		finalport = deploy1.Port
		holdport := strconv.Itoa(deploy1.Port)
		configvars["PORT"] = holdport
	}
	if deploy1.Port == 0 && configvars["PORT"] == "" && appport == 0 {
		finalport = 4747
		configvars["PORT"] = "4747"
	}

	// Assemble config -- akkeris "built in config", "user defined config vars", "service configvars"
	elist := AddAkkerisConfigVars(appname, space)
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

	appIngress, err := ingress.GetAppIngress(db, internal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	siteIngress, err := ingress.GetSiteIngress(db, internal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Create deployment
	var deployment structs.Deployment
	deployment.Labels = deploy1.Labels
	deployment.Space = space
	deployment.App = appname
	deployment.Port = finalport
	deployment.Amount = instances
	deployment.ConfigVars = elist
	deployment.HealthCheck = healthcheck
	deployment.MemoryRequest = memoryrequest
	deployment.MemoryLimit = memorylimit
	deployment.Image = appimage
	deployment.Tag = apptag
	deployment.RevisionHistoryLimit = revisionhistorylimit
	if len(deploy1.Command) > 0 {
		deployment.Command = deploy1.Command

	}
	if (structs.Features{}) != deploy1.Features {
		deployment.Features = deploy1.Features
	}
	if len(deploy1.Filters) > 0 {
		// Inject istio sidecar for http filters
		deployment.Features.IstioInject = true
	}

	if plantype != nil && *plantype != "" && *plantype != "general" {
		deployment.PlanType = *plantype
	}

	deploymentExists, err := rt.DeploymentExists(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	serviceExists, err := rt.ServiceExists(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Do not write to cluster above this line, everything below should apply changes,
	// everything above should do sanity checks, this helps prevent "half" deployments
	// by minimizing resource after the first write

	if !deploymentExists {
		if err = rt.CreateDeployment(&deployment); err != nil {
			utils.ReportError(err, r)
			return
		}
	} else {
		if err = rt.UpdateDeployment(&deployment); err != nil {
			utils.ReportError(err, r)
			return
		}
	}

	// Any deployment features requiring istio transitioned ingresses should
	// be marked here. Only apply this to the web dyno types.
	appFQDN := appname + "-" + space
	if space == "default" {
		appFQDN = appname
	}
	if internal {
		appFQDN = appFQDN + "." + os.Getenv("INTERNAL_DOMAIN")
	} else {
		appFQDN = appFQDN + "." + os.Getenv("EXTERNAL_DOMAIN")
	}

	// Create/update service
	if finalport != -1 {
		if !serviceExists {
			if err := rt.CreateService(space, appname, finalport, deploy1.Labels, deploy1.Features); err != nil {
				utils.ReportError(err, r)
				return
			}
		} else {
			if err := rt.UpdateService(space, appname, finalport, deploy1.Labels, deploy1.Features); err != nil {
				utils.ReportError(err, r)
				return
			}
		}

		// Apply the HTTP filters
		foundJwtFilter := false
		foundCorsFilter := false
		for _, filter := range deploy1.Filters {
			if filter.Type == "jwt" {
				issuer := ""
				jwksUri := ""
				audiences := make([]string, 0)
				excludes := make([]string, 0)
				includes := make([]string, 0)
				if val, ok := filter.Data["issuer"]; ok {
					issuer = val
				}
				if val, ok := filter.Data["jwks_uri"]; ok {
					jwksUri = val
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
				if jwksUri == "" {
					fmt.Printf("WARNING: Invalid jwt configuration, uri was not valid: %s\n", jwksUri)
				} else {
					foundJwtFilter = true
					if err := appIngress.InstallOrUpdateJWTAuthFilter(appname, space, appFQDN, int64(finalport), issuer, jwksUri, audiences, excludes, includes); err != nil {
						fmt.Printf("WARNING: There was an error installing or updating JWT Auth filter: %s\n", err.Error())
					}
				}
			} else if filter.Type == "cors" {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Adding CORS filter %#+v\n", filter)
				}
				allow_origin := make([]string, 0)
				allow_methods := make([]string, 0)
				allow_headers := make([]string, 0)
				expose_headers := make([]string, 0)
				max_age := time.Second * 86400
				allow_credentials := false
				if val, ok := filter.Data["allow_origin"]; ok {
					if val != "" {
						allow_origin = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["allow_methods"]; ok {
					if val != "" {
						allow_methods = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["allow_headers"]; ok {
					if val != "" {
						allow_headers = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["expose_headers"]; ok {
					if val != "" {
						expose_headers = strings.Split(val, ",")
					}
				}
				if val, ok := filter.Data["max_age"]; ok {
					age, err := strconv.ParseInt(val, 10, 32)
					if err == nil {
						max_age = time.Second * time.Duration(age)
					} else {
						fmt.Printf("WARNING: Unable to convert max_age to value %s\n", val)
					}
				}
				if val, ok := filter.Data["allow_credentials"]; ok {
					if val == "true" {
						allow_credentials = true
					} else {
						allow_credentials = false
					}
				}
				if err := appIngress.InstallOrUpdateCORSAuthFilter(appname+"-"+space, "/", allow_origin, allow_methods, allow_headers, expose_headers, max_age, allow_credentials); err != nil {
					fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
				} else {
					foundCorsFilter = true
				}
				routes, err := ingress.GetPathsByApp(db, appname, space)
				if err == nil {
					for _, route := range routes {
						if err := siteIngress.InstallOrUpdateCORSAuthFilter(route.Domain, route.Path, allow_origin, allow_methods, allow_headers, expose_headers, max_age, allow_credentials); err != nil {
							fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter on site: %s: %s\n", route.Domain, err.Error())
						}
					}
				} else {
					fmt.Printf("WARNING: There was an error trying to pull the routes for an app to install the CORS auth filters on: %s\n", err.Error())
				}
			} else {
				fmt.Printf("WARNING: Unknown filter type: %s\n", filter.Type)
			}
		}

		// If we don't have a CORS filter remove it from the app and any sites it may be associated with.
		// this is effectively a no-op if there is no CORS auth filter in the first place
		if !foundCorsFilter {
			if err := appIngress.DeleteCORSAuthFilter(appname+"-"+space, "/"); err != nil {
				fmt.Printf("WARNING: There was an error removing the CORS auth filter from the app: %s\n", err.Error())
			}
			routes, err := ingress.GetPathsByApp(db, appname, space)
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
		if !foundJwtFilter {
			if err := appIngress.DeleteJWTAuthFilter(appname, space, appFQDN, int64(finalport)); err != nil {
				fmt.Printf("WARNING: There was an error removing the JWT auth filter: %s\n", err.Error())
			}
		}
	}

	// Prepare the response back
	var deployresponse structs.Deployresponse
	if !deploymentExists {
		if finalport != -1 {
			deployresponse.Service = "Service Created"
		}
		if appport == -1 {
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
	r.JSON(201, deployresponse)
}

// GetAllConfigVarsV2 - V2 version of app.Deleteapp
// (original: "app/deployment.go")
func GetAllConfigVarsV2(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	appname := params["appname"]

	var (
		appport     int
		instances   int
		plan        string
		healthcheck string
	)

	err := db.QueryRow("SELECT apps.port,spacesapps.instances,COALESCE(spacesapps.plan,'noplan') AS plan,COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from apps,spacesapps where apps.name=$1 and apps.name=spacesapps.appname and spacesapps.space=$2", appname, space).Scan(&appport, &instances, &plan, &healthcheck)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Get bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	// Assemble config -- akkeris "built in config", "user defined config vars", "service configvars"
	elist := AddAkkerisConfigVars(appname, space)
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
	r.JSON(201, elist)
}

// OneOffDeploymentV2 - V2 version of app.OneOffDeployment
// (original: "app/oneoff.go")
func OneOffDeploymentV2(db *sql.DB, oneoff1 structs.OneOffSpec, berr binding.Errors, r render.Render) {
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

// DescribeAppV2 - V2 version of app.Describeapp
// (original: "app/describeapp.go")
func DescribeAppV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	var (
		name string
		port int
	)
	err := db.QueryRow("select name, port from apps where name=$1", appname).Scan(&name, &port)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// To be backwards compatible with older systems lets fake a response back
			// this ... isn't ideal, but .. well..
			r.JSON(http.StatusOK, structs.Appspec{Name: "", Port: -1, Spaces: nil})
			return
		}
		utils.ReportError(err, r)
		return
	}
	spaceapps, err := getSpacesapps(db, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Appspec{Name: name, Port: port, Spaces: spaceapps})
}

// DescribeAppInSpaceV2 - V2 version of app.DescribeappInSpace
// (original: "app/describeapp.go")
func DescribeAppInSpaceV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	spacename := params["space"]

	var instances int
	var plan string
	var healthcheck string
	err := db.QueryRow("select appname, instances, coalesce(plan,'noplan') as plan, COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from spacesapps where space = $1 and appname = $2", spacename, appname).Scan(&appname, &instances, &plan, &healthcheck)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// To be backwards compatible with older systems lets fake a response back
			// this ... isn't ideal, but .. well..
			r.JSON(http.StatusOK, structs.Spaceappspec{Appname: appname, Space: spacename, Instances: 0, Plan: "", Healthcheck: "", Bindings: nil})
			return
		}
		utils.ReportError(err, r)
		return
	}
	bindings, _ := getBindings(db, appname, spacename)
	currentapp := structs.Spaceappspec{Appname: appname, Space: spacename, Instances: instances, Plan: plan, Healthcheck: healthcheck, Bindings: bindings}

	rt, err := runtime.GetRuntimeFor(db, spacename)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	currentimage, err := rt.GetCurrentImage(spacename, appname)
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
	currentapp.Image = currentimage
	r.JSON(http.StatusOK, currentapp)
}

// ListAppsV2 - V2 version of app.Listapps
// (original: "app/listapps.go")
func ListAppsV2(db *sql.DB, params martini.Params, r render.Render) {
	var name string
	rows, err := db.Query("select name from apps")
	defer rows.Close()
	var applist []string
	for rows.Next() {
		err := rows.Scan(&name)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		applist = append(applist, name)
	}
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Applist{Apps: applist})
}

// DescribeSpaceV2 - V2 version of app.Describespace
// (original: "app/describeapp.go")
func DescribeSpaceV2(db *sql.DB, params martini.Params, r render.Render) {
	var list []structs.Spaceappspec
	spacename := params["space"]

	var appname string
	var instances int
	var plan string
	var healthcheck string
	rows, err := db.Query("select appname, instances, coalesce(plan,'noplan') as plan, COALESCE(spacesapps.healthcheck,'tcp') AS healthcheck from spacesapps where space = $1", spacename)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&appname, &instances, &plan, &healthcheck)
		if err != nil {
			utils.ReportError(err, r)
			return
		}

		bindings, _ := getBindings(db, appname, spacename)
		list = append(list, structs.Spaceappspec{Appname: appname, Space: spacename, Instances: instances, Plan: plan, Healthcheck: healthcheck, Bindings: bindings})
	}
	r.JSON(http.StatusOK, list)
}