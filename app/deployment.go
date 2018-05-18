package app

import (
	config "../config"
	service "../service"
	structs "../structs"
	utils "../utils"
	runtime "../runtime"
	"database/sql"
	"os"
	"log"
	"strconv"
	"strings"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

func AddAlamoConfigVars(appname string, space string) ([]structs.EnvVar) {
	var elist []structs.EnvVar
	var spaceconfigvar structs.EnvVar
	var appconfigvar structs.EnvVar
	var fullappname structs.EnvVar
	spaceconfigvar.Name = "ALAMO_SPACE"
	spaceconfigvar.Value = space
	elist = append(elist, spaceconfigvar)
	appconfigvar.Name = "ALAMO_DEPLOYMENT"
	appconfigvar.Value = appname
	elist = append(elist, appconfigvar)
	fullappname.Name = "ALAMO_APPLICATION"
	fullappname.Value = appname + "-" + space
	elist = append(elist, fullappname)
	return elist
}

func GetMemoryLimits(db *sql.DB, plan string) (memorylimit string, memoryrequest string, e error) {
	e = db.QueryRow("SELECT memrequest,memlimit from plans where name=$1", plan).Scan(&memoryrequest, &memorylimit)
	if e != nil {
		return "", "", e
	}
	return memorylimit, memoryrequest, nil
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
	
	// Assemble config -- alamo "built in config", "user defined config vars", "service configvars"
	elist := AddAlamoConfigVars(appname, space)
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name:n, Value:v})
	}
	servicevars := service.GetServiceConfigVars(appbindings)
	for _, e := range servicevars {
		elist = append(elist, e)
	}
	

	// Assemble secrets
	var secrets []structs.Namespec
	var secret structs.Namespec
	secret.Name = os.Getenv("KUBERNETES_IMAGE_PULL_SECRET")
	secrets = append(secrets, secret)

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
	deployment.Space = space
	deployment.App = appname
	deployment.Port = finalport
	deployment.Amount = instances
	deployment.ConfigVars = elist
	deployment.Secrets = secrets
	deployment.HealthCheck = healthcheck
	deployment.MemoryRequest = memoryrequest
	deployment.MemoryLimit = memorylimit
	deployment.Image = appimage
	deployment.Tag = apptag
	deployment.RevisionHistoryLimit = revisionhistorylimit
	if len(deploy1.Command) > 0 {
		deployment.Command = deploy1.Command 
	}
	
	newDeployment := !rt.DeploymentExists(space, appname)
	if newDeployment {
		err = rt.CreateDeployment(&deployment)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	} else {
		err = rt.UpdateDeployment(&deployment)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	}

	// Create/update service
	if finalport != -1 {
		if newDeployment {
			_, err := rt.CreateService(space, appname, finalport)
			if err != nil {
				utils.ReportError(err, r)
				return
			}
		} else {
			_, err := rt.UpdateService(space, appname, finalport)
			if err != nil {
				utils.ReportError(err, r)
				return
			}
		}
	}

	// Prepare the response back
	var deployresponse structs.Deployresponse
	if newDeployment {
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
