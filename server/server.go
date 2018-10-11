package server

import (
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"region-api/app"
	"region-api/certs"
	"region-api/config"
	"region-api/features"
	"region-api/jobs"
	"region-api/maintenance"
	"region-api/monitor"
	"region-api/router"
	"region-api/service"
	"region-api/space"
	"region-api/structs"
	"region-api/templates"
	"region-api/utils"
	"region-api/vault"
	"time"
	"strings"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/auth"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

func ProxyToShuttle(res http.ResponseWriter, req *http.Request) {
	uri, err := url.Parse("http://" + os.Getenv("LOGSHUTTLE_SERVICE_HOST") + ":" + os.Getenv("LOGSHUTTLE_SERVICE_PORT"))
	if err != nil {
		log.Println("Error: Unable to proxy to log shuttle")
		log.Println(err)
		return
	}
	log.Println("Proxying to", uri)
	httputil.NewSingleHostReverseProxy(uri).ServeHTTP(res, req)
}

func ProxyToSession(res http.ResponseWriter, req *http.Request) {
	uri, err := url.Parse("http://" + os.Getenv("LOGSESSION_SERVICE_HOST") + ":" + os.Getenv("LOGSESSION_SERVICE_PORT"))
	if err != nil {
		log.Println("Error: Unable to proxy to log session")
		log.Println(err)
		return
	}
	log.Println("Proxying to", uri)
	rp := httputil.NewSingleHostReverseProxy(uri)
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func ProxyToInfluxDb(res http.ResponseWriter, req *http.Request) {
	uri, err := url.Parse(os.Getenv("INFLUXDB_URL"))
	if err != nil {
		log.Println("Error: Unable to proxy to influxdb")
		log.Println(err)
		return
	}
	log.Println("Proxying influxdb to", uri)
	rp := httputil.NewSingleHostReverseProxy(uri)
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func ProxyToPrometheus(res http.ResponseWriter, req *http.Request) {
	uri, err := url.Parse(os.Getenv("PROMETHEUS_URL"))
	if err != nil {
		log.Println("Error: Unable to proxy to influxdb")
		log.Println(err)
		return
	}
	log.Println("Proxying prometheus metrics to", uri)
	rp := httputil.NewSingleHostReverseProxy(uri)
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func InitOldServiceEndpoints(m *martini.ClassicMartini) {
	// While moving over to the open service broker api spec
	// these old end points formats should no longer be used.
	m.Get("/v1/service/redis/plans", service.Getredisplans)
	m.Get("/v1/service/redis/url/:servicename", service.Getredisurl)
	m.Post("/v1/service/redis/instance", binding.Json(structs.Provisionspec{}), service.Provisionredis)
	m.Delete("/v1/service/redis/instance/:servicename", service.Deleteredis)
	m.Post("/v1/service/redis/instance/tag", binding.Json(structs.Tagspec{}), service.Tagredis)

	m.Get("/v1/service/es/plans", service.Getesplans)
	m.Get("/v1/service/es/url/:servicename", service.Getesurl)
	m.Get("/v1/service/es/instance/:servicename/status", service.Getesstatus)
	m.Post("/v1/service/es/instance", binding.Json(structs.Provisionspec{}), service.Provisiones)
	m.Delete("/v1/service/es/instance/:servicename", service.Deletees)
	m.Post("/v1/service/es/instance/tag", binding.Json(structs.Tagspec{}), service.Tages)

	m.Get("/v1/service/memcached/plans", service.Getmemcachedplans)
	m.Get("/v1/service/memcached/url/:servicename", service.Getmemcachedurl)
	m.Post("/v1/service/memcached/instance", binding.Json(structs.Provisionspec{}), service.Provisionmemcached)
	m.Delete("/v1/service/memcached/instance/:servicename", service.Deletememcached)
	m.Post("/v1/service/memcached/instance/tag", binding.Json(structs.Tagspec{}), service.Tagmemcached)
	m.Get("/v1/service/memcached/operations/stats/:name", service.GetMemcachedStats)
	m.Delete("/v1/service/memcached/operations/cache/:name", service.FlushMemcached)

	m.Get("/v1/service/influxdb/plans", service.GetInfluxdbPlans)
	m.Get("/v1/service/influxdb/url/:servicename", service.GetInfluxdbURL)
	m.Post("/v1/service/influxdb/instance", binding.Json(structs.Provisionspec{}), service.ProvisionInfluxdb)
	m.Delete("/v1/service/influxdb/instance/:servicename", service.DeleteInfluxdb)


	m.Get("/v1/service/cassandra/plans", service.GetCassandraPlans)
	m.Get("/v1/service/cassandra/url/:servicename", service.GetCassandraURL)
	m.Post("/v1/service/cassandra/instance", binding.Json(structs.Provisionspec{}), service.ProvisionCassandra)
	m.Delete("/v1/service/cassandra/instance/:servicename", service.DeleteCassandra)


	m.Get("/v1/service/neptune/plans", service.GetNeptunePlans)
	m.Get("/v1/service/neptune/url/:servicename", service.GetNeptuneURL)
	m.Post("/v1/service/neptune/instance", binding.Json(structs.Provisionspec{}), service.ProvisionNeptune)
	m.Delete("/v1/service/neptune/instance/:servicename", service.DeleteNeptune)
	m.Post("/v1/service/neptune/instance/tag", binding.Json(structs.Tagspec{}), service.TagNeptune)

	m.Get("/v1/service/rabbitmq/plans", service.Getrabbitmqplans)
	m.Post("/v1/service/rabbitmq/instance", binding.Json(structs.Provisionspec{}), service.Provisionrabbitmq)
	m.Get("/v1/service/rabbitmq/url/:servicename", service.Getrabbitmqurl)
	m.Delete("/v1/service/rabbitmq/instance/:servicename", service.Deleterabbitmq)
	m.Post("/v1/service/rabbitmq/instance/tag", binding.Json(structs.Tagspec{}), service.Tagrabbitmq)

	m.Get("/v1/service/s3/plans", service.Gets3plans)
	m.Post("/v1/service/s3/instance", binding.Json(structs.Provisionspec{}), service.Provisions3)
	m.Get("/v1/service/s3/url/:servicename", service.Gets3url)
	m.Delete("/v1/service/s3/instance/:servicename", service.Deletes3)
	m.Post("/v1/service/s3/instance/tag", binding.Json(structs.Tagspec{}), service.Tags3)

	m.Get("/v1/service/postgres/plans", service.GetpostgresplansV1)                                             // deprecated
	m.Get("/v1/service/postgres/url/:servicename", service.GetpostgresurlV1)                                    // deprecated
	m.Post("/v1/service/postgres/instance", binding.Json(structs.Provisionspec{}), service.ProvisionpostgresV1) // deprecated
	m.Delete("/v1/service/postgres/instance/:servicename", service.DeletepostgresV1)                            // deprecated
	m.Post("/v1/service/postgres/instance/tag", binding.Json(structs.Tagspec{}), service.TagpostgresV1)         // deprecated

	m.Get("/v1/service/postgresonprem/plans", service.GetpostgresonpremplansV1)
	m.Get("/v1/service/postgresonprem/url/:servicename", service.GetpostgresonpremurlV1)
	m.Post("/v1/service/postgresonprem/instance", binding.Json(structs.Provisionspec{}), service.ProvisionpostgresonpremV1)
	m.Delete("/v1/service/postgresonprem/instance/:servicename", service.DeletepostgresonpremV1)
	m.Post("/v1/service/postgresonprem/:servicename/roles", service.CreatePostgresonpremRoleV1)
	m.Delete("/v1/service/postgresonprem/:servicename/roles/:role", service.DeletePostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:servicename/roles", service.ListPostgresonpremRolesV1)
	m.Put("/v1/service/postgresonprem/:servicename/roles/:role", service.RotatePostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:servicename/roles/:role", service.GetPostgresonpremRoleV1)
	m.Get("/v1/service/postgresonprem/:servicename", service.GetPostgresonpremV1)

	m.Get("/v1/service/postgresonprem/:servicename/backups", service.ListPostgresonpremBackupsV1)
	m.Get("/v1/service/postgresonprem/:servicename/backups/:backup", service.GetPostgresonpremBackupV1)
	m.Put("/v1/service/postgresonprem/:servicename/backups", service.CreatePostgresonpremBackupV1)
	m.Put("/v1/service/postgresonprem/:servicename/backups/:backup", service.RestorePostgresonpremBackupV1)
	m.Get("/v1/service/postgresonprem/:servicename/logs", service.ListPostgresonpremLogsV1)
	m.Get("/v1/service/postgresonprem/:servicename/logs/:dir/:file", service.GetPostgresonpremLogsV1)
	m.Put("/v1/service/postgresonprem/:servicename", service.RestartPostgresonpremV1)

	m.Get("/v2/services/postgres/plans", service.GetPostgresPlansV2)
	m.Get("/v2/services/postgres/:servicename/url", service.GetPostgresUrlV2)
	m.Post("/v2/services/postgres", binding.Json(structs.Provisionspec{}), service.ProvisionPostgresV2)
	m.Delete("/v2/services/postgres/:servicename", service.DeletePostgresV2)
	m.Post("/v2/services/postgres/:servicename/tags", binding.Json(structs.Tagspec{}), service.TagPostgresV2)
	m.Get("/v2/services/postgres/:servicename/backups", service.ListPostgresBackupsV2)
	m.Get("/v2/services/postgres/:servicename/backups/:backup", service.GetPostgresBackupV2)
	m.Put("/v2/services/postgres/:servicename/backups", service.CreatePostgresBackupV2)
	m.Put("/v2/services/postgres/:servicename/backups/:backup", service.RestorePostgresBackupV2)
	m.Get("/v2/services/postgres/:servicename/logs", service.ListPostgresLogsV2)
	m.Get("/v2/services/postgres/:servicename/logs/:dir/:file", service.GetPostgresLogsV2)
	m.Put("/v2/services/postgres/:servicename", service.RestartPostgresV2)
	m.Post("/v2/services/postgres/:servicename/roles", service.CreatePostgresRoleV2)
	m.Delete("/v2/services/postgres/:servicename/roles/:role", service.DeletePostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename/roles", service.ListPostgresRolesV2)
	m.Put("/v2/services/postgres/:servicename/roles/:role", service.RotatePostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename/roles/:role", service.GetPostgresRoleV2)
	m.Get("/v2/services/postgres/:servicename", service.GetPostgresV2)

	m.Get("/v1/service/mongodb/plans", service.GetmongodbplansV1)
	m.Post("/v1/service/mongodb/instance", binding.Json(structs.Provisionspec{}), service.ProvisionmongodbV1)
	m.Get("/v1/service/mongodb/url/:servicename", service.GetmongodburlV1)
	m.Delete("/v1/service/mongodb/instance/:servicename", service.DeletemongodbV1)
	m.Get("/v1/service/mongodb/:servicename", service.GetmongodbV1)
	m.Get("/v1/service/mongodb/instance/:servicename", service.GetmongodbV1)

	m.Get("/v1/service/aurora-mysql/plans", service.Getauroramysqlplans)
	m.Get("/v1/service/aurora-mysql/url/:servicename", service.Getauroramysqlurl)
	m.Post("/v1/service/aurora-mysql/instance", binding.Json(structs.Provisionspec{}), service.Provisionauroramysql)
	m.Delete("/v1/service/aurora-mysql/instance/:servicename", service.Deleteauroramysql)
	m.Post("/v1/service/aurora-mysql/instance/tag", binding.Json(structs.Tagspec{}), service.Tagauroramysql)

	m.Get("/v1/service/:service/bindings", service.GetBindingList)
}

var catalogOSBProvider *service.OSBClientServices
func InitOpenServiceBrokerEndpoints(db *sql.DB, m *martini.ClassicMartini) {
	var err error
	catalogOSBProvider, err = service.NewOSBClientServices(strings.Split(os.Getenv("SERVICES"),","), db)
	if err != nil {
		log.Fatalln(err)
	}
	service.SetGlobalClientService(catalogOSBProvider)
	m.Get("/v2/catalog", catalogOSBProvider.HttpGetCatalog)
	m.Get("/v2/service_instances/:instance_id/last_operation", catalogOSBProvider.HttpGetLastOperation)
	m.Put("/v2/service_instances/:instance_id", binding.Json(service.ProvisionRequestBody{}), catalogOSBProvider.HttpGetCreateOrUpdateInstance)
	m.Delete("/v2/service_instances/:instance_id", catalogOSBProvider.HttpDeleteInstance)
	// m.Patch("/v2/service_instances/{instance_id}", catalogOSBProvider.HttpPartialUpdateInstance)
	m.Put("/v2/service_instances/:instance_id/service_bindings/:binding_id",  binding.Json(service.BindRequestBody{}), catalogOSBProvider.HttpCreateOrUpdateBinding)
	m.Get("/v2/service_instances/:instance_id/service_bindings/:binding_id", catalogOSBProvider.HttpGetBinding)
	m.Get("/v2/service_instances/:instance_id/service_bindings/:binding_id/last_operation", catalogOSBProvider.HttpGetBindingLastOperation)
	m.Delete("/v2/service_instances/:instance_id/service_bindings/:binding_id", catalogOSBProvider.HttpRemoveBinding)
	// Custom Actions
	m.Delete("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Get("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Patch("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Put("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Post("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
}

func Server(db *sql.DB) *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Post("/v1/app/deploy", binding.Json(structs.Deployspec{}), app.Deployment)
	m.Post("/v1/app/deploy/oneoff", binding.Json(structs.OneOffSpec{}), app.OneOffDeployment)

	m.Get("/v1/config/sets", config.Listsets)
	m.Get("/v1/config/set/:setname", config.Dumpset)
	m.Delete("/v1/config/set/:setname", config.Deleteset)
	m.Post("/v1/config/set/:parent/include/:child", config.Includeset)
	m.Delete("/v1/config/set/:parent/include/:child", config.Deleteinclude)
	m.Post("/v1/config/set", binding.Json(structs.Setspec{}), config.Createset)
	m.Post("/v1/config/set/configvar", binding.Json([]structs.Varspec{}), config.Addvars)
	m.Patch("/v1/config/set/configvar", binding.Json(structs.Varspec{}), config.Updatevar)
	m.Get("/v1/config/set/:setname/configvar/:varname", config.Getvar)
	m.Delete("/v1/config/set/:setname/configvar/:varname", config.Deletevar)

	m.Post("/v1/app", binding.Json(structs.Appspec{}), app.Createapp)
	m.Patch("/v1/app", binding.Json(structs.Appspec{}), app.Updateapp)
	m.Delete("/v1/app/:appname", app.Deleteapp)

	m.Post("/v1/app/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v1/app/:appname/bind/:bindspec", app.Unbindapp)

	m.Get("/v1/app/:appname", app.Describeapp)
	m.Get("/v1/space/:space/app/:app/instance", app.GetInstances)
	m.Get("/v1/space/:space/app/:appname/instance/:instanceid/log", app.GetAppLogs)
	m.Delete("/v1/space/:space/app/:app/instance/:instanceid", app.DeleteInstance)
	m.Get("/v1/apps", app.Listapps)
	m.Get("/v1/apps/plans", app.GetPlans)

	m.Post("/v1/space", binding.Json(structs.Spacespec{}), space.Createspace)
	m.Delete("/v1/space/:space", binding.Json(structs.Spacespec{}), space.Deletespace)
	m.Get("/v1/space/:space", space.Space)
	m.Put("/v1/space/:space/tags", binding.Json(structs.Spacespec{}), space.UpdateSpaceTags)
	m.Put("/v1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), space.AddApp)
	m.Put("/v1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), space.UpdateAppHealthCheck)
	m.Delete("/v1/space/:space/app/:app/healthcheck", space.DeleteAppHealthCheck)
	m.Put("/v1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), space.UpdateAppPlan)
	m.Put("/v1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), space.ScaleApp)
	m.Post("/v1/space/:space/app/:app/rollback/:revision", app.Rollback)
	m.Delete("/v1/space/:space/app/:app", space.DeleteApp)

	m.Post("/v1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v1/space/:space/app/:appname/bind/**", app.Unbindapp)

	m.Post("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", binding.Json(structs.Bindmapspec{}), app.Createbindmap)
	m.Get("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", app.Getbindmaps)
	m.Delete("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname/:mapid", app.Deletebindmap)

	m.Get("/v1/spaces", space.Listspaces)
	m.Get("/v1/space/:space/apps", app.Describespace)
	m.Get("/v1/space/:space/app/:appname", app.DescribeappInSpace)

	m.Get("/v1/space/:space/app/:appname/deployments", app.GetDeployments)
	m.Get("/v1/space/:space/app/:appname/configvars", app.GetAllConfigVars)
	m.Get("/v1/space/:space/app/:appname/configvars/:bindtype/:bindname", app.GetServiceConfigVars)
	m.Post("/v1/space/:space/app/:appname/restart", app.Restart)
	m.Get("/v1/space/:space/app/:app/status", app.Spaceappstatus)
	m.Get("/v1/kube/podstatus/:space/:app", app.PodStatus)
	m.Get("/v1/services", utils.GetServices)
	m.Get("/v1/deployments", utils.GetDeployments)

	m.Get("/v1/space/:space/app/:app/subscribers", app.GetSubscribersDB)
	m.Delete("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), app.RemoveSubscriberDB)
	m.Post("/v1/space/:space/app/:app/subscriber", binding.Json(structs.Subscriberspec{}), app.AddSubscriberDB)

	m.Put("/v1/feature/opsgenie/space/:space/app/:app/:optionvalue", features.UpdateOpsgenieOption)
	m.Get("/v1/feature/opsgenie/space/:space/app/:app", features.GetOpsgenieOption)

	m.Put("/v1/feature/octhc/space/:space/app/:app/:optionvalue", features.UpdateOcthcOption)
	m.Get("/v1/feature/octhc/space/:space/app/:app", features.GetOcthcOption)

	m.Post("/v1/monitor/callback", binding.Json(structs.NagiosAlert{}), monitor.Callback)
	m.Get("/v1/space/:space/app/:app/callback", monitor.GetCallbacks)
	m.Post("/v1/space/:space/app/:app/callback", binding.Json(structs.Callbackspec{}), monitor.CreateCallback)
	m.Delete("/v1/space/:space/app/:app/callback/tag/:tag/method/:method", monitor.DeleteCallback)

	m.Get("/v1/service/vault/plans", vault.GetVaultList)
	m.Get("/v1/service/vault/credentials/**", vault.GetVaultVariablesMasked)

	m.Get("/v1/routers", router.DescribeRouters)
	m.Get("/v1/router/:router", router.DescribeRouter)
	m.Post("/v1/router", binding.Json(structs.Routerspec{}), router.CreateRouter)
	m.Post("/v1/router/:router/path", binding.Json(structs.Routerpathspec{}), router.AddPath)
	m.Delete("/v1/router/:router/path", binding.Json(structs.Routerpathspec{}), router.DeletePath)

	m.Put("/v1/router/:router/path", binding.Json(structs.Routerpathspec{}), router.UpdatePath)
	m.Put("/v1/router/:router", router.PushRouter)
	m.Delete("/v1/router/:router", router.DeleteRouter)

	m.Get("/v1/octhc/router", router.Octhc)
	m.Get("/v1/octhc/kube", utils.Octhc)
	m.Get("/v1/octhc/service/postgres", service.GetPostgresPlansV2)
	m.Get("/v1/octhc/service/aurora-mysql", service.Getauroramysqlplans)
	m.Get("/v1/octhc/service/redis", service.Getredisplans)
	m.Get("/v1/octhc/service/memcached", service.Getmemcachedplans)
	m.Get("/v1/octhc/service/s3", service.Gets3plans)
	m.Get("/v1/octhc/service/vault", vault.GetVaultList)
	m.Get("/v1/octhc/service/rabbitmq", service.Getrabbitmqplans)
	m.Get("/v1/octhc/kubesystem", utils.GetKubeSystemPods)

	m.Get("/v1beta1/jobs", jobs.GetJobs)
	m.Post("/v1beta1/jobs", binding.Json(structs.JobReq{}), jobs.CreateJob)
	m.Put("/v1beta1/jobs", binding.Json(structs.JobReq{}), jobs.UpdateJob)
	m.Get("/v1beta1/space/:space/jobs", jobs.GetJobsSpace)
	m.Get("/v1beta1/space/:space/jobs/run", jobs.GetDeployedJobs)
	m.Get("/v1beta1/space/:space/jobs/:jobName", jobs.GetJob)
	m.Delete("/v1beta1/space/:space/jobs/:jobName", jobs.DeleteJob)
	m.Get("/v1beta1/space/:space/jobs/:jobName/run", jobs.GetDeployedJob)
	m.Post("/v1beta1/space/:space/jobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.DeployJob)
	m.Post("/v1beta1/space/:space/jobs/:jobName/scale/:replicas/:timeout", jobs.ScaleJob)
	m.Delete("/v1beta1/space/:space/jobs/:jobName/run", jobs.StopJob)
	m.Delete("/v1beta1/space/:space/jobs/:jobName/clean", jobs.CleanJobs)

	m.Get("/v1beta1/cronjobs", jobs.GetCronJobs)
	m.Post("/v1beta1/cronjobs", binding.Json(structs.JobReq{}), jobs.CreateCronJob)
	m.Put("/v1beta1/cronjobs", binding.Json(structs.JobReq{}), jobs.UpdateCronJob)
	m.Get("/v1beta1/space/:space/cronjobs", jobs.GetCronJobsSpace)
	m.Get("/v1beta1/space/:space/cronjobs/run", jobs.GetDeployedCronJobs)
	m.Get("/v1beta1/space/:space/cronjobs/:jobName", jobs.GetCronJob)
	m.Delete("/v1beta1/space/:space/cronjobs/:jobName", jobs.DeleteCronJob)
	m.Get("/v1beta1/space/:space/cronjobs/:jobName/run", jobs.GetDeployedCronJob)
	m.Post("/v1beta1/space/:space/cronjobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.DeployCronJob)
	m.Put("/v1beta1/space/:space/cronjobs/:jobName/run", binding.Json(structs.JobDeploy{}), jobs.UpdatedDeployedCronJob)
	m.Delete("/v1beta1/space/:space/cronjobs/:jobName/run", jobs.StopCronJob)

	m.Post("/v1/space/:space/app/:app/maintenance", maintenance.EnableMaintenancePage)
	m.Delete("/v1/space/:space/app/:app/maintenance", maintenance.DisableMaintenancePage)
	m.Get("/v1/space/:space/app/:app/maintenance", maintenance.MaintenancePageStatus)

	m.Post("/v1/certs", binding.Json(structs.CertificateRequestSpec{}), certs.CertificateRequest)
	m.Get("/v1/certs", certs.GetCerts)
	m.Get("/v1/certs/:id", certs.GetCertStatus)
	m.Post("/v1/certs/:id/install", certs.InstallCert)

	m.Get("/v1/utils/service/space/:space/app/:app", utils.GetService)
	m.Get("/v1/utils/nodes", utils.GetNodes)
	m.Get("/v1/utils/urltemplates", templates.GetURLTemplates)

	InitOpenServiceBrokerEndpoints(db, m)
	InitOldServiceEndpoints(m)

	// proxy to log shuttle
	if os.Getenv("LOGSHUTTLE_SERVICE_HOST") != "" && os.Getenv("LOGSHUTTLE_SERVICE_PORT") != "" {
		m.Get("/apps/:app_key/log-drains", ProxyToShuttle)
		m.Post("/apps/:app_key/log-drains", ProxyToShuttle)
		m.Delete("/apps/:app_key/log-drains/:id", ProxyToShuttle)
		m.Get("/apps/:app_key/log-drains/:id", ProxyToShuttle)
		m.Get("/sites/:app_key/log-drains", ProxyToShuttle)
		m.Post("/sites/:app_key/log-drains", ProxyToShuttle)
		m.Delete("/sites/:app_key/log-drains/:id", ProxyToShuttle)
		m.Get("/sites/:app_key/log-drains/:id", ProxyToShuttle)
		m.Post("/log-events", ProxyToShuttle)
	} else {
		log.Println("No LOGSHUTTLE_SERVICE_HOST and LOGSHUTTLE_SERVICE_PORT environment variables found, log shuttle functionality was disabled.")
	}
	// proxy to log session
	if os.Getenv("LOGSESSION_SERVICE_HOST") != "" && os.Getenv("LOGSESSION_SERVICE_PORT") != "" {
		m.Post("/log-sessions", ProxyToSession)
		m.Get("/log-sessions/:id", ProxyToSession)
	} else {
		log.Println("No LOGSESSION_SERVICE_HOST and LOGSESSION_SERVICE_PORT environment variables found, log session functionality was disabled.")
	}
	// proxy influxdb
	if os.Getenv("INFLUXDB_URL") != "" {
		m.Get("/query**", ProxyToInfluxDb)
	} else {
		log.Println("No INFLUXDB_URL environment variables found, influx metrics functionality was disabled.")
	}
	// proxy prometheus
	if os.Getenv("PROMETHEUS_URL") != "" {
		m.Get("/api/v1/query_range**", ProxyToPrometheus)
	} else {
		log.Println("No PROMETHEUS_URL environment variables found, prometheus metrics functionality was disabled.")
	}

	if os.Getenv("ENABLE_AUTH") == "true" {
		m.Use(auth.Basic(utils.AuthUser, utils.AuthPassword))
	}

	return m
}

func CreateDB(db *sql.DB) {
	buf, err := ioutil.ReadFile("./create.sql")
	if err != nil {
		buf, err = ioutil.ReadFile("region-api/create.sql")
		if err != nil {
			buf, err = ioutil.ReadFile("../create.sql")
			if err != nil {
				log.Println("Error: Unable to run migration scripts, could not load create.sql.")
				log.Fatalln(err)
			}
		}
	}
	_, err = db.Query(string(buf))
	if err != nil {
		log.Println("Error: Unable to run migration scripts, execution failed.")
		log.Fatalln(err)
	}

	// This will inspect the stacks, if we have environment variables for a stack and
	// one does not exist in our database we'll create a record for it.
	var defaultStack = "ds1"
	if os.Getenv("DEFAULT_STACK") != "" {
		defaultStack = os.Getenv("DEFAULT_STACK")
	}
	var stackCount int
	err = db.QueryRow("select count(*) as stacks from stacks").Scan(&stackCount)
	if err != nil {
		log.Println("Error: Unable to determine how many stacks are available.")
		log.Fatalln(err)
	}
	var spacesCount int
	err = db.QueryRow("select count(*) as spaces from spaces").Scan(&spacesCount)
	if err != nil {
		log.Println("Error: Unable to determine how many spaces are available.")
		log.Fatalln(err)
	}

	if stackCount == 0 && os.Getenv("KUBERNETES_API_SERVER") != "" {
		authType := os.Getenv("KUBERNETES_CLIENT_TYPE")
		authPath := os.Getenv("KUBERNETES_CERT_SECRET")
		if authType == "token" {
			os.Getenv("KUBERNETES_TOKEN_SECRET")
		}
		_, err := db.Exec("insert into stacks (stack, description, api_server, api_version, image_pull_secret, auth_type, auth_vault_path) values ($1, $2, $3, $4, $5, $6, $7) on conflict (stack) do nothing",
			defaultStack, "", os.Getenv("KUBERNETES_API_SERVER"), os.Getenv("KUBERNETES_API_VERSION"), os.Getenv("KUBERNETES_IMAGE_PULL_SECRET"), authType, authPath)
		if err != nil {
			log.Println("Error: Unable to insert default kubernetes stack.")
			log.Fatalln(err)
		}
		if spacesCount == 0 {
			_, err = db.Exec("insert into spaces (name, internal, compliancetags, stack) values ('default', false, '', $1)", defaultStack)
			if err != nil {
				log.Println("Error: Unable to insert default kubernetes stack.")
				log.Fatalln(err)
			}
		} else {
			_, err = db.Exec("update spaces set stack=$1", defaultStack)
			if err != nil {
				log.Println("Error: Unable to insert default kubernetes stack.")
				log.Fatalln(err)
			}
		}
	}
}

func Init(pool *sql.DB) {
	m := Server(pool)
	CreateDB(pool)
	m.Map(pool)
	m.Run()
}

func TestInit() *martini.ClassicMartini {
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	CreateDB(pool)
	utils.InitAuth()
	m := Server(pool)
	m.Map(pool)
	return m
}
