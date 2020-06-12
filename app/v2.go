package app

import (
	"database/sql"
	"errors"
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
	uuid "github.com/nu7hatch/gouuid"
)

// deploymentExists - Returns whether or not a deployment with a given name and space exists
func deploymentExists(db *sql.DB, name string, space string) (bool, error) {
	deploymentExistsQuery := "select exists(select from v2.deployments where name = $1 and space = $2)"

	var exists bool
	if err := db.QueryRow(deploymentExistsQuery, name, space).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// DeleteAppV2 - V2 version of app.Deleteapp
// (original: "app/deleteapp.go")
// This should delete ALL deployments with a given app ID
func DeleteAppV2(db *sql.DB, params martini.Params, r render.Render) {
	appid := params["appid"]
	if _, err := uuid.ParseHex(appid); err != nil {
		utils.ReportInvalidRequest("Invalid app UUID", r)
	}

	// _, err := db.Exec("DELETE from apps where name=$1", appname)
	// if err != nil {
	// 	utils.ReportError(err, r)
	// 	return
	// }
	// r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: appname + " deleted"})
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
					if err := appIngress.InstallOrUpdateJWTAuthFilter(appname, space, appFQDN, int64(finalport), issuer, jwksURI, audiences, excludes, includes); err != nil {
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
				if err := appIngress.InstallOrUpdateCORSAuthFilter(appname+"-"+space, "/", allowOrigin, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials); err != nil {
					fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
				} else {
					foundCorsFilter = true
				}
				routes, err := ingress.GetPathsByApp(db, appname, space)
				if err == nil {
					for _, route := range routes {
						if err := siteIngress.InstallOrUpdateCORSAuthFilter(route.Domain, route.Path, allowOrigin, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials); err != nil {
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

// GetAllConfigVarsV2 - Get all config vars for a deployment
// (original: "app/deployment.go")
func GetAllConfigVarsV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["appname"]
	space := params["space"]

	exists, err := deploymentExists(db, appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if exists == false {
		utils.ReportInvalidRequest(err.Error(), r)
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

		deployment.Image = sql.NullString{String: currentimage, Valid: currentimage != ""}
		deployments = append(deployments, deployment)
	}
	return deployments, nil
}

// DescribeAppV2 - Get all deployments for an app based on app ID
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

// ListAppsV2 - Return a list of all apps in the Deployments table
// (v1: "app/listapps.go")
func ListAppsV2(db *sql.DB, params martini.Params, r render.Render) {
	rows, err := db.Query("select concat(name, '-', space) from v2.deployments")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()

	var appList []string
	for rows.Next() {
		var app string
		err := rows.Scan(&app)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		appList = append(appList, app)
	}

	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Applist{Apps: appList})
}

func RenameAppV2(db *sql.DB, params martini.Params, renamespec structs.AppRenameSpec, r render.Render) {
	// function stub
}
