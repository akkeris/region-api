package space

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"log"
	"net/http"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"
)

func AddApp(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	appname := params["app"]
	space := params["space"]

	var healthcheck *string
	if spaceapp.Healthcheck == "" {
		healthcheck = nil
	} else {
		healthcheck = &spaceapp.Healthcheck
	}

	inserterr := db.QueryRow("INSERT INTO spacesapps(space,appname,instances,plan,healthcheck) VALUES($1,$2,$3,$4,$5) returning appname;", space, appname, spaceapp.Instances, spaceapp.Plan, healthcheck).Scan(&appname)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "app added to space"})
}

func UpdateAppPlan(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	appname := params["app"]
	space := params["space"]

	_, err := db.Exec("UPDATE spacesapps SET plan=$1 WHERE appname=$2 AND space=$3", spaceapp.Plan, appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + "updated to use " + spaceapp.Plan + " plan"})
}

func UpdateAppHealthCheck(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
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

	_, err := db.Exec("UPDATE spacesapps SET healthcheck=$1 WHERE appname=$2 AND space=$3", spaceapp.Healthcheck, appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + " updated to use " + spaceapp.Healthcheck + " healthcheck"})
}

func DeleteAppHealthCheck(db *sql.DB, params martini.Params, r render.Render) {

	appname := params["app"]
	space := params["space"]

	_, err := db.Exec("UPDATE spacesapps SET healthcheck=NULL WHERE appname=$1 AND space=$2", appname, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "App: " + appname + " healthcheck removed"})
}

func DeleteApp(db *sql.DB, params martini.Params, r render.Render) {
	appname := params["app"]
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	err = rt.DeleteService(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	err = rt.DeleteDeployment(space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if err != nil {
		log.Println(err)
	}
	rslist, err := rt.GetReplicas(space, appname)
	// in a previous iteration we ignored the error, i'll do so here
	// but i need to put an if around it to prevent the nil rslist from
	// bombing out everything
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

	_, err = db.Exec("DELETE from spacesapps where space=$1 and appname=$2", space, appname)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: appname + " removed"})
}

func ScaleApp(db *sql.DB, params martini.Params, spaceapp structs.Spaceappspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	appname := params["app"]
	space := params["space"]

	instances := spaceapp.Instances

	_, err := db.Exec("update spacesapps set instances=$3 where space=$1 and appname=$2", space, appname, instances)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = rt.Scale(space, appname, instances)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusAccepted, structs.Messagespec{Status: http.StatusAccepted, Message: "instances updated"})
}
