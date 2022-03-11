package runtime

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	structs "region-api/structs"
	"strconv"
	"strings"
	"sync"
	"time"

	kube "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type KubeRequest struct {
	Status     string
	StatusCode int
	Body       []byte
}

type Kubernetes struct {
	Runtime
	clientType              string
	apiServer               string
	defaultApiServerVersion string
	clientToken             string
	imagePullSecret         string
	config                  *rest.Config
	client                  *http.Client
	mutex                   *sync.Mutex
	debug                   bool
}

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
	ImagePullSecrets []structs.Namespec `json:"imagePullSecrets,omitempty"`
}

type Namespacespec struct {
	Metadata struct {
		Name   string `json:"name"`
		Labels struct {
			Internal string `json:"akkeris.io/internal,omitempty"`
		} `json:"labels,omitempty"`
		Annotations struct {
			ComplianceTags string `json:"akkeris.io/compliancetags,omitempty"`
		}
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
	TCPSocket      *TcpCheck  `json:"tcpSocket,omitempty"`
	HTTPGET        *HttpCheck `json:"httpGet,omitempty"`
	TimeoutSeconds int        `json:"timeoutSeconds,omitempty"`
	PeriodSeconds  int        `json:"periodSeconds,omitEmpty"`
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
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
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
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		SelfLink          string            `json:"selfLink"`
		UID               string            `json:"uid"`
		ResourceVersion   string            `json:"resourceVersion"`
		CreationTimestamp time.Time         `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       struct {
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

type NodeSelector struct {
	PlanType string `json:"akkeris.io/plan-type,omitempty"`
}

type Tolerations struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Effect   string `json:"effect"`
	Value    string `json:"value"`
}

type Deploymentspec struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels,omitempty"`
	} `json:"metadata"`
	Spec struct {
		RevisionHistoryLimit int `json:"revisionHistoryLimit"`
		Metadata             struct {
			Annotations struct {
				SidecarIstioIOStatus string `json:"sidecar.istio.io/status"`
			} `json:"annotations,omitempty"`
		} `json:"metadata",omitempty`
		Replicas int `json:"replicas"`
		Strategy struct {
			Type          string `json:"type,omitempty"`
			RollingUpdate struct {
				MaxUnavailable interface{} `json:"maxUnavailable"`
				MaxSurge       interface{} `json:"maxSurge,omitempty"`
			} `json:"rollingUpdate"`
		} `json:"strategy"`
		Selector struct {
			MatchLabels struct {
				Name    string `json:"name"`
				App     string `json:"app,omitempty"`
				Version string `json:"version,omitempty"`
			} `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Name        string            `json:"name"`
				Labels      map[string]string `json:"labels,omitempty"`
				Annotations struct {
					SidecarIstioInject   string `json:"sidecar.istio.io/inject,omitempty"`
					AkkerisIORestartTime string `json:"akkeris.io/restartTime,omitempty"`
				} `json:"annotations,omitempty"`
			} `json:"metadata"`
			Spec struct {
				NodeSelector                  *NodeSelector      `json:"nodeSelector,omitempty"`
				Tolerations                   *[]Tolerations     `json:"tolerations,omitempty"`
				Containers                    []ContainerItem    `json:"containers"`
				ImagePullPolicy               string             `json:"imagePullPolicy,omitempty"`
				ImagePullSecrets              []structs.Namespec `json:"imagePullSecrets,omitempty"`
				DnsPolicy                     string             `json:"dnsPolicy,omitempty"`
				InitContainers                *[]ContainerItem   `json:"initContainers,omitempty"`
				Volumes                       *[]structs.Volumes `json:"volumes,omitempty"`
				TerminationGracePeriodSeconds int                `json:"terminationGracePeriodSeconds,omitempty"`
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
				ImagePullPolicy string          `json:"imagePullPolicy,omitempty"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type OneOffPod struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations struct {
			LogtrainDrainEndpoint string `json:"logtrain.akkeris.io/drains"`
		} `json:"annotations,omitempty"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Containers                    []ContainerItem    `json:"containers"`
		ImagePullPolicy               string             `json:"imagePullPolicy,omitempty"`
		ImagePullSecrets              []structs.Namespec `json:"imagePullSecrets,omitempty"`
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
				ImagePullSecrets []structs.Namespec `json:"imagePullSecrets,omitempty"`
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
					ImagePullPolicy        string `json:"imagePullPolicy,omitempty"`
				} `json:"containers"`
				RestartPolicy                 string `json:"restartPolicy"`
				TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
				DNSPolicy                     string `json:"dnsPolicy"`
				SecurityContext               struct {
				} `json:"securityContext"`
				ImagePullSecrets []struct {
					Name string `json:"name"`
				} `json:"imagePullSecrets,omitempty"`
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
					ImagePullPolicy          string `json:"imagePullPolicy,omitempty"`
				} `json:"containers"`
				RestartPolicy                 string `json:"restartPolicy"`
				TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
				DNSPolicy                     string `json:"dnsPolicy"`
				SecurityContext               struct {
				} `json:"securityContext"`
				ImagePullSecrets []struct {
					Name string `json:"name"`
				} `json:"imagePullSecrets,omitempty"`
				SchedulerName string `json:"schedulerName"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func (rt Kubernetes) AssembleImagePullSecrets(imagePullSecrets []structs.Namespec) []structs.Namespec {
	if os.Getenv("IMAGE_PULL_SECRET") != "" {
		ipss := strings.Split(os.Getenv("IMAGE_PULL_SECRET"), ",")
		for _, n := range ipss {
			ips := structs.Namespec{
				Name: n,
			}
			imagePullSecrets = append(imagePullSecrets, ips)
		}
	}
	return imagePullSecrets
}

func NewKubernetes(name string, imagePullSecret string) (r Runtime) {
	// Check if we have a kubeconfig path.
	var kubeconfig *string
	home := homeDir()
	defaultKubeConfig := filepath.Join(home, ".kube", "config")
	kubeconfig = flag.String("kubeconfig", defaultKubeConfig, "absolute path to the kubeconfig file")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	var rt Kubernetes
	if strings.HasPrefix(config.Host, "https://") {
		uri, err := url.Parse(config.Host)
		if err != nil {
			panic(err)
		}
		rt.apiServer = uri.Hostname()
		if uri.Port() != "" {
			rt.apiServer = rt.apiServer + ":" + uri.Port()
		}
		rt.apiServer = rt.apiServer + uri.Path
	} else {
		rt.apiServer = config.Host
	}

	var tlsConfig *tls.Config = &tls.Config{}
	if config.TLSClientConfig.CAData != nil && len(config.TLSClientConfig.CAData) != 0 {
		certs := x509.NewCertPool()
		if ok := certs.AppendCertsFromPEM([]byte(config.TLSClientConfig.CAData)); ok {
			tlsConfig.RootCAs = certs
			tlsConfig.BuildNameToCertificate()
		}
	}

	if config.TLSClientConfig.CertData != nil && len(config.TLSClientConfig.CertData) != 0 {
		rt.clientType = "mtls"
		cert, err := tls.X509KeyPair(config.TLSClientConfig.CertData, config.TLSClientConfig.KeyData)
		if err != nil {
			panic(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.BuildNameToCertificate()
	} else {
		rt.clientType = "token"
		rt.clientToken = config.BearerToken
	}

	fmt.Printf("Connecting to kubernetes cluster at %s\n", config.Host)
	rt.client = &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	rt.config = config
	rt.defaultApiServerVersion = "v1"
	rt.imagePullSecret = imagePullSecret
	rt.mutex = &sync.Mutex{}
	rt.debug = os.Getenv("DEBUG_K8S") == "true"
	var rts Runtime = Runtime(rt)
	return rts
}

func (rt Kubernetes) k8sRequest(method string, path string, payload interface{}) (r *KubeRequest, e error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(strings.ToUpper(method), "https://"+rt.apiServer+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if rt.clientType == "token" {
		req.Header.Add("Authorization", "Bearer "+rt.clientToken)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-type", "application/json")

	if rt.debug {
		log.Printf("-> k8: %s %s with headers [%s] with payload [%s]\n", method, "https://"+rt.apiServer+path, req.Header, body)
	}
	rt.mutex.Lock()
	resp, err := rt.client.Do(req)
	rt.mutex.Unlock()
	if err != nil {
		if rt.debug {
			log.Printf("<- k8 ERROR: %s %s - %s\n", method, "https://"+rt.apiServer+path, err)
		}
		return nil, err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	if rt.debug {
		log.Printf("<- k8: %s %s - %s\n", method, "https://"+rt.apiServer+path, resp.Status)
	}
	return &KubeRequest{Body: respBody, Status: resp.Status, StatusCode: resp.StatusCode}, nil
}

// This is external to provide functionality for specific types of systems (istio for example)
// can make requests, rather than using k8s we use this as its more generic
func (rt Kubernetes) GenericRequest(method string, path string, payload interface{}) ([]byte, int, error) {
	req, err := rt.k8sRequest(method, path, payload)
	if err != nil {
		return []byte{}, 0, err
	}
	return req.Body, req.StatusCode, nil
}

func (rt Kubernetes) Scale(space string, app string, amount int) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to scale app, space is blank.")
	}
	if app == "" {
		return errors.New("FATAL ERROR: Unable to scale app, the app is blank.")
	}
	if amount < 0 {
		return errors.New("FATAL ERROR: Unable to scale app, amount is not a whole positive number.")
	}
	_, err := rt.k8sRequest("put", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app+"/scale",
		Scalespec{Metadata: Metadataspec{Name: app, Namespace: space}, Spec: Specspec{Replicas: amount}})
	if err != nil {
		return err
	}
	return nil
}

func deploymentToDeploymentSpec(deployment *structs.Deployment) (dp Deploymentspec) {
	var c1 ContainerItem
	// assign environment variables
	c1.Env = deployment.ConfigVars
	clist := []ContainerItem{}

	// assemble readiness probe
	var probe ReadinessProbe
	if deployment.Port != -1 {
		var cp1 ContainerPort
		if deployment.HealthCheck != "tcp" {
			probe.HTTPGET = &HttpCheck{Port: deployment.Port, Path: deployment.HealthCheck}
		} else {
			probe.TCPSocket = &TcpCheck{Port: deployment.Port}
		}
		cp1.ContainerPort = deployment.Port
		probe.PeriodSeconds = 20
		probe.TimeoutSeconds = 15
		c1.ReadinessProbe = &probe
		cportlist := []ContainerPort{}
		c1.Ports = append(cportlist, cp1)
	}

	// add additional ports (Akkeris beta feature)
	if deployment.ContainerPorts != nil && len(deployment.ContainerPorts) > 0 {
		for _, containerPort := range deployment.ContainerPorts {
			c1.Ports = append(c1.Ports, ContainerPort{ContainerPort: containerPort})
		}
	}

	// assemble image
	c1.Name = deployment.App
	c1.Image = deployment.Image + ":" + deployment.Tag
	if len(deployment.Command) > 0 {
		c1.Command = deployment.Command
	}
	c1.ImagePullPolicy = "IfNotPresent"

	// assemble memory constraints
	var resources structs.ResourceSpec
	resources.Requests.Memory = deployment.MemoryRequest
	resources.Limits.Memory = deployment.MemoryLimit
	c1.Resources = resources

	// assemble secrets
	c1.ImagePullSecrets = deployment.Secrets

	clist = append(clist, c1)

	var krc Deploymentspec

	krc.Metadata.Name = deployment.App
	krc.Metadata.Namespace = deployment.Space
	krc.Metadata.Labels = deployment.Labels
	krc.Spec.Replicas = deployment.Amount
	krc.Spec.RevisionHistoryLimit = deployment.RevisionHistoryLimit
	krc.Spec.Selector.MatchLabels.Name = deployment.App
	krc.Spec.Template.Metadata.Name = deployment.App

	// copy labels to pod spec, add name app and version as well.
	krc.Spec.Template.Metadata.Labels = make(map[string]string)
	for key, val := range deployment.Labels {
		krc.Spec.Template.Metadata.Labels[key] = val
	}
	krc.Spec.Template.Metadata.Labels["name"] = deployment.App // unsure what this is used for.
	krc.Spec.Template.Metadata.Labels["app"] = deployment.App  // unsure what this is used for.
	krc.Spec.Template.Metadata.Labels["version"] = "v1"        // unsure what this is used for.

	if os.Getenv("FF_ISTIOINJECT") == "true" || deployment.Features.IstioInject || deployment.Features.ServiceMesh {
		krc.Spec.Template.Metadata.Labels["sidecar.istio.io/inject"] = "true"
		krc.Spec.Template.Metadata.Annotations.SidecarIstioInject = "true"
	} else {
		krc.Spec.Template.Metadata.Labels["sidecar.istio.io/inject"] = "false"
		krc.Spec.Template.Metadata.Annotations.SidecarIstioInject = "false"

	}

	krc.Spec.Strategy.RollingUpdate.MaxUnavailable = 0
	krc.Spec.Template.Spec.ImagePullSecrets = deployment.Secrets
	krc.Spec.Template.Spec.Containers = clist
	krc.Spec.Template.Spec.ImagePullPolicy = "IfNotPresent"
	krc.Spec.Template.Spec.TerminationGracePeriodSeconds = 60
	if deployment.PlanType != "" && deployment.PlanType != "general" {
		krc.Spec.Template.Spec.NodeSelector = &NodeSelector{
			PlanType: deployment.PlanType,
		}
		t := make([]Tolerations, 0)
		t = append(t, Tolerations{
			Key:      "akkeris.io/plan-type",
			Operator: "Equal",
			Value:    deployment.PlanType,
			Effect:   "NoSchedule",
		})
		krc.Spec.Template.Spec.Tolerations = &t
	}
	return krc
}

func (rt Kubernetes) UpdateDeployment(deployment *structs.Deployment) (err error) {
	if deployment.Space == "" {
		return errors.New("FATAL ERROR: Unable to update deployment, space is blank.")
	}
	if deployment.App == "" {
		return errors.New("FATAL ERROR: Unable to update deployment, the app is blank.")
	}

	// Assemble secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	resp, err := rt.k8sRequest("PUT", "/apis/apps/v1/namespaces/"+deployment.Space+"/deployments/"+deployment.App,
		deploymentToDeploymentSpec(deployment))
	if err != nil {
		return err
	}
	if resp.StatusCode > 399 || resp.StatusCode < 200 {
		return errors.New("Cannot update deployment for " + deployment.App + "-" + deployment.Space + " received: " + resp.Status + " " + string(resp.Body))
	}
	return nil
}

func (rt Kubernetes) CreateDeployment(deployment *structs.Deployment) (err error) {

	// Assemble secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	resp, err := rt.k8sRequest("POST", "/apis/apps/v1/namespaces/"+deployment.Space+"/deployments", deploymentToDeploymentSpec(deployment))
	if err != nil {
		return err
	}
	if resp.StatusCode > 399 || resp.StatusCode < http.StatusOK {
		return errors.New("Cannot create deployment for " + deployment.App + "-" + deployment.Space + " received: " + resp.Status + " " + string(resp.Body))
	}
	return nil
}

func (rt Kubernetes) getDeployment(space string, app string) (*Deploymentspec, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get deployment, space is blank.")
	}
	if app == "" {
		return nil, errors.New("FATAL ERROR: Unable to get deployment, the app is blank.")
	}
	resp, e := rt.k8sRequest("get", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, nil)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("deployment not found")
	}
	var deployment Deploymentspec
	e = json.Unmarshal(resp.Body, &deployment)
	if e != nil {
		return nil, e
	}
	return &deployment, nil
}

func (rt Kubernetes) DeploymentExists(space string, app string) (bool, error) {
	if space == "" {
		return false, errors.New("The space name was blank.")
	}
	if app == "" {
		return false, errors.New("The app name was blank.")
	}
	resp, e := rt.k8sRequest("get", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, nil)
	if e != nil {
		return false, e
	}
	if resp.StatusCode == 404 {
		return false, nil
	}
	if resp.StatusCode > 199 && resp.StatusCode < 300 {
		return true, nil
	}
	if resp.StatusCode > 399 || resp.StatusCode < 200 {
		return false, errors.New("Cannot determine if deployment exists for " + app + "-" + space + " received: " + resp.Status + " " + string(resp.Body))
	}
	return false, nil
}

func (rt Kubernetes) DeleteDeployment(space string, app string) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to remove deployment, space is blank.")
	}
	if app == "" {
		return errors.New("FATAL ERROR: Unable to remove deployment, the app is blank.")
	}
	_, e = rt.k8sRequest("delete", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, nil)
	return e
}

func (rt Kubernetes) RestartDeployment(space string, app string) (e error) {
	deployment, e := rt.getDeployment(space, app)
	if e != nil {
		if e.Error() == "deployment not found" {
			// Sometimes a restart can get issued for an app that has yet to be deployed,
			// just ignore the error and do nothing.
			return nil
		}
		return e
	}

	currentTime := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	deployment.Spec.Template.Metadata.Annotations.AkkerisIORestartTime = currentTime

	_, e = rt.k8sRequest("put", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, deployment)
	return e
}

func (rt Kubernetes) GetDeploymentHistory(space string, app string) (dslist []structs.DeploymentsSpec, e error) {
	resp, err := rt.k8sRequest("get", "/apis/apps/v1/namespaces/"+space+"/replicasets?labelSelector=name="+app, nil)
	if err != nil {
		log.Println(err)
	}
	var rsl ReplicaSetList
	e = json.Unmarshal(resp.Body, &rsl)
	if e != nil {
		return nil, e
	}
	for _, element := range rsl.Items {
		var ds structs.DeploymentsSpec
		ds.Name = element.Metadata.Name
		ds.Space = element.Metadata.Namespace
		ds.CreationTimestamp = element.Metadata.CreationTimestamp
		ds.Image = element.Spec.Template.Spec.Containers[0].Image
		ds.Revision = element.Metadata.Annotations.DeploymentKubernetesIoRevision
		dslist = append(dslist, ds)
	}
	return dslist, nil
}

func (rt Kubernetes) RollbackDeployment(space string, app string, revision int) (e error) {
	var rollbackspec Rollbackspec = Rollbackspec{ApiVersion: "apps/v1", Name: app, RollbackTo: Revisionspec{Revision: revision}}
	rollbackspec.RollbackTo.Revision = revision
	_, e = rt.k8sRequest("post", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app+"/rollback", rollbackspec)
	if e != nil {
		return e
	}
	return nil
}

func (rt Kubernetes) GetReplicas(space string, app string) (rs []string, e error) {
	resp, err := rt.k8sRequest("get", "/apis/apps/v1/namespaces/"+space+"/replicasets?labelSelector=name="+app, nil)
	if err != nil {
		return nil, err
	}
	var response Itemsspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	for _, item := range response.Items {
		rs = append(rs, item.Metadata.Name)
	}
	return rs, nil
}

func (rt Kubernetes) DeleteReplica(space string, app string, replica string) (e error) {
	if space == "" {
		return errors.New("Unable to delete replica, space is blank.")
	}
	if replica == "" {
		return errors.New("Unable to delete replica, the replica is blank.")
	}
	_, e = rt.k8sRequest("delete", "/apis/apps/v1/namespaces/"+space+"/replicasets/"+replica, nil)
	return e
}

func (rt Kubernetes) GetPods(space string, app string) (rs []string, e error) {
	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods?labelSelector=name="+app, nil)
	if err != nil {
		return nil, err
	}
	var response Itemsspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	for _, item := range response.Items {
		rs = append(rs, item.Metadata.Name)
	}
	return rs, nil
}

func (rt Kubernetes) OneOffExists(space string, name string) bool {
	if space == "" {
		return false
	}
	if name == "" {
		return false
	}
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods/"+name, nil)
	if e != nil {
		log.Println(e)
		return false
	}
	if resp.StatusCode == 200 {
		return true
	} else {
		return false
	}
}

func (rt Kubernetes) CreateOneOffPod(deployment *structs.Deployment) (e error) {
	if deployment.Space == "" {
		return errors.New("FATAL ERROR: Unable to create one off deployment, space is blank.")
	}
	if deployment.App == "" {
		return errors.New("FATAL ERROR: Unable to create one off deployment, the app is blank.")
	}
	// Assemble secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	var koneoff OneOffPod
	koneoff.Metadata.Name = deployment.App
	koneoff.Metadata.Namespace = deployment.Space
	koneoff.APIVersion = "v1"
	koneoff.Kind = "Pod"
	koneoff.Spec.RestartPolicy = "Never"
	koneoff.Spec.ImagePullPolicy = "Always"
	koneoff.Spec.DnsPolicy = "Default"

	koneoff.Metadata.Labels = deployment.Labels
	koneoff.Metadata.Labels["Name"] = deployment.App
	koneoff.Metadata.Labels["Space"] = deployment.Space

	if deployment.Annotations != nil && deployment.Annotations["logtrain.akkeris.io/drains"] != "" {
		koneoff.Metadata.Annotations.LogtrainDrainEndpoint = deployment.Annotations["logtrain.akkeris.io/drains"]
	}

	var container ContainerItem
	container.ImagePullPolicy = "Always"
	container.Name = deployment.App
	container.Image = deployment.Image + ":" + deployment.Tag
	container.ImagePullSecrets = deployment.Secrets
	container.Env = deployment.ConfigVars

	if len(deployment.Command) > 0 {
		container.Command = deployment.Command
	}

	var resources structs.ResourceSpec
	resources.Requests.Memory = deployment.MemoryRequest
	resources.Limits.Memory = deployment.MemoryLimit
	container.Resources = resources

	clist := []ContainerItem{}
	clist = append(clist, container)

	koneoff.Spec.ImagePullSecrets = deployment.Secrets
	koneoff.Spec.Containers = clist

	_, e = rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+deployment.Space+"/pods", koneoff)
	return e
}

func (rt Kubernetes) DeletePod(space string, pod string) (e error) {
	if space == "" {
		return errors.New("Unable to delete pod, space is blank.")
	}
	if pod == "" {
		return errors.New("Unable to delete pod is blank.")
	}
	resp, e := rt.k8sRequest("delete", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods/"+pod, nil)
	if resp.StatusCode > 199 && resp.StatusCode < 300 || resp.StatusCode == 404 {
		return nil
	} else {
		return fmt.Errorf("Invalid response from kubernetes: %d Status Code %s", resp.StatusCode, resp.Body)
	}
	return e
}

func (rt Kubernetes) DeletePods(space string, label string) (e error) {
	if space == "" {
		return errors.New("Unable to delete pod, space is blank.")
	}
	if label == "" {
		return errors.New("Unable to delete pods, the label is blank.")
	}
	resp, e := rt.k8sRequest("delete", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods?labelSelector=name="+label, nil)
	if e != nil {
		return e
	}
	if resp.StatusCode > 199 && resp.StatusCode < 300 || resp.StatusCode == 404 {
		return nil
	} else {
		return fmt.Errorf("Invalid response from kubernetes: %d Status Code %s", resp.StatusCode, resp.Body)
	}
}

func (rt Kubernetes) CreateSpace(name string, internal bool, compliance string) (e error) {
	if name == "" {
		return errors.New("FATAL ERROR: Unable to create space, space is blank.")
	}
	var namespace Namespacespec
	namespace.Metadata.Name = name
	if len(compliance) > 0 {
		namespace.Metadata.Annotations.ComplianceTags = compliance
	}
	namespace.Metadata.Labels.Internal = "false"
	if internal {
		namespace.Metadata.Labels.Internal = "true"
	}
	resp, e := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces", namespace)

	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errors.New("Unable to create space, invalid response code from kubernetes: " + strconv.Itoa(resp.StatusCode))
	}
	return e
}

func (rt Kubernetes) DeleteSpace(name string) (e error) {
	if name == "" || name == "kube-public" || name == "kube-system" {
		return errors.New("Unable to delete space, the name was invalid.")
	}
	resp, e := rt.k8sRequest("delete", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+name, nil)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return errors.New("Unable to delete space, invalid response code from kubernetes: " + resp.Status)
	}
	return nil
}

func (rt Kubernetes) UpdateSpaceTags(space string, compliance string) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to update space tags, space is blank.")
	}
	var namespace Namespacespec
	namespace.Metadata.Name = space
	namespace.Metadata.Annotations.ComplianceTags = compliance
	_, e = rt.k8sRequest("patch", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space, namespace)
	return e
}

func (rt Kubernetes) CreateSecret(space string, name string, data string, mimetype string) (*Secretspec, error) {
	var secret Secretspec
	secret.Metadata.Name = name
	secret.Data.Dockercfg = data
	secret.Type = mimetype
	resp, err := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/secrets", secret)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, errors.New("Unable to create secret, invalid response code from kubernetes: " + resp.Status)
	}
	return &secret, nil
}

func (rt Kubernetes) CopySecret(imagePullSecret structs.Namespec, fromNamespace string, toNamespace string) error {
	if fromNamespace == "" {
		return errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if toNamespace == "" {
		return errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}

	var secret kube.Secret

	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+fromNamespace+"/secrets/"+imagePullSecret.Name, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Unable to copy secret, invalid response code from kubernetes on fetch: " + resp.Status)
	}

	if err := json.Unmarshal(resp.Body, &secret); err != nil {
		return err
	}

	secret.SetResourceVersion("")
	secret.SetNamespace(toNamespace)

	resp, err = rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+toNamespace+"/secrets", secret)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return errors.New("Unable to create secret on copy, invalid response code from kubernetes: " + resp.Status)
	}
	return nil
}

func (rt Kubernetes) AddImagePullSecretToSpace(space string) (e error) {
	var sa Serviceaccountspec = Serviceaccountspec{Metadata: structs.Namespec{Name: "default"}}

	sa.ImagePullSecrets = rt.AssembleImagePullSecrets(sa.ImagePullSecrets)
	resp, err := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/serviceaccounts/default", sa)
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return errors.New("Unable to add secret to space, invalid response code from kubernetes: " + resp.Status)
	}
	return err
}

func (rt Kubernetes) GetCurrentImage(space string, app string) (i string, e error) {
	var image string
	deployment, err := rt.getDeployment(space, app)
	if err != nil {
		return "", err
	}
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	} else {
		image = "None"
	}
	return image, nil
}

func (rt Kubernetes) GetPodStatus(space string, app string) []structs.SpaceAppStatus {
	resp, err := rt.k8sRequest("GET", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods?labelSelector=name="+app, nil)
	if err != nil {
		log.Println("Cannot get pod from kuberenetes:")
		log.Println(err)
	}
	var podstatus PodStatus
	err = json.Unmarshal(resp.Body, &podstatus)
	if err != nil {
		log.Println("Failed to decode pod details:")
		log.Println(err)
	}

	var statuses []structs.SpaceAppStatus
	for _, element := range podstatus.Items {
		var s structs.SpaceAppStatus
		s.App = app
		s.Space = space
		if element.Status.Phase == "Running" {
			s.Status = 0
			s.Output = element.Metadata.Name + "=" + element.Status.Phase
		}
		if element.Status.Phase != "Running" {
			s.Status = 2
			s.Output = element.Status.Phase
		}
		s.Reason = element.Status.Reason
		s.ExtendedOutput = element.Status.Message
		s.Ready = false
		if len(element.Status.ContainerStatuses) > 0 {
			s.Ready = element.Status.ContainerStatuses[0].Ready
			s.Restarted = element.Status.ContainerStatuses[0].RestartCount
			s.State = element.Status.ContainerStatuses[0].State
		}
		statuses = append(statuses, s)
	}
	return statuses
}

func (rt Kubernetes) GetPodDetails(space string, app string) []structs.Instance {
	var instances []structs.Instance

	if space == "" {
		log.Println("FATAL ERROR: Unable to get pod details, space is blank.")
		return instances
	}
	if app == "" {
		log.Println("FATAL ERROR: Unable to get pod details, the app is blank.")
		return instances
	}
	resp, err := rt.k8sRequest("GET", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods?labelSelector=name="+app, nil)
	if err != nil {
		log.Println("Cannot get pod from kubernetes:")
		log.Println(err)
	}
	var podstatus PodStatus
	err = json.Unmarshal(resp.Body, &podstatus)
	if err != nil {
		log.Println("Failed to decode pod details:")
		log.Println(err)
	}

	for _, element := range podstatus.Items {
		var instance structs.Instance
		instance.InstanceID = element.Metadata.Name
		instance.Phase = element.Status.Phase
		instance.StartTime = element.Status.StartTime
		var appstatus []structs.Appstatus
		for _, containerelement := range element.Status.ContainerStatuses {
			var as structs.Appstatus
			state := containerelement.State
			var keys []string
			for k := range state {
				keys = append(keys, k)
			}
			containerstate := keys[0]
			details := make(map[string]string)
			for _, v := range state {
				for k2, v2 := range v.(map[string]interface{}) {
					if v2 != nil {
						v2type := reflect.TypeOf(v2).String()
						if v2type == "string" {
							details[k2] = v2.(string)
						}
						if v2type == "float64" {
							details[k2] = strconv.FormatFloat(v2.(float64), 'f', -1, 64)
						}
					} else {
						details[k2] = ""
					}

				}
			}
			if containerstate == "waiting" {
				as.StartedAt = element.Status.StartTime
				instance.Reason = details["reason"]
			}
			if containerstate == "running" {
				instance.Reason = details["reason"]
				layout := "2006-01-02T15:04:05Z"
				str := details["startedAt"]
				t, err := time.Parse(layout, str)
				if err != nil {
					log.Println("Failed to parse startedAt time on GetPodDetails")
					log.Println(err)
				}
				as.StartedAt = t
			}
			if containerstate == "terminated" {
				instance.Reason = details["reason"]
				layout := "2006-01-02T15:04:05Z"
				str := details["finishedAt"]
				t, err := time.Parse(layout, str)
				if err != nil {
					log.Println("Failed to parse finishedAt time on GetPodDetails")
					log.Println(err)
				}
				as.StartedAt = t
			}

			instance.Phase = instance.Phase + "/" + containerstate
			as.App = containerelement.Name
			as.ReadyStatus = containerelement.Ready
			appstatus = append(appstatus, as)
		}
		instance.Appstatus = appstatus
		instances = append(instances, instance)
	}
	return instances
}

func (rt Kubernetes) GetPodLogs(space string, app string, pod string) (log string, err error) {
	limitBytes := "1000000" //100kb
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods/"+pod+"/log?limitBytes="+limitBytes+"&container="+app, nil)
	if e != nil {
		return "", e
	}
	body := string(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(string(body))
	}
	return string(body), nil
}

func (rt Kubernetes) Exec(space string, app string, instance string, command []string, stdin string) (*string, *string, error) {
	var commandQuery = ""
	for _, arg := range command {
		commandQuery += "command=" + url.QueryEscape(arg) + "&"
	}
	var path = "/api/" + rt.defaultApiServerVersion + "/namespaces/" + space + "/pods/" + instance + "/exec?" + commandQuery + "container=" + app + "&stdin=true&stdout=true&stderr=true&tty=false"
	uri, err := url.Parse("https://" + rt.apiServer + path)

	if rt.debug {
		log.Printf("-> k8 (stream): %s %s with command [%#+v]\n", "POST", "https://"+rt.apiServer+path, command)
	}
	log.Printf("-> k8 (stream): %s %s with command [%#+v] with stdin: \n%s\n", "POST", "https://"+rt.apiServer+path, command, stdin)
	if err != nil {
		return nil, nil, err
	}
	stream, err := remotecommand.NewSPDYExecutor(rt.config, "POST", uri)
	if err != nil {
		return nil, nil, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdinBuf := bytes.NewBufferString(stdin)

	timeout := make(chan error, 1)
	go func() {
		timeout <- stream.Stream(remotecommand.StreamOptions{
			Tty:    false,
			Stdin:  stdinBuf,
			Stdout: &stdout,
			Stderr: &stderr,
		})
	}()

	select {
	case <-time.After(60 * time.Second):
		return nil, nil, errors.New("The program failed to return in the specified time alloted.")
	case res := <-timeout:
		if rt.debug {
			log.Printf("<- k8: %s %s - %s\n", "POST", "https://"+rt.apiServer+path, command)
		}
		if res != nil {
			return nil, nil, err
		}
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		return &stdoutStr, &stderrStr, nil
	}
}

func (rt Kubernetes) GetService(space string, app string) (KubeService, error) {
	var response KubeService
	if space == "" {
		return response, errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if app == "" {
		return response, errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, nil)
	if e != nil {
		return response, e
	}
	if resp.StatusCode == http.StatusNotFound {
		return response, errors.New("service not found")
	}
	if resp.StatusCode != http.StatusOK {
		return response, errors.New("Unable to get service " + resp.Status)
	}
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return response, e
	}
	return response, nil
}

func (rt Kubernetes) ServiceExists(space string, app string) (bool, error) {
	if space == "" {
		return false, errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if app == "" {
		return false, errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, nil)
	if e != nil {
		return false, e
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode == http.StatusOK {
		return true, nil
	} else {
		return false, errors.New("Failed fetching status code: " + resp.Status)
	}
}

func (rt Kubernetes) InternalServiceExists(space string, app string) (bool, error) {
	if space == "" {
		return false, errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if app == "" {
		return false, errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app+"-cp", nil)
	if e != nil {
		return false, e
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode == http.StatusOK {
		return true, nil
	} else {
		return false, errors.New("Failed fetching status code: " + resp.Status)
	}
}

func (rt Kubernetes) CreateService(space string, app string, port int, labels map[string]string, features structs.Features) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to create service, space is blank.")
	}
	if app == "" {
		return errors.New("FATAL ERROR: Unable to create service, the app is blank.")
	}
	if port < 1 || port > 65535 {
		return errors.New("Invalid port range on CreateService: " + strconv.Itoa(port))
	}

	var service Service
	service.Kind = "Service"
	service.Metadata.Annotations.ServiceBetaKubernetesIoAwsLoadBalancerInternal = "0.0.0.0/0"
	service.Metadata.Name = app
	labels["app"] = app
	labels["name"] = app
	service.Metadata.Labels = labels
	service.Spec.Selector.Name = app

	var portitem PortItem
	portitem.Protocol = "TCP"
	portitem.Port = 80
	portitem.TargetPort = port
	if features.Http2EndToEndService {
		portitem.Name = "http2"
	} else {
		portitem.Name = "http"
	}
	portlist := []PortItem{}
	portlist = append(portlist, portitem)
	service.Spec.Ports = portlist
	service.Spec.Type = "NodePort"

	resp, e := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services", service)
	if e != nil {
		return e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return e
	}
	return nil
}

// Used for beta feature -- "container-ports"
// Creates a ClusterIP service "appname-cp"
func (rt Kubernetes) CreateInternalService(space string, app string, ports []int) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to create service, space is blank.")
	}
	if app == "" {
		return errors.New("FATAL ERROR: Unable to create service, the app is blank.")
	}

	var service Service
	service.Kind = "Service"
	service.Metadata.Name = app + "-cp"
	service.Metadata.Labels = map[string]string{"app": app, "name": app + "-cp", "akkeris.io/container-ports": "true"}
	service.Spec.Selector.Name = app
	service.Spec.Type = "ClusterIP"

	portlist := []PortItem{}
	for _, p := range ports {
		var portitem PortItem
		portitem.Protocol = "TCP"
		portitem.Port = p
		portitem.TargetPort = p
		portitem.Name = "cp-" + strconv.Itoa(p)
		portlist = append(portlist, portitem)
	}
	service.Spec.Ports = portlist

	resp, e := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services", service)
	if e != nil {
		return e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return e
	}
	return nil
}

func (rt Kubernetes) UpdateService(space string, app string, port int, labels map[string]string, features structs.Features) (e error) {
	if space == "" {
		return errors.New("Unable to update service, space is blank.")
	}
	if app == "" {
		return errors.New("Unable to update service, the app is blank.")
	}
	if port < 1 || port > 65535 {
		return errors.New("Invalid port range on UpdateService: " + strconv.Itoa(port))
	}
	existingservice, e := rt.GetService(space, app)
	if e != nil {
		return e
	}
	if existingservice.Spec.Ports != nil && len(existingservice.Spec.Ports) > 0 {
		if features.Http2EndToEndService {
			existingservice.Spec.Ports[0].Name = "http2"
		} else {
			existingservice.Spec.Ports[0].Name = "http"
		}
		existingservice.Spec.Ports[0].TargetPort = port
	}
	for k := range labels {
		existingservice.Metadata.Labels[k] = labels[k]
	}

	resp, e := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, existingservice)
	if e != nil {
		return e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return e
	}
	return nil
}

// Used for beta feature -- "container-ports"
// Updates a ClusterIP service "appname-cp"
func (rt Kubernetes) UpdateInternalService(space string, app string, ports []int) (e error) {
	if space == "" {
		return errors.New("Unable to update service, space is blank.")
	}
	if app == "" {
		return errors.New("Unable to update create service, the app is blank.")
	}

	existingservice, e := rt.GetService(space, app+"-cp")
	if e != nil {
		return e
	}

	// Replace all ports
	type Ports []struct {
		Name       string "json:\"name,omitempty\""
		Protocol   string "json:\"protocol\""
		Port       int    "json:\"port\""
		TargetPort int    "json:\"targetPort\""
		NodePort   int    "json:\"nodePort\""
	}
	portlist := Ports{}
	for _, p := range ports {
		var portitem PortItem
		portitem.Protocol = "TCP"
		portitem.Port = p
		portitem.TargetPort = p
		portitem.Name = "cp-" + strconv.Itoa(p)
		portlist = append(portlist, portitem)
	}
	existingservice.Spec.Ports = portlist

	resp, e := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app+"-cp", existingservice)
	if e != nil {
		return e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return e
	}
	return nil
}

func (rt Kubernetes) DeleteService(space string, app string) (e error) {
	if space == "" {
		return errors.New("Unable to delete service, space is blank.")
	}
	if app == "" {
		return errors.New("Unable to delete service, the app is blank.")
	}
	resp, err := rt.k8sRequest("delete", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, nil)
	if err != nil {
		return err
	}
	var response Statusspec
	err = json.Unmarshal(resp.Body, &response)
	if err != nil {
		return err
	}
	return nil
}

// Used for beta feature -- "container-ports"
// Deletes a ClusterIP service "appname-cp"
func (rt Kubernetes) DeleteInternalService(space string, app string) (e error) {
	if space == "" {
		return errors.New("Unable to delete service, space is blank.")
	}
	if app == "" {
		return errors.New("Unable to delete service, the app is blank.")
	}
	resp, err := rt.k8sRequest("delete", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app+"-cp", nil)
	if err != nil {
		return err
	}
	var response Statusspec
	err = json.Unmarshal(resp.Body, &response)
	if err != nil {
		return err
	}
	return nil
}

func (rt Kubernetes) GetServices() (*ServiceCollectionspec, error) {
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/services", nil)
	if e != nil {
		return nil, e
	}
	var services ServiceCollectionspec
	e = json.Unmarshal(resp.Body, &services)
	if e != nil {
		return nil, e
	}
	return &services, nil
}

func (rt Kubernetes) CronJobExists(space string, job string) bool {
	resp, err := rt.k8sRequest("get", "/apis/batch/v2alpha1/namespaces/"+space+"/cronjobs/"+job, nil)
	if err != nil {
		return false
	}
	if resp.StatusCode == 200 {
		return true
	}
	return false
}

func deploymentToCronJob(deployment *structs.Deployment) (cronJob *CronJob) {
	// Limits
	var resources structs.ResourceSpec
	resources.Requests.Memory = deployment.MemoryRequest
	resources.Limits.Memory = deployment.MemoryLimit

	// Image and config
	var container ContainerItem
	container.Name = deployment.App
	container.Image = deployment.Image
	container.ImagePullSecrets = deployment.Secrets
	container.Env = deployment.ConfigVars
	container.Resources = resources

	var clist []ContainerItem
	clist = append(clist, container)

	var job Job
	job.Metadata.Name = deployment.App
	job.Metadata.Namespace = deployment.Space
	job.Metadata.Labels.Name = deployment.App
	job.Metadata.Labels.Space = deployment.Space
	job.Spec.Template.Metadata.Name = deployment.App
	job.Spec.Template.Metadata.Namespace = deployment.Space
	job.Spec.Template.Metadata.Labels.Name = deployment.App
	job.Spec.Template.Metadata.Labels.Space = deployment.Space
	job.Spec.Template.Spec.ImagePullSecrets = deployment.Secrets
	job.Spec.Template.Spec.Containers = clist
	job.Spec.Template.Spec.RestartPolicy = "OnFailure"

	// Template for CronJob
	cronJob.Metadata.Name = deployment.App
	cronJob.Metadata.Namespace = deployment.Space
	cronJob.Metadata.Labels.Name = deployment.App
	cronJob.Metadata.Labels.Space = deployment.Space
	cronJob.Spec.Schedule = deployment.Schedule
	cronJob.Spec.StartingDeadlineSeconds = 60
	cronJob.Spec.JobTemplate = job

	return cronJob
}

func (rt Kubernetes) CreateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error) {
	if deployment.Space == "" {
		return nil, errors.New("FATAL ERROR: Unable to create cronjob, space is blank.")
	}

	// Assemble secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	// Update or Create Job
	resp, err := rt.k8sRequest("post", "/apis/batch/v2alpha1/namespaces/"+deployment.Space+"/cronjobs", deploymentToCronJob(deployment))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, errors.New("Unable to create cron job, received from kubernetes: " + resp.Status)
	}
	var status structs.CronJobStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (rt Kubernetes) UpdateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error) {
	if deployment.Space == "" {
		return nil, errors.New("FATAL ERROR: Unable to update cron job, space is blank.")
	}
	if deployment.App == "" {
		return nil, errors.New("FATAL ERROR: Unable to update cron job, the app is blank.")
	}

	// Assemble secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	resp, err := rt.k8sRequest("put", "/apis/batch/v2alpha1/namespaces/"+deployment.Space+"/cronjobs/"+deployment.App, deploymentToCronJob(deployment))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to update cron job, received from kubernetes: " + resp.Status)
	}
	var status structs.CronJobStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (rt Kubernetes) CreateJob(deployment *structs.Deployment) (*structs.JobStatus, error) {
	if deployment.Space == "" {
		return nil, errors.New("FATAL ERROR: Unable to create job, space is blank.")
	}
	if deployment.App == "" {
		return nil, errors.New("FATAL ERROR: Unable to create job, the job name is blank.")
	}

	// Assemble Secrets
	deployment.Secrets = rt.AssembleImagePullSecrets(deployment.Secrets)

	var resources structs.ResourceSpec
	resources.Requests.Memory = deployment.MemoryRequest
	resources.Limits.Memory = deployment.MemoryLimit

	var container ContainerItem
	container.Name = deployment.App
	container.Image = deployment.Image + ":" + deployment.Tag
	container.ImagePullPolicy = "Always"
	container.ImagePullSecrets = deployment.Secrets
	container.Env = deployment.ConfigVars
	container.Resources = resources

	var clist []ContainerItem
	clist = append(clist, container)

	var job Job
	job.Metadata.Name = deployment.App
	job.Metadata.Namespace = deployment.Space
	job.Metadata.Labels.Name = deployment.App
	job.Metadata.Labels.Space = deployment.Space
	job.Spec.Template.Metadata.Name = deployment.App
	job.Spec.Template.Metadata.Namespace = deployment.Space
	job.Spec.Template.Metadata.Labels.Name = deployment.App
	job.Spec.Template.Metadata.Labels.Space = deployment.Space
	job.Spec.Template.Spec.ImagePullSecrets = deployment.Secrets
	job.Spec.Template.Spec.DnsPolicy = "Default"
	job.Spec.Template.Spec.Containers = clist
	job.Spec.Template.Spec.RestartPolicy = "OnFailure"

	resp, err := rt.k8sRequest("post", "/apis/batch/v1/namespaces/"+deployment.Space+"/jobs", job)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, errors.New("Unable to create cron job, received from kubernetes: " + resp.Status)
	}

	var status structs.JobStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (rt Kubernetes) DeleteJob(space string, jobName string) (e error) {
	if space == "" {
		return errors.New("Cannot remove job with blank space")
	}
	if jobName == "" {
		return errors.New("Cannot delete job with invalid or missing job name")
	}
	// Cleanup running/old jobs
	resp, err := rt.k8sRequest("delete", "/apis/batch/v1/namespaces/"+space+"/jobs?labelSelector=name="+jobName, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Cannot delete job, kubernetes returned back: " + resp.Status)
	}

	// Cleanup pods from those old jobs
	err = rt.DeletePods(space, jobName)
	if err != nil {
		return err
	}
	return nil
}

func (rt Kubernetes) DeleteCronJob(space string, jobName string) (e error) {
	if space == "" {
		return errors.New("Cannot remove job with blank space")
	}
	if jobName == "" {
		return errors.New("Cannot delete job with invalid or missing job name")
	}
	// Delete Cron Job
	resp, err := rt.k8sRequest("delete", "/apis/batch/v2alpha1/namespaces/"+space+"/cronjobs/"+jobName, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Cannot delete cron job, kubernetes returned back: " + resp.Status)
	}
	return rt.DeleteJob(space, jobName)
}

func (rt Kubernetes) GetJob(space string, jobName string) (*structs.JobStatus, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get job, space is blank.")
	}
	if jobName == "" {
		return nil, errors.New("FATAL ERROR: Unable to get job, the job name is blank.")
	}
	resp, err := rt.k8sRequest("get", "/apis/batch/v1/namespaces/"+space+"/jobs/"+jobName, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get job, kubernetes returned: " + resp.Status)
	}

	var status structs.JobStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (rt Kubernetes) GetJobs(space string) ([]structs.JobStatus, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get jobs, space is blank.")
	}
	resp, err := rt.k8sRequest("get", "/apis/batch/v1/namespaces/"+space+"/jobs", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get jobs, kubernetes returned: " + resp.Status)
	}

	var status []structs.JobStatus
	var jobList structs.JobList
	err = json.Unmarshal(resp.Body, &jobList)
	if err != nil {
		return nil, err
	}

	for _, element := range jobList.Items {
		status = append(status, element)
	}

	return status, nil
}

func (rt Kubernetes) GetCronJob(space string, jobName string) (*structs.CronJobStatus, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get cron job, space is blank.")
	}
	if jobName == "" {
		return nil, errors.New("FATAL ERROR: Unable to get cron job, the jobName is blank.")
	}
	resp, err := rt.k8sRequest("get", "/apis/batch/v2alpha1/namespaces/"+space+"/cronjobs/"+jobName, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get cron job, kubernetes returned: " + resp.Status)
	}

	var status structs.CronJobStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (rt Kubernetes) GetCronJobs(space string) (sjobs []structs.CronJobStatus, e error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get cron jobs, space is blank.")
	}
	resp, err := rt.k8sRequest("get", "/apis/batch/v2alpha1/namespaces/"+space+"/cronjobs", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get cron jobs, kubernetes returned: " + resp.Status)
	}

	var status []structs.CronJobStatus
	var jobList structs.CronJobList
	err = json.Unmarshal(resp.Body, &jobList)
	if err != nil {
		return nil, err
	}
	for _, element := range jobList.Items {
		status = append(status, element)
	}
	return status, nil
}

func (rt Kubernetes) ScaleJob(space string, jobName string, replicas int, timeout int) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to scale job, space is blank.")
	}
	if jobName == "" {
		return errors.New("FATAL ERROR: Unable to scale job, the jobName is blank.")
	}
	resp, e := rt.k8sRequest("get", "/apis/batch/v1/namespaces/"+space+"/jobs/"+jobName, nil)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Unable to get job on scale, kubernetes returned: " + resp.Status)
	}

	var job JobScaleGet
	e = json.Unmarshal(resp.Body, &job)
	if e != nil {
		return e
	}

	job.Spec.Parallelism = replicas
	job.Spec.ActiveDeadlineSeconds = timeout
	resp, e = rt.k8sRequest("put", "/apis/batch/v1/namespaces/"+space+"/jobs/"+jobName, job)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Unable to scale job, kubernetes returned: " + resp.Status)
	}
	return nil
}

func (rt Kubernetes) JobExists(space string, jobName string) bool {
	if space == "" {
		return false
	}
	if jobName == "" {
		return false
	}
	resp, e := rt.k8sRequest("get", "/apis/batch/v1/namespaces/"+space+"/jobs/"+jobName, nil)
	if e != nil {
		return false
	}

	if resp.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func (rt Kubernetes) GetPodsBySpace(space string) (*PodStatus, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get pods by space, space is blank.")
	}
	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/pods", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("space does not exist")
	}
	var podStatus PodStatus
	err = json.Unmarshal(resp.Body, &podStatus)
	if err != nil {
		return nil, err
	}
	return &podStatus, nil
}

func (rt Kubernetes) GetNodes() (*structs.KubeNodes, error) {
	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/nodes", nil)
	if err != nil {
		return nil, err
	}
	var nodes structs.KubeNodes
	err = json.Unmarshal(resp.Body, &nodes)
	if err != nil {
		return nil, err
	}
	return &nodes, nil
}
