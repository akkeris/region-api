package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"

	"github.com/lib/pq"

	"github.com/go-martini/martini"
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

// deleteDeploymentV2 - Copy of function in space package. !!Needs to be moved to a common package!!
func deleteDeploymentV2(db *sql.DB, name string, space string) (int, error) {
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

		_, err := deleteDeploymentV2(db, dName, dSpace)
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
func ListAppsV2(db *sql.DB, params martini.Params, r render.Render) {
	rows, err := db.Query("select appid, array_agg(name || '-' || space) deployments from v2.deployments group by appid")
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
