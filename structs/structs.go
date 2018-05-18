package structs

import "time"

type Namespec struct {
	Name      string `json:"name"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Virtualspec struct {
	Rules []string `json:"rules"`
}

type Rulespec struct {
	Name         string `json:"name"`
	Partition    string `json:"partition"`
	ApiAnonymous string `json:"apiAnonymous"`
}

type Routerpathspec struct {
	Domain      string `json:"domain"`
	Path        string `json:"path"`
	Space       string `json:"space"`
	App         string `json:"app"`
	ReplacePath string `json:"replacepath"`
}

type Routerspec struct {
	Domain   string           `json:"domain"`
	Internal bool             `json:"internal"`
	Paths    []Routerpathspec `json:"paths"`
}

//Deployspec deployment spec
type Deployspec struct {
	AppName string   `json:"appname"`
	Image   string   `json:"appimage"`
	Space   string   `json:"space"`
	Port    int      `json:"port"`
	Command []string `json:"command"`
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

//Planspec  plans spec
type Planspec struct {
	Size        string `json:"size"`
	Description string `json:"description"`
}

//Redisspec Redis spec
type Redisspec struct {
	RedisUrl string `json:"REDIS_URL"`
	Spec     string `json:"spec"`
}

//Memcachedspec Redis spec
type Memcachedspec struct {
	MemcachedUrl string `json:"MEMCACHED_URL"`
	Spec         string `json:"spec"`
}

//ESspec spec
type Esspec struct {
	EsUrl     string `json:"ES_URL"`
	KibanaUrl string `json:"KIBANA_URL"`
	Spec      string `json:"spec"`
}

//Rabbitmqspec  spec
type Rabbitmqspec struct {
	RabbitmqUrl string `json:"RABBITMQ_URL"`
	Spec        string `json:"spec"`
}

//S3spec  spec
type S3spec struct {
	S3location  string `json:"S3_LOCATION"`
	S3bucket    string `json:"S3_BUCKET"`
	S3accesskey string `json:"S3_ACCESS_KEY"`
	S3secretkey string `json:"S3_SECRET_KEY"`
	Spec        string `json:"spec"`
}

//Postgresspec Postgres spec
type Postgresspec struct {
	DatabaseUrl string `json:"DATABASE_URL"`
	Spec        string `json:"spec"`
}

//Mongodbspec Postgres spec
type Mongodbspec struct {
	DatabaseUrl string `json:"DATABASE_URL"`
	Spec        string `json:"spec"`
}

//Auroramysqlspec mysql spec
type Auroramysqlspec struct {
	DatabaseUrl         string `json:"DATABASE_URL"`
	DatabaseReadonlyUrl string `json:"DATABASE_READONLY_URL"`
	Spec                string `json:"spec"`
}

type Deployment struct {
	Space string 
	App string
	Amount int
	RevisionHistoryLimit int
	Port int
	HealthCheck string
	Image string
	Tag string 
	Command []string 
	MemoryRequest string
	MemoryLimit string
	Secrets []Namespec
	ConfigVars []EnvVar
	Schedule string
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
	Stack		   string `json:"stack,omitempty"`
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
	Name      string       `json:"name"`
	Resources ResourceSpec `json:"resources"`
	Price     int          `json:"price"`
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

type Rule struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Partition    string `json:"partition"`
	FullPath     string `json:"fullPath"`
	Generation   int    `json:"generation"`
	SelfLink     string `json:"selfLink"`
	APIAnonymous string `json:"apiAnonymous"`
}

type Switch struct {
	Path        string
	ReplacePath string
	NewHost     string
	Pool        string
}

type RuleInfo struct {
	Domain   string
	Switches []Switch
}

type CertificateRequest struct {
	Response CertificateRequestResponseSpec `json:request`
	ID       string                         `json:"id"`
}

type CertificateRequestResponseSpec struct {
	ID       int `json:"id"`
	Requests []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"requests"`
}

type CertificateRequestObject struct {
	ID            int       `json:"id"`
	Date          time.Time `json:"date"`
	Type          string    `json:"type"`
	Status        string    `json:"status"`
	DateProcessed time.Time `json:"date_processed"`
	Requester     struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"requester"`
	Processor struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"processor"`
	Order struct {
		ID          int `json:"id"`
		Certificate struct {
			ID           int       `json:"id"`
			CommonName   string    `json:"common_name"`
			DNSNames     []string  `json:"dns_names"`
			DateCreated  time.Time `json:"date_created"`
			Csr          string    `json:"csr"`
			Organization struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				City    string `json:"city"`
				State   string `json:"state"`
				Country string `json:"country"`
			} `json:"organization"`
			ServerPlatform struct {
				ID         int    `json:"id"`
				Name       string `json:"name"`
				InstallURL string `json:"install_url"`
				CsrURL     string `json:"csr_url"`
			} `json:"server_platform"`
			SignatureHash string `json:"signature_hash"`
			KeySize       int    `json:"key_size"`
			CaCert        struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"ca_cert"`
		} `json:"certificate"`
		Status       string    `json:"status"`
		IsRenewal    bool      `json:"is_renewal"`
		DateCreated  time.Time `json:"date_created"`
		Organization struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			City    string `json:"city"`
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"organization"`
		ValidityYears               int  `json:"validity_years"`
		DisableRenewalNotifications bool `json:"disable_renewal_notifications"`
		AutoRenew                   int  `json:"auto_renew"`
		Container                   struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"container"`
		Product struct {
			NameID                string `json:"name_id"`
			Name                  string `json:"name"`
			Type                  string `json:"type"`
			ValidationType        string `json:"validation_type"`
			ValidationName        string `json:"validation_name"`
			ValidationDescription string `json:"validation_description"`
		} `json:"product"`
		OrganizationContact struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			JobTitle  string `json:"job_title"`
			Telephone string `json:"telephone"`
		} `json:"organization_contact"`
		TechnicalContact struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			JobTitle  string `json:"job_title"`
			Telephone string `json:"telephone"`
		} `json:"technical_contact"`
		User struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
		} `json:"user"`
		Requests []struct {
			ID       int       `json:"id"`
			Date     time.Time `json:"date"`
			Type     string    `json:"type"`
			Status   string    `json:"status"`
			Comments string    `json:"comments"`
		} `json:"requests"`
		CsProvisioningMethod string `json:"cs_provisioning_method"`
		ShipInfo             struct {
			Name    string `json:"name"`
			Addr1   string `json:"addr1"`
			Addr2   string `json:"addr2"`
			City    string `json:"city"`
			State   string `json:"state"`
			Zip     int    `json:"zip"`
			Country string `json:"country"`
			Method  string `json:"method"`
		} `json:"ship_info"`
	} `json:"order"`
	Comments         string `json:"comments"`
	ProcessorComment string `json:"processor_comment"`
}

type CertificateRequestSpec struct {
	ID            string   `json:"id"`
	Comment       string   `json:"comment,omitempty"`
	CN            string   `json:"cn"`
	SAN           []string `json:"san"`
	Key           string   `json:"key,omitempty"`
	CSR           string   `json:"csr,omitempty"`
	Request       string   `json:"request,omitempty"`
	Requestedby   string   `json:"requestedby,omitempty"`
	Requesteddate string   `json:"requesteddate,omitempty"`
	RequestStatus string   `json:"requeststatus,omitempty"`
	Order         string   `json:"order,omitempty"`
	OrderStatus   string   `json:"orderstatus,omitempty"`
	Installed     bool     `json:"installed,omitempty"`
	Installeddate string   `json:"installeddate,omitempty"`
	ValidFrom     string   `json:"validfrom,omitempty"`
	ValidTo       string   `json:"validto,omitempty"`
	VIP           string   `json:"vip,omitempty"`
	SignatureHash string   `json:"signature,omitempty"`
}

type DigicertRequest struct {
	Certificate struct {
		CommonName        string   `json:"common_name"`
		DNSNames          []string `json:"dns_names,omitempty"`
		Csr               string   `json:"csr"`
		OrganizationUnits []string `json:"organization_units"`
		ServerPlatform    struct {
			ID int `json:"id"`
		} `json:"server_platform"`
		SignatureHash string `json:"signature_hash"`
	} `json:"certificate"`
	Organization struct {
		ID int `json:"id"`
	} `json:"organization"`
	ValidityYears int    `json:"validity_years"`
	Comments      string `json:"comments,omitempty"`
}

type OrderList struct {
	Orders []struct {
		ID          int `json:"id"`
		Certificate struct {
			ID            int      `json:"id"`
			CommonName    string   `json:"common_name"`
			DNSNames      []string `json:"dns_names"`
			ValidTill     string   `json:"valid_till"`
			SignatureHash string   `json:"signature_hash"`
		} `json:"certificate"`
		Status       string    `json:"status"`
		IsRenewed    bool      `json:"is_renewed"`
		DateCreated  time.Time `json:"date_created"`
		Organization struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"organization"`
		ValidityYears int `json:"validity_years"`
		Container     struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"container"`
		Product struct {
			NameID string `json:"name_id"`
			Name   string `json:"name"`
			Type   string `json:"type"`
		} `json:"product"`
		HasDuplicates bool   `json:"has_duplicates"`
		Price         int    `json:"price"`
		ProductNameID string `json:"product_name_id"`
	} `json:"orders"`
	Page struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"page"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type OrderSpec struct {
	ID          int `json:"id"`
	Certificate struct {
		ID           int       `json:"id"`
		Thumbprint   string    `json:"thumbprint"`
		SerialNumber string    `json:"serial_number"`
		CommonName   string    `json:"common_name"`
		DNSNames     []string  `json:"dns_names"`
		DateCreated  time.Time `json:"date_created"`
		ValidFrom    string    `json:"valid_from"`
		ValidTill    string    `json:"valid_till"`
		Csr          string    `json:"csr"`
		Organization struct {
			ID int `json:"id"`
		} `json:"organization"`
		OrganizationUnits []string `json:"organization_units"`
		ServerPlatform    struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			InstallURL string `json:"install_url"`
			CsrURL     string `json:"csr_url"`
		} `json:"server_platform"`
		SignatureHash string `json:"signature_hash"`
		KeySize       int    `json:"key_size"`
		CaCert        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"ca_cert"`
	} `json:"certificate"`
	Status string `json:"status"`
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
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Items      []KubeNodeItems `json:"items"`
}

type URLTemplates struct {
	Internal string `json:"internal"`
	External string `json:"external"`
}

