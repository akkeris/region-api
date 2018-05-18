package jobs

import (
	config "../config"
	service "../service"
	structs "../structs"
	utils "../utils"
	runtime "../runtime"
	app "../app"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

// GetJobs gets a list of current jobs
func GetJobs(db *sql.DB, r render.Render) {
	var jobs []string
	stmt, err := db.Prepare("select name,space from jobs")
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

// GetJobsSpace gets a list of current jobs in a given space
func GetJobsSpace(db *sql.DB, params martini.Params, r render.Render) {
	var jobs []string
	space := params["space"]
	stmt, err := db.Prepare("select name from jobs where space=$1")
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

// GetJob gets details of a job
func GetJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]
	var job structs.JobReq

	stmt, err := db.Prepare("select name,space,cmd,plan from jobs where name=$1 and space=$2")
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	defer stmt.Close()
	rows, err := stmt.Query(jobName, space)
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&job.Name, &job.Space, &job.CMD, &job.Plan)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	}
	r.JSON(200, job)
}

// GetDeployedJob returns the running job info
func GetDeployedJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	rt, err := runtime.GetRuntimeFor(db, space);
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	job, err := rt.GetJob(space, jobName)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, job)
}

// GetDeployedJobs returns the running jobs info
func GetDeployedJobs(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	
	rt, err := runtime.GetRuntimeFor(db, space);
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	jobs, err := rt.GetJobs(space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, jobs)
}

// CreateJob creates a new job in database
func CreateJob(db *sql.DB, spec structs.JobReq, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Name == "" || spec.Space == "" || spec.Plan == "" {
		utils.ReportInvalidRequest("Job name, space, plan Required", r)
		return
	}

	var jobName string
	if inserterr := db.QueryRow("INSERT INTO jobs(name,space,cmd,plan) VALUES($1,$2,$3,$4) returning name;",
		spec.Name, spec.Space, spec.CMD, spec.Plan).Scan(&jobName); inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	message := structs.Messagespec{Status: http.StatusCreated, Message: "Job, " + jobName + " created"}
	r.JSON(http.StatusCreated, message)
}

func UpdateJob(db *sql.DB, spec structs.JobReq, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Name == "" || spec.Space == "" {
		utils.ReportInvalidRequest("Job name and space Required", r)
		return
	}
	var jobName string
	var query string

	qerr := db.QueryRow("select name from jobs where name=$1 and space=$2", spec.Name, spec.Space).Scan(&query)
	if qerr != nil && strings.Contains(qerr.Error(), "no rows in result set") {
		if inserterr := db.QueryRow("INSERT INTO jobs(name,space,cmd,plan) VALUES($1,$2,$3,$4) returning name;",
			spec.Name, spec.Space, spec.CMD, spec.CMD).Scan(&jobName); inserterr != nil {
			utils.ReportError(inserterr, r)
			return
		}
		message := structs.Messagespec{Status: http.StatusCreated, Message: "Job, " + jobName + " created"}
		r.JSON(http.StatusCreated, message)
		return
	} else if qerr != nil {
		utils.ReportError(qerr, r)
		return
	}

	if inserterr := db.QueryRow("UPDATE jobs set cmd=$1,plan=$4 where name=$2 and space=$3 returning name;",
		spec.CMD, spec.Name, spec.Space, spec.Plan).Scan(&jobName); inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}

	message := structs.Messagespec{Status: http.StatusCreated, Message: "Job, " + jobName + " updated"}
	r.JSON(http.StatusNoContent, message)
}

//DeleteJob deletes given job
func DeleteJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

	if jobName == "" || space == "" {
		utils.ReportInvalidRequest("Job's name and space are required", r)
		return
	}

	//delete job from config
	stmt, err := db.Prepare("DELETE from jobs where name=$1 and space=$2")
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
	var message structs.Messagespec
	message.Status = http.StatusOK
	message.Message = jobName + " deleted"
	r.JSON(http.StatusOK, message)
}

// DeployJob sends job to server
func DeployJob(db *sql.DB, params martini.Params, req structs.JobDeploy, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	shouldDelete := req.DeleteBeforeCreate
	jobName := params["jobName"]
	space := params["space"]

	var (
		cmd           string
		plan          string
		secrets       []structs.Namespec
		secret        structs.Namespec
	)

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
	err := db.QueryRow("select cmd,COALESCE(jobs.plan,'noplan') AS plan from jobs where name=$1 and space=$2",
		jobName, space).Scan(&cmd, &plan)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if rt.JobExists(space, jobName) {
		if shouldDelete {
			err := rt.DeleteJob(space, jobName)
			if err != nil {
				utils.ReportError(err, r)
				return
			}
		} else {
			message := structs.Messagespec{Status: http.StatusConflict, Message: "Job already exists, please use deleteBeforeCreate for re-run/overwrite"}
			r.JSON(http.StatusConflict, message)
			return
		}
	}

	// Get bindings
	configset, bindings, err := config.GetBindings(db, space, jobName)

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
		elist = append(elist, structs.EnvVar{Name:n, Value:v})
	}
	servicevars := service.GetServiceConfigVars(bindings)
	for _, e := range servicevars {
		elist = append(elist, e)
	}

	// Image Secret
	secret.Name = os.Getenv("KUBERNETES_IMAGE_PULL_SECRET")
	secrets = append(secrets, secret)

	// Create deployment
	var deployment structs.Deployment
	deployment.Space = space
	deployment.App = jobName
	deployment.Amount = 1
	deployment.ConfigVars = elist
	deployment.Secrets = secrets
	deployment.MemoryRequest = memoryRequest
	deployment.MemoryLimit = memoryLimit
	deployment.Image = repo
	deployment.Tag = tag

	response, err := rt.CreateJob(&deployment)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusCreated, response)
}

// StopJob deletes running/or deployed job
func StopJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]

    rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	// Return 404 if no job to delete
	if rt.JobExists(space, jobName) {
		err = rt.DeleteJob(space, jobName)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		message := structs.Messagespec{Status: http.StatusOK, Message: jobName + " deleted"}
		r.JSON(http.StatusOK, message)
	} else {
		message := structs.Messagespec{Status: http.StatusNotFound, Message: jobName + " does not exist"}
		r.JSON(http.StatusNotFound, message)
	}
}

// CleanJobs cleans all the jobs with a label (typically job name) for older jobs
func CleanJobs(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	jobName := params["jobName"]
	
	rt, err := runtime.GetRuntimeFor(db, space);
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	err = rt.DeleteJob(space, jobName)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	// Cleanup pods from those old jobs
	err = rt.DeletePods(space, jobName)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	message := structs.Messagespec{Status: http.StatusOK, Message: "jobs related to " + jobName + " deleted"}
	r.JSON(http.StatusOK, message)
}

func ScaleJob(db *sql.DB, params martini.Params, r render.Render) {
	jobName := params["jobName"]
	space := params["space"]
	replicas := params["replicas"]
    timeout := params["timeout"]

    rt, err := runtime.GetRuntimeFor(db, space);
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Return 404 if no job to delete
	if rt.JobExists(space, jobName) {
		replicasi, err := strconv.Atoi(replicas)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
        timeouti, err := strconv.Atoi(timeout)
        if err != nil {
                utils.ReportError(err, r)
                return
        }
		err = rt.ScaleJob(space, jobName, replicasi, timeouti)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		message := structs.Messagespec{Status: http.StatusOK, Message: jobName + " scaled"}
		r.JSON(http.StatusOK, message)
	} else {
		message := structs.Messagespec{Status: http.StatusNotFound, Message: jobName + " does not exist"}
		r.JSON(http.StatusNotFound, message)
	}
}

