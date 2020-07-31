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

// getDeploymentInfo - Get database record for the deployment with the given name and space
func getDeploymentInfo(db *sql.DB, name string, space string) (structs.AppDeploymentSpec, error) {
	var d structs.AppDeploymentSpec

	deploymentQuery := "select appid, name, space, instances, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where name = $1 and space = $2"
	if err := db.QueryRow(deploymentQuery, name, space).Scan(
		&d.AppID,
		&d.Name,
		&d.Space,
		&d.Instances,
		&d.Plan,
		&d.Healthcheck,
	); err != nil {
		return d, err
	}

	return d, nil
}

// DeleteDeploymentV2 - V2 version of space.DeleteAppV2
// (original: "space/app.go")
func DeleteDeploymentV2(db *sql.DB, name string, space string) (int, error) {
	exists, err := checkDeployment(db, name, space)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if exists == false {
		return http.StatusBadRequest, errors.New("Invalid app or space name")
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if err = rt.DeleteService(space, name); err != nil {
		return http.StatusInternalServerError, err
	}

	if err = rt.DeleteDeployment(space, name); err != nil {
		return http.StatusInternalServerError, err
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
		return http.StatusInternalServerError, err
	}

	_, err = db.Exec("DELETE from v2.deployments where space=$1 and name=$2", space, name)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
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

// createOrUpdateService - Create or update service for a given deployment and runtime
func createOrUpdateService(db *sql.DB, rt runtime.Runtime, payload structs.DeploySpecV2) error {
	serviceExists, err := rt.ServiceExists(payload.Space, payload.Name)
	if err != nil {
		return err
	}

	isInternal, err := utils.IsInternalSpace(db, payload.Space)
	if err != nil {
		return err
	}

	appIngress, err := ingress.GetAppIngress(db, isInternal)
	if err != nil {
		return err
	}

	siteIngress, err := ingress.GetSiteIngress(db, isInternal)
	if err != nil {
		return err
	}

	// Any deployment features requiring istio transitioned ingresses should
	// be marked here. Only apply this to the web dyno types.

	appFQDN := payload.Name + "-" + payload.Space
	if payload.Space == "default" {
		appFQDN = payload.Name
	}
	if isInternal {
		appFQDN = appFQDN + "." + os.Getenv("INTERNAL_DOMAIN")
	} else {
		appFQDN = appFQDN + "." + os.Getenv("EXTERNAL_DOMAIN")
	}

	// Create/update service
	if !serviceExists {
		if err := rt.CreateService(payload.Space, payload.Name, payload.Port, payload.Labels, payload.Features); err != nil {
			return err
		}
	} else {
		if err := rt.UpdateService(payload.Space, payload.Name, payload.Port, payload.Labels, payload.Features); err != nil {
			return err
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
				if err := appIngress.InstallOrUpdateJWTAuthFilter(payload.Name, payload.Space, appFQDN, int64(payload.Port), issuer, jwksURI, audiences, excludes, includes); err != nil {
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
			if err := appIngress.InstallOrUpdateCORSAuthFilter(payload.Name+"-"+payload.Space, "/", allowOrigin, allowMethods, allowHeaders, exposeHeaders, maxAge, allowCredentials); err != nil {
				fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
			} else {
				foundCorsFilter = true
			}
			routes, err := ingress.GetPathsByApp(db, payload.Name, payload.Space)
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
				if err := appIngress.InstallOrUpdateCSPFilter(payload.Name+"-"+payload.Space, "/", policy); err != nil {
					fmt.Printf("WARNING: There was an error installing or updating CORS Auth filter: %s\n", err.Error())
				} else {
					foundCspFilter = true
				}
				routes, err := ingress.GetPathsByApp(db, payload.Name, payload.Space)
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
		if err := appIngress.DeleteCORSAuthFilter(payload.Name+"-"+payload.Space, "/"); err != nil {
			fmt.Printf("WARNING: There was an error removing the CORS auth filter from the app: %s\n", err.Error())
		}
		routes, err := ingress.GetPathsByApp(db, payload.Name, payload.Space)
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
		if err := appIngress.DeleteCSPFilter(payload.Name+"-"+payload.Space, "/"); err != nil {
			fmt.Printf("WARNING: There was an error removing the CSP filter from the app: %s\n", err.Error())
		}
		routes, err := ingress.GetPathsByApp(db, payload.Name, payload.Space)
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
		if err := appIngress.DeleteJWTAuthFilter(payload.Name, payload.Space, appFQDN, int64(payload.Port)); err != nil {
			fmt.Printf("WARNING: There was an error removing the JWT auth filter: %s\n", err.Error())
		}
	}

	return nil
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
		return deployresponse, http.StatusBadRequest, errors.New("Deployment Name can not be blank")
	}
	if payload.Space == "" {
		return deployresponse, http.StatusBadRequest, errors.New("Space Name can not be blank")
	}
	if payload.Image == "" {
		return deployresponse, http.StatusBadRequest, errors.New("Image must be specified")
	}
	if !(strings.Contains(payload.Image, ":")) {
		return deployresponse, http.StatusBadRequest, errors.New("Image must contain tag")
	}

	rt, err := runtime.GetRuntimeFor(db, payload.Space)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	name := payload.Name
	space := payload.Space
	image := strings.Split(payload.Image, ":")[0]
	imageTag := strings.Split(payload.Image, ":")[1]

	deploymentQuery := "select instances, port, coalesce(plan, 'noplan') as plan, coalesce(healthcheck, 'tcp') as healthcheck from v2.deployments where name = $1 and space = $2"
	if err = db.QueryRow(deploymentQuery, name, space).Scan(&instances, &nullPort, &plan, &healthcheck); err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	if nullPort.Valid {
		port = int(nullPort.Int64)
	} else {
		port = 0
	}

	// Get bindings
	appconfigset, appbindings, err := config.GetBindings(db, space, name)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	// Get memory limits
	memorylimit, memoryrequest, err := app.GetMemoryLimits(db, plan)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	// Get user defined config vars
	configvars, err := config.GetConfigVars(db, appconfigset)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	// Assemble config -- akkeris "built in config", "user defined config vars", "service configvars"
	elist := app.AddAkkerisConfigVars(name, space)
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}

	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, name, space, appbindings)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}
	for _, e := range servicevars {
		elist = append(elist, e)
	}

	// We have everything we need to create a one-off pod at this point
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
			return deployresponse, http.StatusInternalServerError, err
		}
		deployresponse.Controller = "Deployment Created"
		deployresponse.Service = "Service not required"
		return deployresponse, http.StatusCreated, nil
	}

	isInternal, err := utils.IsInternalSpace(db, space)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
	}

	if payload.Labels == nil {
		payload.Labels = make(map[string]string)
	}

	plantype, err := app.GetPlanType(db, plan)
	if err != nil {
		return deployresponse, http.StatusInternalServerError, err
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
		return deployresponse, http.StatusInternalServerError, err
	}

	// Do not write to cluster above this line, everything below should apply changes,
	// everything above should do sanity checks, this helps prevent "half" deployments
	// by minimizing resource after the first write

	if !deploymentExists {
		if err = rt.CreateDeployment(&deployment); err != nil {
			return deployresponse, http.StatusInternalServerError, err
		}
	} else {
		if err = rt.UpdateDeployment(&deployment); err != nil {
			return deployresponse, http.StatusInternalServerError, err
		}
	}

	// Create/update service for web dyno types
	if finalport != -1 {
		if err = createOrUpdateService(db, rt, payload); err != nil {
			return deployresponse, http.StatusInternalServerError, err
		}
	}

	// Prepare response back
	if finalport != -1 {
		deployresponse.Service = "Service Created"
	} else {
		deployresponse.Service = "Service not required"
	}

	if !deploymentExists {
		deployresponse.Controller = "Deployment Created"
	} else {
		deployresponse.Controller = "Deployment Updated"
	}

	return deployresponse, http.StatusCreated, nil
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
