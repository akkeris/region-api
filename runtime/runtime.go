package runtime

import (
	"database/sql"
	structs "region-api/structs"
	"time"
)

type Specspec struct {
	Replicas int `json:"replicas"`
}

type Metadataspec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Itemspec struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

type Scalespec struct {
	Metadata Metadataspec `json:"metadata"`
	Spec     Specspec     `json:"spec"`
}

type Statusspec struct {
	Status string `json:"status"`
}

type Dataspec struct {
	Dockercfg string `json:".dockerconfigjson"`
}

type Secretspec struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Data Dataspec `json:"data"`
	Type string   `json:"type"`
}

type Itemsspec struct {
	Items []Itemspec `json:"items"`
}

type Serviceaccountspec struct {
	Metadata         structs.Namespec   `json:"metadata"`
	ImagePullSecrets []structs.Namespec `json:"imagePullSecrets"`
}

type Namespacespec struct {
	Metadata struct {
		Name   string `json:"name"`
		Labels struct {
			ComplianceTags string `json:"compliancetags,omitempty"`
		} `json:"labels,omitempty"`
	} `json:"metadata"`
}

type ContainerPort struct {
	ContainerPort int `json:"containerPort"`
}

type TcpCheck struct {
	Port int `json:"port,omitempty"`
}

type HttpCheck struct {
	Port int    `json:"port,omitempty"`
	Path string `json:"path,omitempty"`
}

type ReadinessProbe struct {
	TCPSocket *TcpCheck  `json:"tcpSocket,omitempty"`
	HTTPGET   *HttpCheck `json:"httpGet,omitempty"`
}

type ContainerItem struct {
	Name             string                  `json:"name"`
	Image            string                  `json:"image"`
	Args             []string                `json:"args,omitempty"`
	Command          []string                `json:"command,omitempty"`
	Env              []structs.EnvVar        `json:"env,omitempty"`
	Ports            []ContainerPort         `json:"ports,omitempty"`
	ImagePullPolicy  string                  `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []structs.Namespec      `json:"imagePullSecrets,omitempty"`
	Resources        structs.ResourceSpec    `json:"resources,omitempty"`
	ReadinessProbe   *ReadinessProbe         `json:"readinessProbe,omitempty"`
	SecurityContext  structs.SecurityContext `json:"securityContext,omitempty"`
	VolumeMounts     []structs.VolumeMounts  `json:"volumeMounts",omitempty`
}

type Createspec struct {
	Message  string `json:"name"`
	Metadata struct {
		Uid string `json:"uid"`
	} `json:"metadata"`
}

type PortItem struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	NodePort   int    `json:"nodePort"`
}

type Service struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Annotations struct {
			ServiceBetaKubernetesIoAwsLoadBalancerInternal string `json:"service.beta.kubernetes.io/aws-load-balancer-internal"`
		} `json:"annotations"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Labels    struct {
			App  string `json:"app"`
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Selector struct {
			Name string `json:"name"`
		} `json:"selector"`
		Ports []PortItem `json:"ports"`
		Type  string     `json:"type"`
	} `json:"spec"`
}

type KubeService struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		SelfLink          string    `json:"selfLink"`
		UID               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			App  string `json:"app"`
			Name string `json:"name"`
		} `json:"labels"`
		Annotations struct {
			ServiceBetaKubernetesIoAwsLoadBalancerInternal string `json:"service.beta.kubernetes.io/aws-load-balancer-internal"`
		} `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Ports []struct {
			Name       string `json:"name,omitempty"`
			Protocol   string `json:"protocol"`
			Port       int    `json:"port"`
			TargetPort int    `json:"targetPort"`
			NodePort   int    `json:"nodePort"`
		} `json:"ports"`
		Selector struct {
			Name string `json:"name"`
		} `json:"selector"`
		ClusterIP       string `json:"clusterIP"`
		Type            string `json:"type"`
		SessionAffinity string `json:"sessionAffinity"`
	} `json:"spec"`
}

type ServiceCollectionspec struct {
	Items []Service `json:"items"`
}

type Deploymentspec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		RevisionHistoryLimit int `json:"revisionHistoryLimit"`
		Metadata             struct {
			Annotations struct {
				SidecarIstioIOStatus string `json:"sidecar.istio.io/status"`
			} `json:"annotations,ommitempty"`
		} `json:"metadata",omitempty`
		Replicas int `json:"replicas"`
		Strategy struct {
			Type          string `json:"type,omitempty"`
			RollingUpdate struct {
				MaxUnavailable int `json:"maxUnavailable"`
				MaxSurge       int `json:"maxSurge,omitempty"`
			} `json:"rollingUpdate"`
		} `json:"strategy"`
		Selector struct {
			MatchLabels struct {
				Name string `json:"name"`
			} `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Name   string `json:"name"`
				Labels struct {
					Name string `json:"name"`
					App  string `json:"app,omitempty"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers       []ContainerItem    `json:"containers"`
				ImagePullPolicy  string             `json:"imagePullPolicy"`
				ImagePullSecrets []structs.Namespec `json:"imagePullSecrets"`
				DnsPolicy        string             `json:"dnsPolicy,omitempty"`
				InitContainers   *[]ContainerItem   `json:"initContainers,omitempty"`
				Volumes          *[]structs.Volumes `json:"volumes,omitempty"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type DeploymentCollectionspec struct {
	Items []Deploymentspec `json:"items"`
}

type ReplicationController struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
		Selector struct {
			Name string `json:"name"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Name   string `json:"name"`
				Labels struct {
					Name string `json:"name"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers      []ContainerItem `json:"containers"`
				ImagePullPolicy string          `json:"imagePullPolicy"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type OneOffPod struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name   string `json:"name"`
		Labels struct {
			Name  string `json:"name"`
			Space string `json:"space"`
		} `json:"labels"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Containers                    []ContainerItem    `json:"containers"`
		ImagePullPolicy               string             `json:"imagePullPolicy,omitempty"`
		ImagePullSecrets              []structs.Namespec `json:"imagePullSecrets"`
		RestartPolicy                 string             `json:"restartPolicy"`
		TerminationGracePeriodSeconds int                `json:"terminationGracePeriodSeconds"`
		DnsPolicy                     string             `json:"dnsPolicy,omitempty"`
	} `json:"spec"`
}

type CronJob struct {
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
		JobTemplate             Job    `json:"jobTemplate"`
	} `json:"spec"`
}

type Job struct {
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
		Template              struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Labels    struct {
					Name  string `json:"name"`
					Space string `json:"space"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers       []ContainerItem    `json:"containers"`
				NodeSelector     string             `json:"nodeSelector,omitempty"`
				ImagePullSecrets []structs.Namespec `json:"imagePullSecrets"`
				RestartPolicy    string             `json:"restartPolicy"`
				DnsPolicy        string             `json:"dnsPolicy,omitempty"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type PodStatusspec struct {
	HostIP     string `json:"hostIP"`
	Phase      string `json:"phase"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	Conditions []struct {
		Type               string      `json:"type"`
		Status             string      `json:"status"`
		LastProbeTime      interface{} `json:"lastProbeTime"`
		LastTransitionTime time.Time   `json:"lastTransitionTime"`
	} `json:"conditions"`
	StartTime         time.Time `json:"startTime"`
	ContainerStatuses []struct {
		Name      string                 `json:"name"`
		State     map[string]interface{} `json:"state"`
		LastState struct {
		} `json:"lastState"`
		Ready        bool `json:"ready"`
		RestartCount int  `json:"restartCount"`
	} `json:"containerStatuses"`
}

type PodStatusItems struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status PodStatusspec `json:"status"`
}

type PodStatus struct {
	Items []PodStatusItems `json:"items"`
}

type ReplicaSetList struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink        string `json:"selfLink"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []ReplicaSetSpec `json:"items"`
}

type ReplicaSetSpec struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		SelfLink          string    `json:"selfLink"`
		UID               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		Generation        int       `json:"generation"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			Name            string `json:"name"`
			PodTemplateHash string `json:"pod-template-hash"`
		} `json:"labels"`
		Annotations struct {
			DeploymentKubernetesIoRevision string `json:"deployment.kubernetes.io/revision"`
		} `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
		Selector struct {
			MatchLabels struct {
				Name            string `json:"name"`
				PodTemplateHash string `json:"pod-template-hash"`
			} `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Name              string      `json:"name"`
				CreationTimestamp interface{} `json:"creationTimestamp"`
				Labels            struct {
					Name            string `json:"name"`
					PodTemplateHash string `json:"pod-template-hash"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
					Ports []struct {
						ContainerPort int    `json:"containerPort"`
						Protocol      string `json:"protocol"`
					} `json:"ports"`
					Env []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"env"`
					Resources struct {
					} `json:"resources"`
					TerminationMessagePath string `json:"terminationMessagePath"`
					ImagePullPolicy        string `json:"imagePullPolicy"`
				} `json:"containers"`
				RestartPolicy                 string `json:"restartPolicy"`
				TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
				DNSPolicy                     string `json:"dnsPolicy"`
				SecurityContext               struct {
				} `json:"securityContext"`
				ImagePullSecrets []struct {
					Name string `json:"name"`
				} `json:"imagePullSecrets"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Replicas           int `json:"replicas"`
		ObservedGeneration int `json:"observedGeneration"`
	} `json:"status"`
}

type Revisionspec struct {
	Revision int `json:"revision"`
}

type Rollbackspec struct {
	ApiVersion         string            `json:"apiVersion"`
	Name               string            `json:"name"`
	UpdatedAnnotations map[string]string `json:"updatedAnnotations,omitempty"`
	RollbackTo         Revisionspec      `json:"rollbackTo"`
}

type JobScaleGet struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		SelfLink          string    `json:"selfLink"`
		UID               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			Name  string `json:"name"`
			Space string `json:"space"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Parallelism           int `json:"parallelism"`
		Completions           int `json:"completions"`
		ActiveDeadlineSeconds int `json:"activeDeadlineSeconds"`
		Selector              struct {
			MatchLabels struct {
				ControllerUID string `json:"controller-uid"`
			} `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Name              string      `json:"name"`
				Namespace         string      `json:"namespace"`
				CreationTimestamp interface{} `json:"creationTimestamp"`
				Labels            struct {
					ControllerUID string `json:"controller-uid"`
					JobName       string `json:"job-name"`
					Name          string `json:"name"`
					Space         string `json:"space"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
					Env   []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"env"`
					TerminationMessagePath   string `json:"terminationMessagePath"`
					TerminationMessagePolicy string `json:"terminationMessagePolicy"`
					ImagePullPolicy          string `json:"imagePullPolicy"`
				} `json:"containers"`
				RestartPolicy                 string `json:"restartPolicy"`
				TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
				DNSPolicy                     string `json:"dnsPolicy"`
				SecurityContext               struct {
				} `json:"securityContext"`
				ImagePullSecrets []struct {
					Name string `json:"name"`
				} `json:"imagePullSecrets"`
				SchedulerName string `json:"schedulerName"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type Runtime interface {
	Scale(space string, app string, amount int) (e error)
	GetService(space string, app string) (service KubeService, e error)
	CreateService(space string, app string, port int) (c *Createspec, e error)
	UpdateService(space string, app string, port int) (c *Createspec, e error)
	DeleteService(space string, app string) (e error)
	GetServices() (*ServiceCollectionspec, error)
	CreateDeployment(deployment *structs.Deployment) (err error)
	UpdateDeployment(deployment *structs.Deployment) (err error)
	GetDeployments() (*DeploymentCollectionspec, error)
	GetDeployment(space string, app string) (deployment *Deploymentspec, e error)
	DeleteDeployment(space string, app string) (e error)
	DeploymentExists(space string, app string) (exists bool)
	GetReplicas(space string, app string) (rs []string, e error)
	DeleteReplica(space string, app string, replica string) (e error)
	CreateOneOffPod(deployment *structs.Deployment) (e error)
	DeletePod(space string, pod string) (e error)
	DeletePods(space string, label string) (e error)
	GetPods(space string, app string) (rs []string, e error)
	CreateSpace(name string, compliance string) (e error)
	DeleteSpace(name string) (e error)
	CreateSecret(space string, name string, data string, mimetype string) (s *Secretspec, e error)
	AddImagePullSecretToSpace(space string) (e error)
	UpdateSpaceTags(space string, compliance string) (e error)
	RestartDeployment(space string, app string) (e error)
	GetCurrentImage(space string, app string) (i string, e error)
	GetPodDetails(space string, app string) []structs.Instance
	GetPodLogs(app string, space string, pod string) (log string, err error)
	OneOffExists(space string, name string) bool
	GetDeploymentHistory(space string, app string) (dslist []structs.DeploymentsSpec, err error)
	RollbackDeployment(space string, app string, revision int) (e error)
	GetPodStatus(space string, app string) []structs.SpaceAppStatus
	CronJobExists(space string, job string) bool
	GetCronJob(space string, jobName string) (*structs.CronJobStatus, error)
	GetCronJobs(space string) (sjobs []structs.CronJobStatus, e error)
	CreateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error)
	UpdateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error)
	DeleteCronJob(space string, jobName string) (e error)
	DeleteJob(space string, jobName string) (e error)
	GetJob(space string, jobName string) (*structs.JobStatus, error)
	GetJobs(space string) ([]structs.JobStatus, error)
	ScaleJob(space string, jobName string, replicas int, timeout int) (e error)
	JobExists(space string, jobName string) bool
	CreateJob(deployment *structs.Deployment) (*structs.JobStatus, error)
	GetPodsBySpace(space string) (*PodStatus, error)
	GetNodes() (*structs.KubeNodes, error)
}

var stackRuntimeCache map[string]Runtime = make(map[string]Runtime)
var stackToSpaceCache map[string]string = make(map[string]string)

func GetRuntimeStack(db *sql.DB, stack string) (rt Runtime, e error) {
	// Cache for the win!
	i, ok := stackRuntimeCache[stack]
	if ok {
		return i, nil
	}

	var (
		stackn            string
		description       string
		api_server        string
		api_version       string
		image_pull_secret string
		auth_type         string
		auth_vault_path   string
	)
	rows := db.QueryRow("select stacks.stack, stacks.description, stacks.api_server, stacks.api_version, stacks.image_pull_secret, stacks.auth_type, stacks.auth_vault_path from stacks where stacks.stack = ?", stack)
	err := rows.Scan(&stackn, &description, &api_server, &api_version, &image_pull_secret, &auth_type, &auth_vault_path)
	if err != nil {
		return nil, err
	}
	// If necessary we could flip on the type of runtime to use here:
	stackRuntimeCache[stack] = NewKubernetes(&KubernetesConfig{Name: stackn, APIServer: api_server, APIVersion: api_version, ImagePullSecret: image_pull_secret, AuthType: auth_type, AuthVaultPath: auth_vault_path})
	return stackRuntimeCache[stack], nil
}

func GetRuntimeFor(db *sql.DB, space string) (rt Runtime, e error) {
	// Cache for the win!
	j, ok := stackToSpaceCache[space]
	if ok {
		i, ok := stackRuntimeCache[j]
		if ok {
			return i, nil
		}
	}

	var (
		stack             string
		description       string
		api_server        string
		api_version       string
		image_pull_secret string
		auth_type         string
		auth_vault_path   string
	)
	rows := db.QueryRow("select stacks.stack, stacks.description, stacks.api_server, stacks.api_version, stacks.image_pull_secret, stacks.auth_type, stacks.auth_vault_path from stacks join spaces on spaces.stack = stacks.stack where spaces.name = $1", space)
	err := rows.Scan(&stack, &description, &api_server, &api_version, &image_pull_secret, &auth_type, &auth_vault_path)
	if err != nil {
		return nil, err
	}
	stackToSpaceCache[space] = stack
	// If necessary we could flip on the type of runtime to use here:
	stackRuntimeCache[stack] = NewKubernetes(&KubernetesConfig{Name: stack, APIServer: api_server, APIVersion: api_version, ImagePullSecret: image_pull_secret, AuthType: auth_type, AuthVaultPath: auth_vault_path})
	return stackRuntimeCache[stack], nil
}

func GetAllRuntimes(db *sql.DB) (rt []Runtime, e error) {
	rows, err := db.Query("select stacks.stack, stacks.description, stacks.api_server, stacks.api_version, stacks.image_pull_secret, stacks.auth_type, stacks.auth_vault_path from stacks")
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	runtimes := []Runtime{}
	for rows.Next() {
		var (
			stack             string
			description       string
			api_server        string
			api_version       string
			image_pull_secret string
			auth_type         string
			auth_vault_path   string
		)
		err := rows.Scan(&stack, &description, &api_server, &api_version, &image_pull_secret, &auth_type, &auth_vault_path)
		if err != nil {
			return nil, err
		}
		// If necessary we could flip on the type of runtime to use here:
		stackRuntimeCache[stack] = NewKubernetes(&KubernetesConfig{Name: stack, APIServer: api_server, APIVersion: api_version, ImagePullSecret: image_pull_secret, AuthType: auth_type, AuthVaultPath: auth_vault_path})
		runtimes = append(runtimes, stackRuntimeCache[stack])
	}

	return runtimes, nil
}
