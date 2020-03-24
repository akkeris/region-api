package structs

import (
	"time"
	"gopkg.in/guregu/null.v3/zero"
)

type Namespec struct {
	Name string `json:"name"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HttpFilters struct {
	Type string `json:"type"`
	Data map[string]string `json:"data"`
}

//Deployspec deployment spec
type Deployspec struct {
	AppName  string   `json:"appname"`
	Image    string   `json:"appimage"`
	Space    string   `json:"space"`
	Port     int      `json:"port"`
	Command  []string `json:"command"`
	Features Features `json:"features,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Filters  []HttpFilters `json:"filters,omitempty"`
}

type Features struct {
	ServiceMesh bool `json:"serviceMesh,omitempty"`
	IstioInject bool `json:"istioInject,omitempty"`
	Http2Service bool `json:"http2,omitempty"`
	Http2EndToEndService bool `json:"http2-end-to-end,omitempty"`
}

//Setspec setspec
type Setspec struct {
	Setname string `json:"name"`
	Settype string `json:"type"`
}

//Varspec varspec
type Varspec struct {
	Setname  string `json:"setname"`
	Varname  string `json:"varname"`
	Varvalue string `json:"varvalue"`
}

//Tagspec tagspec
type Tagspec struct {
	Resource string `json:"resource"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

type Exec struct {
	Command []string `json:"command"`
	Stdin string `json:"stdin"`
}

//Provisionspec provisionspec
type Provisionspec struct {
	Plan        string `json:"plan"`
	Billingcode string `json:"billingcode"`
}

type Messagespec struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

//Appspec application spec
type Appspec struct {
	Name string `json:"appname"`
	Port int    `json:"appport"`
	//Instances int        `json:"instances"`
	//Bindings  []Bindspec `json:"bindings"`
	Spaces []Spaceappspec `json:"spaces"`
}

type Applist struct {
	Apps []string `json:"apps"`
}

type Spacelist struct {
	Spaces []string `json:"spaces"`
}

//Spaceappspec application spec
type Spaceappspec struct {
	Appname     string     `json:"appname"`
	Space       string     `json:"space"`
	Instances   int        `json:"instances"`
	Bindings    []Bindspec `json:"bindings"`
	Plan        string     `json:"plan"`
	Healthcheck string     `json:"healthcheck,omitempty"`
	Image       string     `json:"image"`
}

//Bindspec bind spec
type Bindspec struct {
	App      string `json:"appname"`
	Space    string `json:"space"`
	Bindtype string `json:"bindtype"`
	Bindname string `json:"bindname"`
}

//Bindspec bind spec
type Bindmapspec struct {
	Id       string `json:"id",omitempty`
	App      string `json:"appname"`
	Space    string `json:"space"`
	Bindtype string `json:"bindtype"`
	Bindname string `json:"bindname"`
	VarName  string `json:"varname"`
	NewName  string `json:"newname",omitempty`
	Action   string `json:"action"`
}

//Planspec  plans spec
type Planspec struct {
	Size        string `json:"size"`
	Description string `json:"description"`
}

//Rabbitmqspec  spec
type Rabbitmqspec struct {
	RabbitmqUrl   string `json:"RABBITMQ_URL"`
	RabbitmqUiUrl string `json:"RABBITMQUI_URL"`
	Spec          string `json:"spec"`
}

//Postgresspec Postgres spec
type Postgresspec struct {
	DatabaseUrl string `json:"DATABASE_URL"`
	Spec        string `json:"spec"`
}

//Mongodbspec Postgres spec
type Mongodbspec struct {
	MongodbUrl string `json:"MONGODB_URL"`
	Spec       string `json:"spec"`
}

//Auroramysqlspec mysql spec
type Auroramysqlspec struct {
	DatabaseUrl         string `json:"DATABASE_URL"`
	DatabaseReadonlyUrl string `json:"DATABASE_READONLY_URL"`
	Spec                string `json:"spec"`
}

//Neptunespec Neptune db spec
type Neptunespec struct {
	NeptuneDatabaseURL string `json:"NEPTUNE_DATABASE_URL"`
	NeptuneAccessKey   string `json:"NEPTUNE_ACCESS_KEY"`
	NeptuneSecretKey   string `json:"NEPTUNE_SECRET_KEY"`
	NeptuneRegion      string `json:"NEPTUNE_REGION"`
	Spec               string `json:"spec"`
}

type Influxdbspec struct {
	Name     string `json:"INFLUX_DB"`
	Url      string `json:"INFLUX_URL"`
	Username string `json:"INFLUX_USERNAME"`
	Password string `json:"INFLUX_PASSWORD"`
	Spec     string `json:"spec"`
}


type Deployment struct {
	Space                string
	App                  string
	Amount               int
	RevisionHistoryLimit int
	Port                 int
	HealthCheck          string
	Image                string
	Tag                  string
	Command              []string
	MemoryRequest        string
	MemoryLimit          string
	Secrets              []Namespec
	ConfigVars           []EnvVar
	Schedule             string
	Features             Features
	Labels				 map[string]string
	PlanType			 string
}

//Deployresponse deploy response
type Deployresponse struct {
	Controller string `json:"controller"`
	Service    string `json:"service"`
}

//Brokerresponse broker response
type Brokerresponse struct {
	Response string `json:"response"`
}

type Vaultsecretspec struct {
	LeaseID       string      `json:"lease_id"`
	LeaseDuration int         `json:"lease_duration"`
	Renewable     bool        `json:"renewable"`
	Warnings      interface{} `json:"warnings"`
	Data          struct {
		Base64 string `json:"base64"`
		Name   string `json:"name"`
	} `json:"data"`
}

type Spacespec struct {
	Name           string `json:"name"`
	Internal       bool   `json:"internal"`
	ComplianceTags string `json:"compliancetags"`
	Stack          string `json:"stack,omitempty"`
}

type DeploymentsSpec struct {
	Name              string    `json:"name"`
	Space             string    `json:"namespace"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Image             string    `json:"image"`
	Revision          string    `json:"revision"`
}

type SpaceAppStatus struct {
	App            string                 `json:"app"`
	Space          string                 `json:"space"`
	Status         int                    `json:"status"`
	Output         string                 `json:"output"`
	ExecutionTime  string                 `json:"executiontime"`
	ExtendedOutput string                 `json:"extendedoutput"`
	LastCheckTime  int                    `json:"lastchecktime"`
	Reason         string                 `json:"reason"`
	State          map[string]interface{} `json:"state"`
	Ready          bool                   `json:"ready"`
	Restarted      int                    `json:"restarted"`
}

type Subscriber struct {
	Subscriber   string
	Servicegroup string
}
type Subscriberspec struct {
	Appname string `json:"appname"`
	Space   string `json:"space"`
	Email   string `json:"email"`
}

type NagiosAlert struct {
	Servicegroup     string `json:"servicegroup"`
	CheckType        string `json:"checktype"`
	LongDateTime     string `json:"longdatetime"`
	NotificationType string `json:"notificationtype"`
	Name             string `json:"name"`
	Alias            string `json:"alias"`
	Address          string `json:"address"`
	State            string `json:"state"`
	Output           string `json:"output"`
	Appname          string `json:"appname"`
	Space            string `json:"space"`
}

type Callbackspec struct {
	Space   string `json:"space"`
	Appname string `json:"appname"`
	Url     string `json:"url"`
	Tag     string `json:"tag"`
	Method  string `json:"method"`
}

type VaultSecret struct {
	LeaseID       string `json:"lease_id"`
	Renewable     bool   `json:"renewable"`
	LeaseDuration int    `json:"lease_duration"`
	Data          struct {
		Hostname     string `json:"hostname"`
		Location     string `json:"location"`
		Password     string `json:"password"`
		Port         string `json:"port"`
		Resourcename string `json:"resourcename"`
		Username     string `json:"username"`
	} `json:"data"`
	Warnings interface{} `json:"warnings"`
	Auth     interface{} `json:"auth"`
}

type VaultList struct {
	LeaseID       string `json:"lease_id"`
	Renewable     bool   `json:"renewable"`
	LeaseDuration int    `json:"lease_duration"`
	Data          struct {
		Keys []string `json:"keys"`
	} `json:"data"`
	Warnings interface{} `json:"warnings"`
	Auth     interface{} `json:"auth"`
}

type Creds struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Appstatus struct {
	App         string    `json:"app"`
	ReadyStatus bool      `json:"readystatus"`
	StartedAt   time.Time `json:"startedat"`
}

type Instance struct {
	InstanceID string      `json:"instanceid"`
	Phase      string      `json:"phase"`
	StartTime  time.Time   `json:"starttime"`
	Reason     string      `json:"reason"`
	Appstatus  []Appstatus `json:"appstatus"`
}

type PromStat struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type PodMemoryspec struct {
	Namespace string `json:"namespace"`
	Podname   string `json:"podname"`
	MemoryMB  int    `json:"memorymb"`
}

type JobStatus struct {
	Metadata struct {
		Name                       string `json:"name"`
		Namespace                  string `json:"namespace"`
		DeletionGracePeriodSeconds int    `json:"deletionGracePeriodSeconds,omitempty"`
		Labels                     struct {
			Name  string `json:"name"`
			Space string `json:"space"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Parallelism           int `json:"parallelism,omitempty"`
		Completions           int `json:"completions,omitempty"`
		ActiveDeadlineSeconds int `json:"activeDeadlineSeconds,omitempty"`
	} `json:"spec"`
	Status struct {
		Conditions []struct {
			Type               string `json:"type,omitempty"`
			Status             string `json:"status,omitempty"`
			LastProbeTime      string `json:"lastProbeTime,omitempty"`
			LastTransitionTime string `json:"lastTransitionTime,omitempty"`
			Reason             string `json:"reason,omitempty"`
			Message            string `json:"message,omitempty"`
		} `json:"conditions"`
		StartTime      string `json:"startTime,omitempty"`
		CompletionTime string `json:"completionTime,omitempty"`
		Active         int    `json:"active,omitempty"`
		Succeeded      int    `json:"succeeded,omitempty"`
		Failed         int    `json:"failed,omitempty"`
	} `json:"status"`
}

type CronJobStatus struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Labels    struct {
			Name  string `json:"name"`
			Space string `json:"space"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Schedule                string `json:"schedule"`
		StartingDeadlineSeconds int    `json:"startingDeadlineSeconds"`
		ConcurrencyPolicy       string `json:"concurrencyPolicy"`
		Suspend                 bool   `json:"suspend"`
	} `json:"spec"`
	Active []struct {
		Kind            string `json:"kind"`
		Namespace       string `json:"namespace"`
		Name            string `json:"name"`
		UID             string `json:"uid"`
		APIVersion      string `json:"apiVersion"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"active"`
	LastScheduleTime time.Time `json:"lastScheduleTime"`
}

type ResourceSpec struct {
	Requests struct {
		Memory string `json:"memory,omitempty"`
		CPU    string `json:"cpu,omitempty"`
	} `json:"requests"`
	Limits struct {
		Memory string `json:"memory,omitempty"`
		CPU    string `json:"cpu,omitempty"`
	} `json:"limits"`
}

type QoS struct {
	Name        string       `json:"name"`
	Resources   ResourceSpec `json:"resources"`
	Price       int          `json:"price"`
	Description zero.String  `json:"description"`
	Deprecated  bool         `json:"deprecated"`
	Type        zero.String  `json:"type"`
}

type OneOffSpec struct {
	Space   string   `json:"space"`
	Podname string   `json:"podname"`
	Image   string   `json:"image"`
	Command string   `json:"command,omitempty"`
	Env     []EnvVar `json:"env"`
}

type JobList struct {
	Items []JobStatus `json:"items"`
}

type CronJobList struct {
	Items []CronJobStatus `json:"items"`
}

// JobReq request structure, stored in database
type JobReq struct {
	Name     string `json:"name"`  // required
	Space    string `json:"space"` // required
	CMD      string `json:"cmd,omitempty"`
	Schedule string `json:"schedule,omitempty"`
	Plan     string `json:"plan"`
}

// JobDeploy structure, passed to kubernetes
type JobDeploy struct {
	Image                 string `json:"image,omitempty"`
	DeleteBeforeCreate    bool   `json:"deleteBeforeCreate,omitempty"`
	Instances             int    `json:"instances,omitempty"`
	RestartPolicy         string `json:"restartPolicy,omitempty"`
	ActiveDeadlineSeconds int    `json:"activeDeadlineSeconds,omitempty"`
}

type Response struct {
	Status int
	Body   []byte
}

type Logspec struct {
	Log    string    `json:"log"`
	Stream string    `json:"stream"`
	Time   time.Time `json:"time"`
	Docker struct {
		ContainerID string `json:"container_id"`
	} `json:"docker"`
	Kubernetes struct {
		NamespaceName string `json:"namespace_name"`
		PodID         string `json:"pod_id"`
		PodName       string `json:"pod_name"`
		ContainerName string `json:"container_name"`
		Labels        struct {
			Name string `json:"name"`
		} `json:"labels"`
		Host string `json:"host"`
	} `json:"kubernetes"`
	Topic string `json:"topic"`
	Tag   string `json:"tag"`
}

type Maintenancespec struct {
	App    string `json:"app"`
	Space  string `json:"space"`
	Status string `json:"status"`
}
type F5creds struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}
type Rules struct {
	Items []struct {
		APIAnonymous string `json:"apiAnonymous"`
		FullPath     string `json:"fullPath"`
		Generation   int    `json:"generation"`
		Kind         string `json:"kind"`
		Name         string `json:"name"`
		Partition    string `json:"partition"`
		SelfLink     string `json:"selfLink"`
	} `json:"items"`
	Kind     string `json:"kind"`
	SelfLink string `json:"selfLink"`
}

type Switch struct {
	Path        string
	ReplacePath string
	NewHost     string
	Pool        string
	Nodeport    string
	Unipool     string
}

type RuleInfo struct {
	Domain   string
	Switches []Switch
}

type EnvVar struct {
	Name      string     `json:"name"`
	Value     string     `json:"value"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

type ValueFrom struct {
	FieldRef FieldRef `json:"fieldRef,omitempty"`
}
type FieldRef struct {
	FieldPath string `json:"fieldPath,omitempty"`
}

type SecurityContext struct {
	Privileged             bool `json:"privileged,omitempty"`
	ReadOnlyRootFilesystem bool `json:"readOnlyRootFilesystem,omitempty"`
	RunAsUser              int  `json:"runAsUser,omitempty"`
	Capabilities           struct {
		Add []string `json:"add,omitempty"`
	} `json:"capabilities,omitempty"`
}

type VolumeMounts struct {
	MountPath string `json:"mountPath,omitempty"`
	Name      string `json:"name,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type Volumes struct {
	Name     string    `json:"name,omitempty"`
	EmptyDir *EmptyDir `json:"emptyDir,omitempty"`
	Secret   *Secret   `json:"secret,omitempty"`
}

type EmptyDir struct {
	Medium string `json:"medium,omitempty"`
}

type Secret struct {
	Optional   bool   `json:"optional,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

type KubeNodeItems struct {
	Metadata struct {
		Name string `json:"name"`
		UID  string `json:"uid"`
	} `json:"metadata"`
	Spec struct {
		Unschedulable bool `json:"unschedulable"`
	} `json:"spec"`
	Status struct {
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
	} `json:"status"`
}

type KubeNodes struct {
	Kind       string          `json:"kind"`
	APIVersion string          `json:"apiVersion"`
	Items      []KubeNodeItems `json:"items"`
}

type URLTemplates struct {
	Internal string `json:"internal"`
	External string `json:"external"`
}

type KafkaTopic struct {
	Topic struct {
		Name   string `json:"name"`
		Config struct {
			Name          string `json:"name"`
			Cleanuppolicy string `json:"cleanup.policy,omitempty"`
			Partitions    *int   `json:"partitions,omitempty"`
			Retentionms   *int   `json:"retention.ms,omitempty"`
			Replicas      *int   `json:"replicas,omitempty"`
		} `json:"config"`
	} `json:"topic"`
}

type KafkaAclCredentials struct {
	AclCredentials struct {
		Username string `json:"username"`
	} `json:"aclCredentials"`
}

type Kafkaspec struct {
	Spec string `json:"spec"`
}

type TopicSchemaMapping struct {
	Topic  string `json:"topic"`
	Schema struct {
		Name string `json:"name"`
	} `json:"schema"`
}

type TopicKeyMapping struct {
	Topic   string `json:"topic"`
	KeyType string `json:"keyType"`
	Schema  *struct {
		Name string `json:"name"`
	} `json:"schema,omitempty"`
}

type AclRequest struct {
	Topic             string `json:"topic"`
	User              string `json:"user,omitempty"`
	Space             string `json:"space"`
	Appname           string `json:"app"`
	Role              string `json:"role"`
	ConsumerGroupName string `json:"consumerGroupName,omitempty"`
}

type KafkaConsumerGroupSeekRequest struct {
	Topic         string `json:"topic"`
	Partitions    []int  `json:"partitions,omitempty"`
	SeekTo        string `json:"seekTo"`
	AllPartitions bool   `json:"allPartitions,omitempty"`
}
