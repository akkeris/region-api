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
	"region-api/jobs"
	"region-api/maintenance"
	"region-api/router"
	"region-api/runtime"
	"region-api/service"
	"region-api/space"
	"region-api/structs"
	"region-api/templates"
	"region-api/utils"
	"region-api/vault"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/auth"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/robfig/cron"
)

func GetInfo(db *sql.DB, params martini.Params, r render.Render) {
	r.JSON(http.StatusOK, map[string]string{
		"kafka_hosts": os.Getenv("KAFKA_BROKERS"),
	})
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func Proxy(uri string, message string) *httputil.ReverseProxy {
	target, err := url.Parse(uri)
	if err != nil {
		log.Println("Error: Unable to proxy to " + message)
		log.Println(err)
		return nil
	}
	log.Println("Proxying to", target)
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		req.Header.Set("Host", target.Host)
	}
	return &httputil.ReverseProxy{Director: director}
}

func ProxyToShuttle(res http.ResponseWriter, req *http.Request) {
	logshuttle_url := "http://" + os.Getenv("LOGSHUTTLE_SERVICE_HOST") + ":" + os.Getenv("LOGSHUTTLE_SERVICE_PORT")
	rp := Proxy(logshuttle_url, "logshuttle")
	if rp == nil {
		return
	}
	rp.ServeHTTP(res, req)
}

func ProxyToSession(res http.ResponseWriter, req *http.Request) {
	logsession_url := "http://" + os.Getenv("LOGSESSION_SERVICE_HOST") + ":" + os.Getenv("LOGSESSION_SERVICE_PORT")
	rp := Proxy(logsession_url, "logsession")
	if rp == nil {
		return
	}
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func ProxyToInfluxDb(res http.ResponseWriter, req *http.Request) {
	rp := Proxy(os.Getenv("INFLUXDB_URL"), "influxdb")
	if rp == nil {
		return
	}
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func ProxyToPrometheus(res http.ResponseWriter, req *http.Request) {
	rp := Proxy(os.Getenv("PROMETHEUS_URL"), "prometheus")
	if rp == nil {
		return
	}
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

func InitOldServiceEndpoints(m *martini.ClassicMartini) {
	// While moving over to the open service broker api spec
	// these old end points formats should no longer be used.
	m.Get("/v1/service/kafka/plans", service.GetKafkaPlansV1)
	m.Post("/v1/service/kafka/instance", binding.Json(structs.Provisionspec{}), service.ProvisionKafkaV1)
	m.Delete("/v1/service/kafka/instance/:servicename", service.DeleteKafkaV1)
	m.Post("/v1/service/kafka/cluster/:cluster/topic", binding.Json(structs.KafkaTopic{}), service.ProvisionTopicV1)
	m.Get("/v1/service/kafka/topics", service.GetTopicsV1)
	m.Delete("/v1/service/kafka/cluster/:cluster/topics/:topic", service.DeleteTopicV1)
	m.Get("/v1/service/kafka/topics/:topic", service.GetTopicV1)
	m.Get("/v1/service/kafka/cluster/:cluster/configs", service.GetConfigsV1)
	m.Get("/v1/service/kafka/cluster/:cluster/configs/:name", service.GetConfigV1)
	m.Get("/v1/service/kafka/cluster/:cluster/schemas", service.GetSchemasV1)
	m.Get("/v1/service/kafka/cluster/:cluster/schemas/:schema", service.GetSchemaV1)
	m.Post("/v1/service/kafka/cluster/:cluster/topic-key-mapping", binding.Json(structs.TopicKeyMapping{}), service.CreateTopicKeyMappingV1)
	m.Post("/v1/service/kafka/cluster/:cluster/topic-schema-mapping", binding.Json(structs.TopicSchemaMapping{}), service.CreateTopicSchemaMappingV1)
	m.Get("/v1/service/kafka/cluster/:cluster/acls", service.GetAclsV1)
	m.Post("/v1/service/kafka/cluster/:cluster/acls", binding.Json(structs.AclRequest{}), service.CreateAclV1)
	m.Delete("/v1/service/kafka/acls/:id", service.DeleteAclV1)
	m.Get("/v1/service/kafka/cluster/:cluster/topics/:topic/preview", service.GetTopicPreviewV1)
	m.Get("/v1/service/kafka/cluster/:cluster/consumer-groups", service.GetConsumerGroupsV1)
	m.Get("/v1/service/kafka/cluster/:cluster/consumer-groups/:consumerGroupName/offsets", service.GetConsumerGroupOffsetsV1)
	m.Get("/v1/service/kafka/cluster/:cluster/consumer-groups/:consumerGroupName/members", service.GetConsumerGroupMembersV1)
	m.Post("/v1/service/kafka/cluster/:cluster/consumer-groups/:consumerGroupName/seek", binding.Json(structs.KafkaConsumerGroupSeekRequest{}), service.SeekConsumerGroupV1)

	m.Get("/v1/service/influxdb/plans", service.GetInfluxdbPlans)
	m.Get("/v1/service/influxdb/url/:servicename", service.GetInfluxdbURL)
	m.Post("/v1/service/influxdb/instance", binding.Json(structs.Provisionspec{}), service.ProvisionInfluxdb)
	m.Delete("/v1/service/influxdb/instance/:servicename", service.DeleteInfluxdb)

	m.Get("/v1/service/rabbitmq/plans", service.Getrabbitmqplans)
	m.Post("/v1/service/rabbitmq/instance", binding.Json(structs.Provisionspec{}), service.Provisionrabbitmq)
	m.Get("/v1/service/rabbitmq/url/:servicename", service.Getrabbitmqurl)
	m.Delete("/v1/service/rabbitmq/instance/:servicename", service.Deleterabbitmq)
	m.Post("/v1/service/rabbitmq/instance/tag", binding.Json(structs.Tagspec{}), service.Tagrabbitmq)

	m.Get("/v1/service/:service/bindings", service.GetBindingList)
}

var catalogOSBProvider *service.OSBClientServices

func InitOpenServiceBrokerEndpoints(db *sql.DB, m *martini.ClassicMartini) {
	var err error
	catalogOSBProvider, err = service.NewOSBClientServices(strings.Split(os.Getenv("SERVICES"), ","), db)
	if err != nil {
		log.Fatalln(err)
	}
	service.SetGlobalClientService(catalogOSBProvider)
	m.Get("/v2/catalog", catalogOSBProvider.HttpGetCatalog)
	m.Get("/v2/service_instances/:instance_id/last_operation", catalogOSBProvider.HttpGetLastOperation)
	m.Put("/v2/service_instances/:instance_id", binding.Json(service.ProvisionRequestBody{}), catalogOSBProvider.HttpGetCreateOrUpdateInstance)
	m.Delete("/v2/service_instances/:instance_id", catalogOSBProvider.HttpDeleteInstance)
	m.Patch("/v2/service_instances/:instance_id", binding.Json(service.UpdateRequestBody{}), catalogOSBProvider.HttpPartialUpdateInstance)
	m.Put("/v2/service_instances/:instance_id/service_bindings/:binding_id", binding.Json(service.BindRequestBody{}), catalogOSBProvider.HttpCreateOrUpdateBinding)
	m.Get("/v2/service_instances/:instance_id/service_bindings/:binding_id", catalogOSBProvider.HttpGetBinding)
	m.Get("/v2/service_instances/:instance_id/service_bindings/:binding_id/last_operation", catalogOSBProvider.HttpGetBindingLastOperation)
	m.Delete("/v2/service_instances/:instance_id/service_bindings/:binding_id", catalogOSBProvider.HttpRemoveBinding)
	// Custom Actions
	m.Delete("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Get("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Patch("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Put("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Post("/v2/service_instances/:instance_id/actions/:action_id", catalogOSBProvider.HttpForwardAction)
	m.Delete("/v2/service_instances/:instance_id/actions/:action_id/:action_subject", catalogOSBProvider.HttpForwardAction)
	m.Get("/v2/service_instances/:instance_id/actions/:action_id/:action_subject", catalogOSBProvider.HttpForwardAction)
	m.Patch("/v2/service_instances/:instance_id/actions/:action_id/:action_subject", catalogOSBProvider.HttpForwardAction)
	m.Put("/v2/service_instances/:instance_id/actions/:action_id/:action_subject", catalogOSBProvider.HttpForwardAction)
	m.Post("/v2/service_instances/:instance_id/actions/:action_id/:action_subject", catalogOSBProvider.HttpForwardAction)
}

func Server(db *sql.DB) *martini.ClassicMartini {
	m := martini.Classic()
	m.Use(render.Renderer())

	// cause the dns provider to begin caching itself.
	go router.GetDnsProvider()
	// cause runtime to cache itself.
	go runtime.GetAllRuntimes(db)

	m.Get("/v2/config", GetInfo)

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
	m.Post("/v1/app/deploy", binding.Json(structs.Deployspec{}), app.Deployment)
	m.Post("/v1/app/deploy/oneoff", binding.Json(structs.OneOffSpec{}), app.OneOffDeployment)
	m.Post("/v1/app/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v1/app/:appname/bind/:bindspec", app.Unbindapp)
	m.Get("/v1/app/:appname", app.Describeapp)
	m.Get("/v1/space/:space/app/:app/instance", app.GetInstances)
	m.Get("/v1/space/:space/app/:appname/instance/:instanceid/log", app.GetAppLogs)
	m.Delete("/v1/space/:space/app/:app/instance/:instanceid", app.DeleteInstance)
	m.Post("/v1/space/:space/app/:app/instance/:instance/exec", binding.Json(structs.Exec{}), app.Exec)
	m.Get("/v1/apps", app.Listapps)
	m.Get("/v1/apps/plans", app.GetPlans)
	m.Post("/v1/space/:space/app/:app/rollback/:revision", app.Rollback)
	m.Post("/v1/space/:space/app/:appname/bind", binding.Json(structs.Bindspec{}), app.Createbind)
	m.Delete("/v1/space/:space/app/:appname/bind/**", app.Unbindapp)
	m.Post("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", binding.Json(structs.Bindmapspec{}), app.Createbindmap)
	m.Get("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname", app.Getbindmaps)
	m.Delete("/v1/space/:space/app/:appname/bindmap/:bindtype/:bindname/:mapid", app.Deletebindmap)
	m.Get("/v1/space/:space/apps", app.Describespace)
	m.Get("/v1/space/:space/app/:appname", app.DescribeappInSpace)
	m.Get("/v1/space/:space/app/:appname/configvars", app.GetAllConfigVars)
	m.Get("/v1/space/:space/app/:appname/configvars/:bindtype/:bindname", app.GetServiceConfigVars)
	m.Post("/v1/space/:space/app/:appname/restart", app.Restart)
	m.Get("/v1/space/:space/app/:app/status", app.Spaceappstatus)
	m.Get("/v1/kube/podstatus/:space/:app", app.PodStatus)

	m.Get("/v1/spaces", space.Listspaces)
	m.Post("/v1/space", binding.Json(structs.Spacespec{}), space.Createspace)
	m.Delete("/v1/space/:space", binding.Json(structs.Spacespec{}), space.Deletespace)
	m.Get("/v1/space/:space", space.Space)
	m.Put("/v1/space/:space/tags", binding.Json(structs.Spacespec{}), space.UpdateSpaceTags)
	m.Put("/v1/space/:space/app/:app", binding.Json(structs.Spaceappspec{}), space.AddApp)
	m.Put("/v1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), space.UpdateAppHealthCheck)
	m.Delete("/v1/space/:space/app/:app/healthcheck", space.DeleteAppHealthCheck)
	m.Put("/v1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), space.UpdateAppPlan)
	m.Put("/v1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), space.ScaleApp)
	m.Delete("/v1/space/:space/app/:app", space.DeleteApp)

	vault.AddToMartini(m)
	router.AddToMartini(m)
	certs.AddToMartini(m)

	m.Get("/v1/octhc/kube", utils.Octhc)
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

	m.Get("/v1/utils/service/space/:space/app/:app", utils.GetService)
	m.Get("/v1/utils/nodes", utils.GetNodes)
	m.Get("/v1/utils/urltemplates", templates.GetURLTemplates)

	InitOpenServiceBrokerEndpoints(db, m)
	InitOldServiceEndpoints(m)

	// Add V2 Endpoints
	initV2Endpoints(m)

	vault.GetVaultListPeriodic()
	c := cron.New()
	c.AddFunc("@every 10m", func() { go vault.GetVaultListPeriodic() })
	c.Start()

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

	// ========================================
	// Temporary v2 Schema Migration Variables
	// ========================================

	// --  $1: controller-api database host
	// --  $2: controller-api database name
	// --  $3: controller-api database username
	// --  $4: controller-api database password

	if os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_HOST") == "" {
		log.Fatalln("Error: Unable to run v2 schema migration - Missing controller-api database host!")
	} else if os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_NAME") == "" {
		log.Fatalln("Error: Unable to run v2 schema migration - Missing controller-api database name!")
	} else if os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_USERNAME") == "" {
		log.Fatalln("Error: Unable to run v2 schema migration - Missing controller-api database username!")
	} else if os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_PASSWORD") == "" {
		log.Fatalln("Error: Unable to run v2 schema migration - Missing controller-api database password!")
	}

	// revive:disable

	v2temp_ControllerDBHost := os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_HOST")
	v2temp_ControllerDBName := os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_NAME")
	v2temp_ControllerDBUser := os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_USERNAME")
	v2temp_ControllerDBPassword := os.Getenv("V2_TEMP_CONTROLLER_API_DATABASE_PASSWORD")

	// revive:enable

	if _, err = db.Query(
		string(buf),
		v2temp_ControllerDBHost,
		v2temp_ControllerDBName,
		v2temp_ControllerDBUser,
		v2temp_ControllerDBPassword,
	); err != nil {
		log.Println("Error: Unable to run migration scripts, execution failed.")
		log.Fatalln(err)
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
