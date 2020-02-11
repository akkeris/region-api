package runtime

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	structs "region-api/structs"
	"strconv"
	"strings"
	"sync"
	"time"
	"flag"
	"path/filepath"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	resource "k8s.io/apimachinery/pkg/api/resource"
	v1beta1_batch "k8s.io/api/batch/v1beta1"
	v1batch "k8s.io/api/batch/v1"
	v1beta1 "k8s.io/api/apps/v1beta1"
	v1beta2 "k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/rest"
)

/**
 * For various types see:
 * https://github.com/kubernetes/api/blob/master/core/v1/types.go
 * https://github.com/kubernetes/api/blob/master/apps/v1/types.go
 * https://github.com/kubernetes/api/blob/master/apps/v1beta2/types.go
 * https://github.com/kubernetes/api/blob/master/apps/v1beta1/types.go
 * https://github.com/kubernetes/apimachinery/tree/master/pkg/apis/meta/v1
 * https://github.com/kubernetes/apimachinery/blob/master/pkg/api/resource/quantity.go
 */

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
	config					*rest.Config
	client                  *http.Client
	mutex                   *sync.Mutex
	debug                   bool
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
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
			tlsConfig.RootCAs = certs;
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

func (rt Kubernetes) Scale(space string, app string, amount int32) (e error) {
	if space == "" {
		return errors.New("FATAL ERROR: Unable to scale app, space is blank.")
	}
	if app == "" {
		return errors.New("FATAL ERROR: Unable to scale app, the app is blank.")
	}
	if amount < 0 {
		return errors.New("FATAL ERROR: Unable to scale app, amount is not a whole positive number.")
	}
	scale := v1beta2.Scale{Spec: v1beta2.ScaleSpec{Replicas: amount}}
	scale.SetNamespace(space)
	scale.SetName(app)
	_, err := rt.k8sRequest("put", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app+"/scale", scale)
	if err != nil {
		return err
	}
	return nil
}

func deploymentToDeploymentSpec(deployment *structs.Deployment) (dp v1.Deployment) {
	var c1 corev1.Container
	// assign environment variables
	c1.Env = make([]corev1.EnvVar, 0)
	for _, env := range deployment.ConfigVars {
		c1.Env = append(c1.Env, corev1.EnvVar{Name: env.Name, Value:env.Value})
	}
	clist := []corev1.Container{}

	// assemble readiness probe
	var probe corev1.Probe
	if deployment.Port != -1 {
		var cp1 corev1.ContainerPort
		if deployment.HealthCheck != "tcp" {
			probe.HTTPGet = &corev1.HTTPGetAction{Port: intstr.FromInt(deployment.Port), Path: deployment.HealthCheck}
		} else {
			probe.TCPSocket = &corev1.TCPSocketAction{Port: intstr.FromInt(deployment.Port)}
		}
		cp1.ContainerPort = int32(deployment.Port)
		probe.PeriodSeconds = 20
		probe.TimeoutSeconds = 15
		c1.ReadinessProbe = &probe
		c1.Ports = []corev1.ContainerPort{cp1}
	}

	// assemble image
	c1.Name = deployment.App
	c1.Image = deployment.Image + ":" + deployment.Tag
	if len(deployment.Command) > 0 {
		c1.Command = deployment.Command
	}
	c1.ImagePullPolicy = "Always"

	// assemble memory constraints
	var resources corev1.ResourceRequirements
	memRequest, err := resource.ParseQuantity(deployment.MemoryRequest)
	if err != nil {
		panic(err)
	}
	memLimit, err := resource.ParseQuantity(deployment.MemoryLimit)
	if err != nil {
		panic(err)
	}
	resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	resources.Requests[corev1.ResourceMemory] = memRequest
	resources.Limits[corev1.ResourceMemory] = memLimit
	c1.Resources = resources

	clist = append(clist, c1)

	var krc v1.Deployment

	krc.SetName(deployment.App)
	krc.SetNamespace(deployment.Space)
	krc.SetLabels(deployment.Labels)
	var replicas int32 = int32(deployment.Amount)
	var revision int32 = int32(deployment.RevisionHistoryLimit)
	krc.Spec.Replicas = &replicas
	krc.Spec.RevisionHistoryLimit = &revision
	matchLabels := metav1.LabelSelector{}
	krc.Spec.Selector = &matchLabels
	krc.Spec.Selector.MatchLabels = map[string]string{"name": deployment.App}
	krc.Spec.Template.SetName(deployment.App)

	// copy labels to pod spec, add name app and version as well.
	krc.Spec.Template.SetLabels(make(map[string]string))
	for key, val := range deployment.Labels {
		krc.Spec.Template.GetLabels()[key] = val
	}
	krc.Spec.Template.GetLabels()["name"] = deployment.App // unsure what this is used for.
	krc.Spec.Template.GetLabels()["app"] = deployment.App  // unsure what this is used for.
	krc.Spec.Template.GetLabels()["version"] = "v1"        // unsure what this is used for.
	krc.Spec.Template.SetAnnotations(make(map[string]string))
	if os.Getenv("FF_ISTIOINJECT") == "true" || deployment.Features.IstioInject || deployment.Features.ServiceMesh {
		krc.Spec.Template.GetAnnotations()["sidecar.istio.io/inject"] = "true"
	}
	maxUnavailable := intstr.FromInt(0)
	krc.Spec.Strategy.RollingUpdate.MaxUnavailable = &maxUnavailable
	krc.Spec.Template.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
	for _, item := range deployment.Secrets {
		krc.Spec.Template.Spec.ImagePullSecrets = append(krc.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name:item.Name})
	}
	krc.Spec.Template.Spec.Containers = clist
	var terminationGracePeriodSeconds int64 = 60
	krc.Spec.Template.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

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
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

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
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}
	resp, err := rt.k8sRequest("POST", "/apis/apps/v1/namespaces/"+deployment.Space+"/deployments", deploymentToDeploymentSpec(deployment))
	if err != nil {
		return err
	}
	if resp.StatusCode > 399 || resp.StatusCode < http.StatusOK {
		return errors.New("Cannot create deployment for " + deployment.App + "-" + deployment.Space + " received: " + resp.Status + " " + string(resp.Body))
	}
	return nil
}

func (rt Kubernetes) getDeployment(space string, app string) (*v1.Deployment, error) {
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
	var deployment v1.Deployment
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
	deployment.Spec.Template.GetAnnotations()["akkeris.io/restartTime,omitempty"] = currentTime

	_, e = rt.k8sRequest("put", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, deployment)
	return e
}


func (rt Kubernetes) Exec(space string, app string, instance string, command []string) (error) {
	var method = "POST"
	var commandQuery = strings.Join(command, "&command=")
	var path = "/api/" + rt.defaultApiServerVersion + "/namespaces/" + space + "/pods/" + app + "-" + instance + "/exec?command=" + commandQuery +  "&container=" + app + "&stdin=false&stdout=true&tty=false"
	req, err := http.NewRequest("POST", "https://"+rt.apiServer+path, nil)
	if err != nil {
		return err
	}
	if rt.clientType == "token" {
		req.Header.Add("Authorization", "Bearer "+rt.clientToken)
	}
	req.Header.Add("X-Stream-Protocol-Version", "channel.k8s.io")
	req.Header.Add("X-Stream-Protocol-Version", "v3.channel.k8s.io")
	req.Header.Add("X-Stream-Protocol-Version", "v4.channel.k8s.io")
	req.Header.Add("User-Agent", "region-api")
	if rt.debug {
		log.Printf("-> k8: %s %s with headers [%s] with command [%#+v]\n", method, "https://"+rt.apiServer+path, req.Header, command)
	}
	rt.mutex.Lock()
	resp, err := rt.client.Do(req)
	rt.mutex.Unlock()
	if err != nil {
		if rt.debug {
			log.Printf("<- k8 ERROR: %s %s - %s\n", method, "https://"+rt.apiServer+path, err)
		}
		return  err
	}
	resp.Body.Close()
	if rt.debug {
		log.Printf("<- k8: %s %s - %s\n", method, "https://"+rt.apiServer+path, resp.Status)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errors.New("Received error trying to run exec " + resp.Status)
	}
	return nil
}

func (rt Kubernetes) GetDeploymentHistory(space string, app string) (dslist []structs.DeploymentsSpec, e error) {
	resp, err := rt.k8sRequest("get", "/apis/apps/v1/namespaces/"+space+"/replicasets?labelSelector=name="+app, nil)
	if err != nil {
		log.Println(err)
	}
	var rsl v1.ReplicaSetList
	e = json.Unmarshal(resp.Body, &rsl)
	if e != nil {
		return nil, e
	}
	for _, element := range rsl.Items {
		var ds structs.DeploymentsSpec
		ds.Name = element.GetName()
		ds.Space = element.GetNamespace()
		ds.CreationTimestamp = element.GetCreationTimestamp().UTC()
		ds.Image = element.Spec.Template.Spec.Containers[0].Image
		ds.Revision = element.GetAnnotations()["deployment.kubernetes.io/revision"]
		dslist = append(dslist, ds)
	}
	return dslist, nil
}

func (rt Kubernetes) RollbackDeployment(space string, app string, revision int) (e error) {
	//var rollbackspec Rollbackspec = Rollbackspec{ApiVersion: "apps/v1", Name: app, RollbackTo: Revisionspec{Revision: revision}}
	//rollbackspec.RollbackTo.Revision = revision
	// According to kubernetes docs this has been deprecated,
	// may consider removing https://github.com/kubernetes/api/blob/master/apps/v1beta1/types.go#L361

	rollback := v1beta1.DeploymentRollback{Name: app, RollbackTo:v1beta1.RollbackConfig{Revision: int64(revision)}}
	_, e = rt.k8sRequest("post", "/apis/apps/v1beta1/namespaces/"+space+"/deployments/"+app+"/rollback", rollback)
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
	var response v1.ReplicaSetList
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	for _, item := range response.Items {
		rs = append(rs, item.GetName())
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
	var response corev1.PodList
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	for _, item := range response.Items {
		rs = append(rs, item.GetName())
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
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

	var koneoff corev1.Pod
	koneoff.SetName(deployment.App)
	koneoff.SetNamespace(deployment.Space)
	koneoff.SetLabels(map[string]string{"name":deployment.App, "space":deployment.Space})
	koneoff.Spec.RestartPolicy = "Never"
	koneoff.Spec.DNSPolicy = "Default"

	var container corev1.Container
	container.ImagePullPolicy = "Always"
	container.Name = deployment.App
	container.Image = deployment.Image + ":" + deployment.Tag
	//container.ImagePullSecrets = deployment.Secrets
	container.Env = make([]corev1.EnvVar, 0)
	for _, env := range deployment.ConfigVars {
		container.Env = append(container.Env, corev1.EnvVar{Name: env.Name, Value:env.Value})
	}

	if len(deployment.Command) > 0 {
		container.Command = deployment.Command
	}

	// assemble memory constraints
	var resources corev1.ResourceRequirements
	memRequest, err := resource.ParseQuantity(deployment.MemoryRequest)
	if err != nil {
		panic(err)
	}
	memLimit, err := resource.ParseQuantity(deployment.MemoryLimit)
	if err != nil {
		panic(err)
	}
	resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	resources.Requests[corev1.ResourceMemory] = memRequest
	resources.Limits[corev1.ResourceMemory] = memLimit
	container.Resources = resources

	clist := []corev1.Container{}
	clist = append(clist, container)

	koneoff.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
	for _, item := range deployment.Secrets {
		koneoff.Spec.ImagePullSecrets = append(koneoff.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name:item.Name})
	}
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
	var namespace corev1.Namespace
	namespace.SetName(name)
	if len(compliance) > 0 {
		namespace.SetAnnotations(map[string]string{"akkeris.io/compliancetags": compliance})
	}
	if internal {
		namespace.SetLabels(map[string]string{"akkeris.io/internal": "true"})
	} else {
		namespace.SetLabels(map[string]string{"akkeris.io/internal": "false"})
	}
	resp, e := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces", namespace)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errors.New("Unable to create space, invalid response code from kubernetes: " + resp.Status)
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
	var namespace corev1.Namespace
	namespace.SetName(space)
	namespace.SetAnnotations(map[string]string{"akkeris.io/compliancetags": compliance})
	_, e = rt.k8sRequest("patch", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space, namespace)
	return e
}

func (rt Kubernetes)  CopySecret(secretName string, fromNamespace string, toNamespace string) (error) {
	if fromNamespace == "" {
		return errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if toNamespace == "" {
		return errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}
	
	var secret corev1.Secret
	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+fromNamespace+"/secrets/" + secretName, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Unable to copy secret, invalid response code from kubernetes on fetch: " + resp.Status)
	}

	if err := json.Unmarshal(resp.Body, &secret); err != nil {
		return err
	}

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
	var sa corev1.ServiceAccount
	sa.SetName("default")
	sa.ImagePullSecrets = []corev1.LocalObjectReference{corev1.LocalObjectReference{Name: rt.imagePullSecret}}
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
	var pods corev1.PodList
	err = json.Unmarshal(resp.Body, &pods)
	if err != nil {
		log.Println("Failed to decode pod details:")
		log.Println(err)
	}

	var statuses []structs.SpaceAppStatus
	for _, element := range pods.Items {
		var s structs.SpaceAppStatus
		s.App = app
		s.Space = space
		if element.Status.Phase == "Running" {
			s.Status = 0
			s.Output = element.GetName() + "=" + string(element.Status.Phase)
		}
		if element.Status.Phase != "Running" {
			s.Status = 2
			s.Output = string(element.Status.Phase)
		}
		s.Reason = element.Status.Reason
		s.ExtendedOutput = element.Status.Message
		s.Ready = false
		if len(element.Status.ContainerStatuses) > 0 {
			s.Ready = element.Status.ContainerStatuses[0].Ready
			s.Restarted = int(element.Status.ContainerStatuses[0].RestartCount)
			s.State = make(map[string]interface{})
			if element.Status.ContainerStatuses[0].State.Waiting != nil {
				s.State["waiting"] = map[string]interface{}{"reason":element.Status.ContainerStatuses[0].State.Waiting.Reason, "message":element.Status.ContainerStatuses[0].State.Waiting.Message,}
			}
			if element.Status.ContainerStatuses[0].State.Running != nil {
				s.State["running"] = map[string]interface{}{"startedAt":element.Status.ContainerStatuses[0].State.Running.StartedAt,}
			}
			if element.Status.ContainerStatuses[0].State.Terminated != nil {
				s.State["terminated"] = map[string]interface{}{"exitCode":element.Status.ContainerStatuses[0].State.Terminated.ExitCode, "signal":element.Status.ContainerStatuses[0].State.Terminated.Signal, "reason":element.Status.ContainerStatuses[0].State.Terminated.Reason, "message":element.Status.ContainerStatuses[0].State.Terminated.Message, "startedAt":element.Status.ContainerStatuses[0].State.Terminated.StartedAt, "finishedAt":element.Status.ContainerStatuses[0].State.Terminated.FinishedAt, "containerID":element.Status.ContainerStatuses[0].State.Terminated.ContainerID,}
			}
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
		log.Println("Cannot get pod from kuberenetes:")
		log.Println(err)
	}
	var podstatus corev1.PodList
	err = json.Unmarshal(resp.Body, &podstatus)
	if err != nil {
		log.Println("Failed to decode pod details:")
		log.Println(err)
	}

	for _, element := range podstatus.Items {
		var instance structs.Instance
		instance.InstanceID = element.GetName()
		instance.Phase = string(element.Status.Phase)
		instance.StartTime = element.Status.StartTime.UTC()
		var appstatus []structs.Appstatus
		for _, containerStatus := range element.Status.ContainerStatuses {
			var as structs.Appstatus
			var state = "waiting"
			if containerStatus.State.Waiting != nil { //containerstate == "waiting" {
				as.StartedAt = element.Status.StartTime.UTC()
				instance.Reason = containerStatus.State.Waiting.Reason
			}
			if containerStatus.State.Running != nil { //containerstate == "running" {
				state = "running"
				instance.Reason = ""
				as.StartedAt = containerStatus.State.Running.StartedAt.UTC()
			}
			if containerStatus.State.Terminated != nil { //if containerstate == "terminated" {
				state = "terminated"
				instance.Reason = containerStatus.State.Terminated.Reason
				// no idea why this is started at, but when we moved
				// to kube strucutres it was this way, so we'll keep it
				as.StartedAt = containerStatus.State.Terminated.FinishedAt.UTC()
			}

			instance.Phase = instance.Phase + "/" + state
			as.App = containerStatus.Name
			as.ReadyStatus = containerStatus.Ready
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

func (rt Kubernetes) GetService(space string, app string) (*corev1.Service, error) {
	var response corev1.Service
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get service, space is blank.")
	}
	if app == "" {
		return nil, errors.New("FATAL ERROR: Unable to get service, the app is blank.")
	}
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, nil)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get service " + resp.Status)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("service not found")
	}
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return &response, e
	}
	return &response, nil
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

	var service corev1.Service
	service.SetAnnotations(map[string]string{"service.beta.kubernetes.io/aws-load-balancer-internal":"0.0.0.0/0"})
	service.SetName(app)
	labels["app"] = app
	labels["name"] = app
	service.SetLabels(labels)
	service.Spec.Selector = make(map[string]string)
	service.Spec.Selector["name"] = app

	var portitem corev1.ServicePort
	portitem.Protocol = "TCP"
	portitem.Port = 80
	portitem.TargetPort = intstr.FromInt(port)
	if features.Http2EndToEndService {
		portitem.Name = "http2"
	} else {
		portitem.Name = "http"
	}
	service.Spec.Ports = []corev1.ServicePort{portitem}
	service.Spec.Type = "NodePort"

	resp, e := rt.k8sRequest("post", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services", service)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errors.New("Unable to create service: " + resp.Status)
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
		existingservice.Spec.Ports[0].TargetPort = intstr.FromInt(port)
	}
	for k := range labels {
		existingservice.GetLabels()[k] = labels[k]
	}

	resp, e := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, existingservice)
	if e != nil {
		return e
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errors.New("Unable to update service: " + resp.Status)
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusConflict {
		return errors.New("Unable to delete service: " + resp.Status)
	}
	return nil
}

func (rt Kubernetes) GetServices() (*corev1.ServiceList, error) {
	resp, e := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/services", nil)
	if e != nil {
		return nil, e
	}
	var services corev1.ServiceList
	e = json.Unmarshal(resp.Body, &services)
	if e != nil {
		return nil, e
	}
	return &services, nil
}

func (rt Kubernetes) CronJobExists(space string, job string) bool {
	resp, err := rt.k8sRequest("get", "/apis/batch/v1beta1/namespaces/"+space+"/cronjobs/"+job, nil)
	if err != nil {
		return false
	}
	if resp.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func deploymentToCronJob(deployment *structs.Deployment) (cronJob *v1beta1_batch.CronJob) {
	var resources corev1.ResourceRequirements
	memRequest, err := resource.ParseQuantity(deployment.MemoryRequest)
	if err != nil {
		panic(err)
	}
	memLimit, err := resource.ParseQuantity(deployment.MemoryLimit)
	if err != nil {
		panic(err)
	}
	resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	resources.Requests[corev1.ResourceMemory] = memRequest
	resources.Limits[corev1.ResourceMemory] = memLimit

	// Image and config
	var container corev1.Container
	container.Name = deployment.App
	container.Image = deployment.Image
	container.Env = make([]corev1.EnvVar, 0)
	for _, env := range deployment.ConfigVars {
		container.Env = append(container.Env, corev1.EnvVar{Name: env.Name, Value:env.Value})
	}
	container.Resources = resources

	clist := []corev1.Container{container}

	var job v1beta1_batch.JobTemplateSpec
	job.SetName(deployment.App)
	job.SetNamespace(deployment.Space)
	job.SetLabels(map[string]string{"name":deployment.App, "space":deployment.Space})
	job.Spec.Template.SetName(deployment.App)
	job.Spec.Template.SetNamespace(deployment.Space)
	job.Spec.Template.SetLabels(map[string]string{"name":deployment.App, "space":deployment.Space})
	job.Spec.Template.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
	for _, item := range deployment.Secrets {
		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name:item.Name})
	}

	job.Spec.Template.Spec.Containers = clist
	job.Spec.Template.Spec.DNSPolicy = "Default"
	job.Spec.Template.Spec.RestartPolicy = "OnFailure"

	// Template for CronJob
	cronJob.SetName(deployment.App)
	cronJob.SetNamespace(deployment.Space)
	cronJob.SetLabels(map[string]string{"name":deployment.App, "space":deployment.Space})
	cronJob.Spec.Schedule = deployment.Schedule
	var startingDeadlineSeconds = int64(60)
	cronJob.Spec.StartingDeadlineSeconds = &startingDeadlineSeconds 
	cronJob.Spec.JobTemplate = job

	return cronJob
}

func (rt Kubernetes) CreateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error) {
	if deployment.Space == "" {
		return nil, errors.New("FATAL ERROR: Unable to create cronjob, space is blank.")
	}

	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

	// Update or Create Job
	resp, err := rt.k8sRequest("post", "/apis/batch/v1beta1/namespaces/"+deployment.Space+"/cronjobs", deploymentToCronJob(deployment))
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

func (rt Kubernetes) GetCronJob(space string, jobName string) (*structs.CronJobStatus, error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to get cron job, space is blank.")
	}
	if jobName == "" {
		return nil, errors.New("FATAL ERROR: Unable to get cron job, the jobName is blank.")
	}
	resp, err := rt.k8sRequest("get", "/apis/batch/v1beta1/namespaces/"+space+"/cronjobs/"+jobName, nil)
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
	resp, err := rt.k8sRequest("get", "/apis/batch/v1beta1/namespaces/"+space+"/cronjobs", nil)
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

func (rt Kubernetes) UpdateCronJob(deployment *structs.Deployment) (*structs.CronJobStatus, error) {
	if deployment.Space == "" {
		return nil, errors.New("FATAL ERROR: Unable to update cron job, space is blank.")
	}
	if deployment.App == "" {
		return nil, errors.New("FATAL ERROR: Unable to update cron job, the app is blank.")
	}

	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

	resp, err := rt.k8sRequest("put", "/apis/batch/v1beta1/namespaces/"+deployment.Space+"/cronjobs/"+deployment.App, deploymentToCronJob(deployment))
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

	// Image Secret
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

	var container corev1.Container
	var resources corev1.ResourceRequirements
	memRequest, err := resource.ParseQuantity(deployment.MemoryRequest)
	if err != nil {
		panic(err)
	}
	memLimit, err := resource.ParseQuantity(deployment.MemoryLimit)
	if err != nil {
		panic(err)
	}
	resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	resources.Requests[corev1.ResourceMemory] = memRequest
	resources.Limits[corev1.ResourceMemory] = memLimit
	container.Resources = resources

	container.Name = deployment.App
	container.Image = deployment.Image + ":" + deployment.Tag
	container.ImagePullPolicy = "Always"

	container.Env = make([]corev1.EnvVar, 0)
	for _, env := range deployment.ConfigVars {
		container.Env = append(container.Env, corev1.EnvVar{Name: env.Name, Value:env.Value})
	}

	var job v1batch.Job
	job.SetName(deployment.App)
	job.SetNamespace(deployment.Space)
	job.SetLabels(map[string]string{"name":deployment.App,"space":deployment.Space})
	job.Spec.Template.SetName(deployment.App)
	job.Spec.Template.SetNamespace(deployment.Space)
	job.Spec.Template.SetLabels(map[string]string{"name":deployment.App,"space":deployment.Space})
	job.Spec.Template.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
	for _, item := range deployment.Secrets {
		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name:item.Name})
	}
	job.Spec.Template.Spec.DNSPolicy = "Default"
	job.Spec.Template.Spec.Containers = []corev1.Container{container}
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

	var job v1batch.Job
	e = json.Unmarshal(resp.Body, &job)
	if e != nil {
		return e
	}
	var r int32 = int32(replicas)
	var t int64 = int64(timeout)
	job.Spec.Parallelism = &r
	job.Spec.ActiveDeadlineSeconds = &t
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

func (rt Kubernetes) GetPodsBySpace(space string) (*corev1.PodList, error) {
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
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get pods by space: " + resp.Status)
	}
	var podStatus corev1.PodList
	err = json.Unmarshal(resp.Body, &podStatus)
	if err != nil {
		return nil, err
	}
	return &podStatus, nil
}

func (rt Kubernetes) GetNodes() (*corev1.NodeList, error) {
	resp, err := rt.k8sRequest("get", "/api/"+rt.defaultApiServerVersion+"/nodes", nil)
	if err != nil {
		return nil, err
	}
	var nodes corev1.NodeList
	err = json.Unmarshal(resp.Body, &nodes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to get list of nodes: " + resp.Status)
	}
	return &nodes, nil
}
