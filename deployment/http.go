package deployment

import (
	"database/sql"
	"net/http"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"

	"github.com/go-martini/martini"
	"github.com/lib/pq"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	uuid "github.com/nu7hatch/gouuid"
)

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

	updatedDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, updatedDeployment)
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

	updatedDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, updatedDeployment)
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

	updatedDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, updatedDeployment)
}

// DeleteDeploymentV2Handler - HTTP Handler for DeleteDeploymentV2
func DeleteDeploymentV2Handler(db *sql.DB, params martini.Params, r render.Render) {
	name := params["deployment"]
	space := params["space"]

	oldDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	responseCode, err := DeleteDeploymentV2(db, name, space)
	if err != nil {
		if responseCode == http.StatusBadRequest {
			utils.ReportInvalidRequest(err.Error(), r)
		} else {
			utils.ReportError(err, r)
		}
		return
	}

	r.JSON(responseCode, oldDeployment)
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

	scaledDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusAccepted, scaledDeployment)
}

// DeleteSpaceV2 - V2 version of space.Deletespace
// (original: "space/space.go")
// TODO: Move back to space package?
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
// TODO: Move back to space package?
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

		deployment, err := getDeploymentInfo(db, name, space)
		if err != nil {
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

	deployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
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

	r.JSON(http.StatusCreated, configList)
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

	newDeployment, err := getDeploymentInfo(db, name, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusCreated, newDeployment)
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
	var deletedApps []string

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
			} else {
				deletedApps = append(deletedApps, dName+"-"+dSpace)
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

	r.JSON(http.StatusOK, deletedApps)
}

// DescribeAppV2 - Get all deployments for an app based on app ID
// TODO: Move this back to app package?
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
// TODO: Move this back to app package?
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
