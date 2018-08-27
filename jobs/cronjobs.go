package jobs

import (
	"database/sql"
	"net/http"
	app "region-api/app"
	config "region-api/config"
	runtime "region-api/runtime"
	service "region-api/service"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/robfig/cron"
)

// GetCronJobs gets a list of current jobs
func GetCronJobs(db *sql.DB, r render.Render) {
	var jobs []string
	stmt, err := db.Prepare("select name,space from cronjobs")
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()

	for rows.Next() {
		var jobName string
		var space string
		err := rows.Scan(&jobName, &space)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		jobs = append(jobs, jobName+":"+space)
	}
	r.JSON(200, jobs)
}

// GetCronJobsSpace gets a list of current jobs in a given space
func GetCronJobsSpace(db *sql.DB, params martini.Params, r render.Render) {
	var jobs []string
	space := params["space"]
	stmt, err := db.Prepare("select name from cronjobs where space=$1")
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer stmt.Close()
	rows, err := stmt.Query(space)
	defer rows.Close()

	for rows.Next() {
		var jobName string
		err := rows.Scan(&jobName)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		jobs = append(jobs, jobName)
	}
	r.JSON(200, jobs)
}

// GetCronJob gets details of a job
func GetCronJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]
	var job structs.JobReq

	stmt, err := db.Prepare("select name,space,cmd,schedule,plan from cronjobs where name=$1 and space=$2")
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer stmt.Close()
	rows, err := stmt.Query(jobName, space)
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&job.Name, &job.Space, &job.CMD, &job.Schedule, &job.Plan)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	}
	r.JSON(200, job)
}

// GetDeployedCronJob returns the running job info
func GetDeployedCronJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	sjob, err := rt.GetCronJob(space, jobName)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, sjob)
}

// GetDeployedCronJobs returns the running job info
func GetDeployedCronJobs(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	sjobs, err := rt.GetCronJobs(space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, sjobs)
}

func CreateCronJob(db *sql.DB, spec structs.JobReq, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Name == "" || spec.Space == "" || spec.Schedule == "" || spec.Plan == "" {
		utils.ReportInvalidRequest("Job's name, space, plan, and schedule are required", r)
		return
	}

	_, err := cron.Parse(spec.Schedule)
	if err != nil {
		utils.ReportInvalidRequest("Invalid Cron: "+err.Error(), r)
		return
	}

	var jobName string
	inserterr := db.QueryRow("INSERT INTO cronjobs(name,space,cmd,schedule,plan) VALUES($1,$2,$3,$4,$5) returning name;",
		spec.Name, spec.Space, spec.CMD, spec.Schedule, spec.Plan).Scan(&jobName)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	message := structs.Messagespec{Status: http.StatusCreated, Message: "Cron Job, " + jobName + " created"}
	r.JSON(http.StatusCreated, message)
}

func UpdateCronJob(db *sql.DB, spec structs.JobReq, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Name == "" || spec.Space == "" || spec.Schedule == "" || spec.Plan == "" {
		utils.ReportInvalidRequest("Job's name, space, plan, and schedule are required", r)
		return
	}
	var jobName string
	var query string

	_, err := cron.Parse(spec.Schedule)
	if err != nil {
		utils.ReportInvalidRequest("Invalid Cron: "+err.Error(), r)
		return
	}

	qerr := db.QueryRow("select name from cronjobs where name=$1 and space=$2", spec.Name, spec.Space).Scan(&query)
	if qerr != nil && strings.Contains(qerr.Error(), "no rows in result set") {
		if inserterr := db.QueryRow("INSERT INTO cronjobs(name,space,cmd,schedule,plan) VALUES($1,$2,$3,$4,$5) returning name;",
			spec.Name, spec.Space, spec.CMD, spec.Schedule, spec.Plan).Scan(&jobName); inserterr != nil {
			utils.ReportError(inserterr, r)
			return
		}
		message := structs.Messagespec{Status: http.StatusCreated, Message: "Cron Job, " + jobName + " created"}
		r.JSON(http.StatusCreated, message)
		return
	} else if qerr != nil {
		utils.ReportError(qerr, r)
		return
	}

	if inserterr := db.QueryRow("UPDATE cronjobs set cmd=$1,schedule=$2,plan=$5 where name=$3 and space=$4 returning name;",
		spec.CMD, spec.Schedule, spec.Name, spec.Space, spec.Plan).Scan(&jobName); err != nil {
		utils.ReportError(inserterr, r)
		return
	}

	message := structs.Messagespec{Status: http.StatusCreated, Message: "Cron Job, " + " updated"}
	r.JSON(http.StatusNoContent, message)
}

func DeleteCronJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	if jobName == "" || space == "" {
		utils.ReportInvalidRequest("Job's name and space are required", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	// Return 404 if no job to delete
	if !rt.CronJobExists(space, jobName) {
		message := structs.Messagespec{Status: http.StatusNotFound, Message: jobName + " does not exist"}
		r.JSON(http.StatusNotFound, message)
	}

	//delete job from config
	stmt, err := db.Prepare("DELETE from cronjobs where name=$1 and space=$2")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	delapp, err := stmt.Exec(jobName, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	_, err = delapp.RowsAffected()
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: jobName + " deleted"})
}

func DeployCronJob(db *sql.DB, params martini.Params, req structs.JobDeploy, berr binding.Errors, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	var (
		cmd      string
		plan     string
		schedule string
	)

	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if req.Image == "" {
		utils.ReportInvalidRequest("Image required", r)
		return
	} else if !(strings.Contains(req.Image, ":")) {
		utils.ReportInvalidRequest("Image must contain tag", r)
		return
	}
	repo := strings.Split(req.Image, ":")[0]
	tag := strings.Split(req.Image, ":")[1]

	err := db.QueryRow("select cmd,schedule,COALESCE(cronjobs.plan,'noplan') AS plan from cronjobs where name=$1 and space=$2",
		jobName, space).Scan(&cmd, &schedule, &plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if rt.CronJobExists(space, jobName) {
		message := structs.Messagespec{Status: http.StatusConflict, Message: "Cron Job already exists"}
		r.JSON(http.StatusConflict, message)
		return
	}

	// Get bindings
	configset, appbindings, err := config.GetBindings(db, space, jobName)

	// Get memory limits
	memoryLimit, memoryRequest, err := app.GetMemoryLimits(db, plan)

	// Get config vars
	configvars, err := config.GetConfigVars(db, configset)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	elist := app.AddAlamoConfigVars(jobName, space)
	for n, v := range configvars {
		elist = append(elist, structs.EnvVar{Name: n, Value: v})
	}
	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, jobName, space, appbindings)
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
	deployment.App = jobName
	deployment.Amount = 1
	deployment.ConfigVars = elist
	deployment.MemoryRequest = memoryRequest
	deployment.MemoryLimit = memoryLimit
	deployment.Image = repo
	deployment.Tag = tag

	response, err := rt.CreateCronJob(&deployment)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusCreated, response)
}

func UpdatedDeployedCronJob(db *sql.DB, params martini.Params, req structs.JobDeploy, berr binding.Errors, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	var (
		cmd      string
		plan     string
		schedule string
	)

	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if req.Image == "" {
		utils.ReportInvalidRequest("Image required", r)
		return
	} else if !(strings.Contains(req.Image, ":")) {
		utils.ReportInvalidRequest("Image must contain tag", r)
		return
	}
	repo := strings.Split(req.Image, ":")[0]
	tag := strings.Split(req.Image, ":")[1]

	//Get stuff from db
	err := db.QueryRow("select cmd,schedule,COALESCE(cronjobs.plan,'noplan') AS plan from cronjobs where name=$1 and space=$2",
		jobName, space).Scan(&cmd, &schedule, &plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if !rt.CronJobExists(space, jobName) {
		message := structs.Messagespec{Status: http.StatusNotFound, Message: "The job specified was not found."}
		r.JSON(http.StatusNotFound, message)
		return
	}

	// Get bindings
	configset, appbindings, err := config.GetBindings(db, space, jobName)

	// Get memory limits
	memoryLimit, memoryRequest, err := app.GetMemoryLimits(db, plan)

	// Get config vars
	configvars, err := config.GetConfigVars(db, configset)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	elist := []structs.EnvVar{}
	for k, v := range configvars {
		var e1 structs.EnvVar
		e1.Name = k
		e1.Value = v
		elist = append(elist, e1)
	}
	// add service vars
	err, servicevars := service.GetServiceConfigVars(db, jobName, space, appbindings)
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
	deployment.App = jobName
	deployment.Amount = 1
	deployment.ConfigVars = elist
	deployment.MemoryRequest = memoryRequest
	deployment.MemoryLimit = memoryLimit
	deployment.Image = repo
	deployment.Tag = tag

	response, err := rt.UpdateCronJob(&deployment)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, response)
}

func StopCronJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Return 404 if no job to delete
	if !rt.CronJobExists(space, jobName) {
		message := structs.Messagespec{Status: http.StatusNotFound, Message: jobName + " does not exist"}
		r.JSON(http.StatusNotFound, message)
	}

	err = rt.DeleteJob(space, jobName)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: jobName + " deleted"})
}
