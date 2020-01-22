package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"log"
	"os"
	config "region-api/config"
	runtime "region-api/runtime"
	service "region-api/service"
	structs "region-api/structs"
	ingress "region-api/router"
	utils "region-api/utils"
	"strconv"
	"strings"
	"fmt"
	"time"
)

func AddAkkerisConfigVars(appname string, space string) []structs.EnvVar {
	elist := make([]structs.EnvVar, 0)
	elist = append(elist, structs.EnvVar{
		Name:"ALAMO_SPACE",
		Value:space,
	})
	elist = append(elist, structs.EnvVar{
		Name:"AKKERIS_SPACE",
		Value:space,
	})
	elist = append(elist, structs.EnvVar{
		Name:"ALAMO_DEPLOYMENT",
		Value:appname,
	})
	elist = append(elist, structs.EnvVar{
		Name:"AKKERIS_DEPLOYMENT",
		Value:appname,
	})
	elist = append(elist, structs.EnvVar{
		Name:"ALAMO_APPLICATION",
		Value:appname + "-" + space,
	})
	elist = append(elist, structs.EnvVar{
		Name:"AKKERIS_APPLICATION",
		Value:appname + "-" + space,
	})
	return elist
}

func GetMemoryLimits(db *sql.DB, plan string) (memorylimit string, memoryrequest string, e error) {
	e = db.QueryRow("SELECT memrequest,memlimit from plans where name=$1", plan).Scan(&memoryrequest, &memorylimit)
	if e != nil {
		return "", "", e
	}
	return memorylimit, memoryrequest, nil
}

func GetServiceConfigVars(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	appname := params["appname"]
	bindtype := params["bindtype"]
	bindname := params["bindname"]

	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, appname, space, []structs.Bindspec{structs.Bindspec{App: appname, Space: space, Bindtype: bindtype, Bindname: bindname}})
	if err != nil {
		utils.ReportError(err, r)
		return

	}
	r.JSON(201, servicevars)
}

func GetAllConfigVars(db *sql.DB, params martini.Params, r render.Render) {
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

//Deployment centralized
func Deployment(db *sql.DB, deploy1 structs.Deployspec, berr binding.Errors, r render.Render) {
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
	deploymentExists, err := rt.DeploymentExists(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
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
		if !deploymentExists {
			if _, err := rt.CreateService(space, appname, finalport, deploy1.Labels, deploy1.Features); err != nil {
				utils.ReportError(err, r)
				return
			}
		} else {
			if _, err := rt.UpdateService(space, appname, finalport, deploy1.Labels, deploy1.Features); err != nil {
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
					audiences = strings.Split(val, ",")
				}
				if val, ok := filter.Data["excludes"]; ok {
					excludes = strings.Split(val, ",")
				}
				if val, ok := filter.Data["includes"]; ok {
					includes = strings.Split(val, ",")
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
					allow_origin = strings.Split(val, ",")
				}
				if val, ok := filter.Data["allow_methods"]; ok {
					allow_methods = strings.Split(val, ",")
				}
				if val, ok := filter.Data["allow_headers"]; ok {
					allow_headers = strings.Split(val, ",")
				}
				if val, ok := filter.Data["expose_headers"]; ok {
					expose_headers = strings.Split(val, ",");
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
				if err := appIngress.InstallOrUpdateCORSAuthFilter(appname + "-" + space, "/", allow_origin, allow_methods, allow_headers, expose_headers, max_age, allow_credentials); err != nil {
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
			if err := appIngress.DeleteCORSAuthFilter(appname + "-" + space, "/"); err != nil {
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
