package space

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"region-api/app"
	"region-api/config"
	runtime "region-api/runtime"
	"region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
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
func UpdateDeploymentPlanV2(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	appname := params["app"]
	space := params["space"]

	if _, err := db.Exec("UPDATE v2.deployments SET plan=$1 WHERE name=$2 AND space=$3", spaceapp.Plan, appname, space); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + "updated to use " + spaceapp.Plan + " plan"})
}

// UpdateDeploymentHealthCheckV2 - V2 version of space.UpdateAppHealthCheck
// (original: "space/app.go")
func UpdateDeploymentHealthCheckV2(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	if spaceapp.Healthcheck == "" {
		utils.ReportInvalidRequest("healthcheck required", r)
		return
	}

	appname := params["app"]
	space := params["space"]

	_, err := db.Exec("UPDATE v2.deployments SET healthcheck=$1 WHERE name=$2 AND space=$3", spaceapp.Healthcheck, appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + " updated to use " + spaceapp.Healthcheck + " healthcheck"})
}

// DeleteDeploymentHealthCheckV2 - V2 version of space.DeleteAppHealthCheck
// (original: "space/app.go")
func DeleteDeploymentHealthCheckV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["app"]
	space := params["space"]

	if _, err := db.Exec("UPDATE v2.deployments SET healthcheck=NULL WHERE name=$1 AND space=$2", appname, space); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + " healthcheck removed"})
}

// DeleteDeploymentV2 - V2 version of space.DeleteAppV2
// (original: "space/app.go")
func DeleteDeploymentV2(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["app"]
	space := params["space"]

	exists, err := checkDeployment(db, appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if exists == false {
		utils.ReportInvalidRequest("Invalid app or space name", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.DeleteService(space, appname); err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.DeleteDeployment(space, appname); err != nil {
		utils.ReportError(err, r)
		return
	}

	rslist, err := rt.GetReplicas(space, appname)
	// in a previous iteration we ignored the error, i'll do so here
	// but i need to put an if around it to prevent the nil rslist from
	// bombing out everything
	if err != nil {
		fmt.Println(err)
	}
	if err == nil {
		for _, rs := range rslist {
			err = rt.DeleteReplica(space, appname, rs)
			if err != nil {
				log.Println("Failed to remove replica set: ", err)
			}
		}
	} else {
		log.Println("Error getting replica sets:", err)
	}

	var podlist []string
	podlist, err = rt.GetPods(space, appname)
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
	_, err = db.Exec("DELETE from appbindings where space=$1 and appname=$2", space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	_, err = db.Exec("DELETE from v2.deployments where space=$1 and name=$2", space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: appname + " removed"})
}

// ScaleDeploymentV2 - V2 version of space.ScaleApp
// (original: "space/app.go")
func ScaleDeploymentV2(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	appname := params["app"]
	space := params["space"]

	instances := spaceapp.Instances

	if _, err := db.Exec("update v2.deployments set instances=$3 where space=$1 and name=$2", space, appname, instances); err != nil {
		utils.ReportError(err, r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.Scale(space, appname, instances); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusAccepted, structs.Messagespec{Status: http.StatusAccepted, Message: "instances updated"})
}

// AddDeploymentV2 - V2 version of space.AddApp
// (original: "space/app.go")
func AddDeploymentV2(db *sql.DB, params martini.Params, deployment structs.AppDeploymentSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	appname := params["app"]
	space := params["space"]

	var healthcheck *string
	if deployment.Healthcheck.String == "" {
		healthcheck = nil
	} else {
		healthcheck = &deployment.Healthcheck.String
	}

	insertQuery := "insert into v2.deployments(name, space, plan, instances, healthcheck) values($1, $2, $3, $4, $5) returning name"
	inserterr := db.QueryRow(insertQuery, appname, space, deployment.Plan, deployment.Instances, healthcheck).Scan(&appname)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "app added to space"})
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

	rows, err := db.Query("select appid from v2.deployments where space = $1", space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var appid sql.NullString
		if err := rows.Scan(&appid); err != nil {
			utils.ReportError(err, r)
			return
		}

		appDeployments, err := app.GetDeployments(db, appid.String, rt)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, deployment := range appDeployments {
			deployments = append(deployments, deployment)
		}
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
