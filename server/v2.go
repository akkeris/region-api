package server

import (
	"region-api/app"
	"region-api/config"
	"region-api/jobs"
	"region-api/maintenance"
	"region-api/service"
	"region-api/space"
	"region-api/structs"
	"region-api/templates"
	"region-api/utils"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
)

func initV2Endpoints(m *martini.ClassicMartini) {

	/****************************
	*
	*	 Modified from V1 Schema
	*
	****************************/

	// "from apps,spacesapps"
	m.Delete("/v2beta1/app/:appname", app.DeleteAppV2)
	m.Post("/v2beta1/app/deploy", binding.Json(structs.Deployspec{}), app.DeploymentV2)
	m.Get("/v2beta1/space/:space/app/:appname/configvars", app.GetAllConfigVarsV2)
	m.Post("/v2beta1/app/deploy/oneoff", binding.Json(structs.OneOffSpec{}), app.OneOffDeploymentV2)
	m.Get("/v2beta1/app/:appname", app.DescribeAppV2)

	// "from apps"
	m.Get("/v2beta1/apps", app.ListAppsV2)

	// "from spacesapps"
	m.Get("/v2beta1/space/:space/app/:appname", app.DescribeAppInSpaceV2)
	m.Get("/v2beta1/space/:space/apps", app.DescribeSpaceV2)
	m.Put("/v2beta1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), space.UpdateAppPlanV2)
	m.Put("/v2beta1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), space.UpdateAppHealthCheckV2)
	m.Delete("/v2beta1/space/:space/app/:app/healthcheck", space.DeleteAppHealthCheckV2)
	m.Delete("/v2beta1/space/:space/app/:app", space.DeleteAppV2)
	m.Put("/v2beta1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), space.ScaleAppV2)
	m.Put("/v2beta1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), space.AddAppV2)
	m.Delete("/v2beta1/space/:space", binding.Json(structs.Spacespec{}), space.DeleteSpaceV2)

	/****************************
	*
	*	 Unchanged from V1 Schema
	*
	****************************/

	m.Get("/v2beta1/config/sets", config.Listsets)
	m.Get("/v2beta1/config/set/:setname", config.Dumpset)
	m.Delete("/v2beta1/config/set/:setname", config.Deleteset)
	m.Post("/v2beta1/config/set/:parent/include/:child", config.Includeset)
	m.Delete("/v2beta1/config/set/:parent/include/:child", config.Deleteinclude)
	m.Post("/v2beta1/config/set", binding.Json(structs.Setspec{}), config.Createset)
	m.Post("/v2beta1/config/set/configvar", binding.Json([]structs.Varspec{}), config.Addvars)
	m.Patch("/v2beta1/config/set/configvar", binding.Json(structs.Varspec{}), config.Updatevar)
	m.Get("/v2beta1/config/set/:setname/configvar/:varname", config.Getvar)
	m.Delete("/v2beta1/config/set/:setname/configvar/:varname", config.Deletevar)

	m.Post("/v2beta1/app", binding.Json(structs.Appspec{}), app.Createapp)
	m.Patch("/v2beta1/app", binding.Json(structs.Appspec{}), app.Updateapp)
	m.Post("/v2beta1/app/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v2beta1/app/:appname/bind/:bindspec", app.Unbindapp)
	m.Get("/v2beta1/space/:space/app/:app/instance", app.GetInstances)
	m.Get("/v2beta1/space/:space/app/:appname/instance/:instanceid/log", app.GetAppLogs)
	m.Delete("/v2beta1/space/:space/app/:app/instance/:instanceid", app.DeleteInstance)
	m.Post("/v2beta1/space/:space/app/:app/instance/:instance/exec", binding.Json(structs.Exec{}), app.Exec)
	m.Get("/v2beta1/apps/plans", app.GetPlans)
	m.Post("/v2beta1/space/:space/app/:app/rollback/:revision", app.Rollback)
	m.Post("/v2beta1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v2beta1/space/:space/app/:appname/bind/**", app.Unbindapp)
	m.Post("/v2beta1/space/:space/app/:appname/bindmap/:bindtype/:bindname", binding.Json(structs.Bindmapspec{}), app.Createbindmap)
	m.Get("/v2beta1/space/:space/app/:appname/bindmap/:bindtype/:bindname", app.Getbindmaps)
	m.Delete("/v2beta1/space/:space/app/:appname/bindmap/:bindtype/:bindname/:mapid", app.Deletebindmap)
	m.Get("/v2beta1/space/:space/app/:appname/configvars/:bindtype/:bindname", app.GetServiceConfigVars)
	m.Post("/v2beta1/space/:space/app/:appname/restart", app.Restart)
	m.Get("/v2beta1/space/:space/app/:app/status", app.Spaceappstatus)
	m.Get("/v2beta1/kube/podstatus/:space/:app", app.PodStatus)

	m.Get("/v2beta1/spaces", space.Listspaces)
	m.Post("/v2beta1/space", binding.Json(structs.Spacespec{}), space.Createspace)
	m.Get("/v2beta1/space/:space", space.Space)
	m.Put("/v2beta1/space/:space/tags", binding.Json(structs.Spacespec{}), space.UpdateSpaceTags)

	m.Get("/v2beta1/octhc/kube", utils.Octhc)
	m.Get("/v2beta1/octhc/service/rabbitmq", service.Getrabbitmqplans)
	m.Get("/v2beta1/octhc/kubesystem", utils.GetKubeSystemPods)

	m.Get("/v2beta1/jobs", jobs.GetJobs)
	m.Post("/v2beta1/jobs", binding.Json(structs.JobReq{}), jobs.CreateJob)
	m.Put("/v2beta1/jobs", binding.Json(structs.JobReq{}), jobs.UpdateJob)
	m.Get("/v2beta1/space/:space/jobs", jobs.GetJobsSpace)
	m.Get("/v2beta1/space/:space/jobs/run", jobs.GetDeployedJobs)
	m.Get("/v2beta1/space/:space/jobs/:jobName", jobs.GetJob)
	m.Delete("/v2beta1/space/:space/jobs/:jobName", jobs.DeleteJob)
	m.Get("/v2beta1/space/:space/jobs/:jobName/run", jobs.GetDeployedJob)
	m.Post("/v2beta1/space/:space/jobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.DeployJob)
	m.Post("/v2beta1/space/:space/jobs/:jobName/scale/:replicas/:timeout", jobs.ScaleJob)
	m.Delete("/v2beta1/space/:space/jobs/:jobName/run", jobs.StopJob)
	m.Delete("/v2beta1/space/:space/jobs/:jobName/clean", jobs.CleanJobs)

	m.Get("/v2beta1/cronjobs", jobs.GetCronJobs)
	m.Post("/v2beta1/cronjobs", binding.Json(structs.JobReq{}), jobs.CreateCronJob)
	m.Put("/v2beta1/cronjobs", binding.Json(structs.JobReq{}), jobs.UpdateCronJob)
	m.Get("/v2beta1/space/:space/cronjobs", jobs.GetCronJobsSpace)
	m.Get("/v2beta1/space/:space/cronjobs/run", jobs.GetDeployedCronJobs)
	m.Get("/v2beta1/space/:space/cronjobs/:jobName", jobs.GetCronJob)
	m.Delete("/v2beta1/space/:space/cronjobs/:jobName", jobs.DeleteCronJob)
	m.Get("/v2beta1/space/:space/cronjobs/:jobName/run", jobs.GetDeployedCronJob)
	m.Post("/v2beta1/space/:space/cronjobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.DeployCronJob)
	m.Put("/v2beta1/space/:space/cronjobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.UpdatedDeployedCronJob)
	m.Delete("/v2beta1/space/:space/cronjobs/:jobName/run", jobs.StopCronJob)

	m.Post("/v2beta1/space/:space/app/:app/maintenance", maintenance.EnableMaintenancePage)
	m.Delete("/v2beta1/space/:space/app/:app/maintenance", maintenance.DisableMaintenancePage)
	m.Get("/v2beta1/space/:space/app/:app/maintenance", maintenance.MaintenancePageStatus)

	m.Get("/v2beta1/utils/service/space/:space/app/:app", utils.GetService)
	m.Get("/v2beta1/utils/nodes", utils.GetNodes)
	m.Get("/v2beta1/utils/urltemplates", templates.GetURLTemplates)
}
