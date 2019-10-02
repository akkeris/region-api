package runtime

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	vault "github.com/akkeris/vault-client"
	"io/ioutil"
	"log"
	"net/http"
	"fmt"
	"os"
	"reflect"
	structs "region-api/structs"
	"strconv"
	"strings"
	"sync"
	"time"
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
	client                  *http.Client
	mutex                   *sync.Mutex
	debug                   bool
}

type KubernetesConfig struct {
	Name            string
	APIServer       string
	APIVersion      string
	ImagePullSecret string
	AuthType        string
	AuthVaultPath   string
}

func NewKubernetes(config *KubernetesConfig) (r Runtime) {
	log.Println("Using kubernetes server " + config.APIServer)
	if config.ImagePullSecret == "" {
		log.Fatalln("No kubernetes name was available! Aborting.")
	}
	if config.ImagePullSecret == "" {
		log.Fatalln("No kubernetes image pull secret was available! Aborting.")
	}
	if config.APIServer == "" {
		log.Fatalln("No kubernetes api server was available! Aborting.")
	}
	if config.APIVersion == "" {
		log.Fatalln("No kubernetes api version was available! Aborting.")
	}
	if config.AuthType == "" {
		log.Fatalln("No kubernetes auth type was available! Aborting.")
	}
	if config.AuthVaultPath == "" {
		log.Fatalln("No kubernetes auth vault path was available! Aborting.")
	}
	var rt Kubernetes
	rt.clientType = config.AuthType
	rt.apiServer = config.APIServer
	rt.defaultApiServerVersion = config.APIVersion
	rt.imagePullSecret = config.ImagePullSecret
	rt.mutex = &sync.Mutex{}
	rt.debug = os.Getenv("DEBUG_K8S") == "true"
	if rt.clientType == "cert" {
		secret := vault.GetSecret(config.AuthVaultPath)
		admin_crt := strings.Replace(vault.GetFieldFromVaultSecret(secret, "admin-crt"), "\\n", "\n", -1)
		admin_key := strings.Replace(vault.GetFieldFromVaultSecret(secret, "admin-key"), "\\n", "\n", -1)
		cert, err := tls.X509KeyPair([]byte(admin_crt), []byte(admin_key))
		if err != nil {
			log.Println(admin_crt)
			log.Println(admin_key)
			log.Println("Failed to obtain kubernetes certificate for " + rt.apiServer)
			log.Fatalln(err)
		}
		caCertPool := x509.NewCertPool()
		ca_crt := strings.Replace(vault.GetFieldFromVaultSecret(secret, "ca-crt"), "\\n", "\n", -1)
		caCertPool.AppendCertsFromPEM([]byte(ca_crt))
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}
		tlsConfig.BuildNameToCertificate()
		rt.client = &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	} else if rt.clientType == "token" {
		rt.clientToken = vault.GetField(config.AuthVaultPath, "token")
		rt.client = &http.Client{}
	} else {
		log.Fatalln("No valid authentication method was found for kubernetes " + rt.apiServer)
	}

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

	// assemble readiness proble
	var probe ReadinessProbe
	if deployment.Port != -1 {
		var cp1 ContainerPort
		if deployment.HealthCheck != "tcp" {
			probe.HTTPGET = &HttpCheck{Port: deployment.Port, Path: deployment.HealthCheck}
		} else {
			probe.TCPSocket = &TcpCheck{Port: deployment.Port}
		}
		cp1.ContainerPort = deployment.Port
		c1.ReadinessProbe = &probe
		cportlist := []ContainerPort{}
		c1.Ports = append(cportlist, cp1)
	}

	// assemble image
	c1.Name = deployment.App
	c1.Image = deployment.Image + ":" + deployment.Tag
	if len(deployment.Command) > 0 {
		c1.Command = deployment.Command
	}
	c1.ImagePullPolicy = "Always"

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
	krc.Spec.Template.Metadata.Labels["name"] = deployment.App	// unsure what this is used for.
	krc.Spec.Template.Metadata.Labels["app"] = deployment.App	// unsure what this is used for.
	krc.Spec.Template.Metadata.Labels["version"] = "v1"	// unsure what this is used for.

	if os.Getenv("FF_ISTIOINJECT") == "true" || deployment.Features.IstioInject || deployment.Features.ServiceMesh {
		krc.Spec.Template.Metadata.Annotations.SidecarIstioIoInject = "true"
	}

	krc.Spec.Strategy.RollingUpdate.MaxUnavailable = 0
	krc.Spec.Template.Spec.ImagePullSecrets = deployment.Secrets
	krc.Spec.Template.Spec.Containers = clist
	krc.Spec.Template.Spec.ImagePullPolicy = "Always"
	krc.Spec.Template.Spec.TerminationGracePeriodSeconds = 60

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

func (rt Kubernetes) GetDeployment(space string, app string) (*Deploymentspec, error) {
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
	deployment, e := rt.GetDeployment(space, app)
	if e != nil {
		if e.Error() == "deployment not found" {
			// Sometimes a restart can get issued for an app that has yet to be deployed,
			// just ignore the error and do nothing.
			return nil
		}
		return e
	}
	oldenvs := deployment.Spec.Template.Spec.Containers[0].Env
	var newenvs []structs.EnvVar
	newenvs = append(newenvs, structs.EnvVar{Name: "RESTART", Value: time.Now().Format(time.RFC850)})
	for _, element := range oldenvs {
		if element.Name != "RESTART" {
			newenvs = append(newenvs, element)
		}
	}
	deployment.Spec.Template.Spec.Containers[0].Env = newenvs

	_, e = rt.k8sRequest("put", "/apis/apps/v1/namespaces/"+space+"/deployments/"+app, deployment)
	return e
}

func (rt Kubernetes) GetDeployments() (*DeploymentCollectionspec, error) {
	resp, e := rt.k8sRequest("get", "/apis/apps/v1/deployments", nil)
	if e != nil {
		return nil, e
	}
	var deployments DeploymentCollectionspec
	e = json.Unmarshal(resp.Body, &deployments)
	if e != nil {
		return nil, e
	}
	return &deployments, nil
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
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

	var koneoff OneOffPod
	koneoff.Metadata.Name = deployment.App
	koneoff.Metadata.Namespace = deployment.Space
	koneoff.APIVersion = "v1"
	koneoff.Kind = "Pod"
	koneoff.Metadata.Labels.Name = deployment.App
	koneoff.Metadata.Labels.Space = deployment.Space
	koneoff.Spec.RestartPolicy = "Never"
	koneoff.Spec.ImagePullPolicy = "Always"
	koneoff.Spec.DnsPolicy = "Default"

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
	if resp.StatusCode != 201 {
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
	if resp.StatusCode != 200 {
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

func (rt Kubernetes) AddImagePullSecretToSpace(space string) (e error) {
	var sa Serviceaccountspec = Serviceaccountspec{Metadata: structs.Namespec{Name: "default"}}
	var ipss []structs.Namespec
	ipss = append(ipss, structs.Namespec{Name: rt.imagePullSecret})
	sa.ImagePullSecrets = ipss
	resp, err := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/serviceaccounts/default", sa)
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return errors.New("Unable to add secret to space, invalid response code from kubernetes: " + resp.Status)
	}
	return err
}

func (rt Kubernetes) GetCurrentImage(space string, app string) (i string, e error) {
	var image string
	deployment, err := rt.GetDeployment(space, app)
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
		log.Println("Cannot get pod from kuberenetes:")
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
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return response, e
	}
	return response, nil
}

func (rt Kubernetes) CreateService(space string, app string, port int, labels map[string]string, features structs.Features) (c *Createspec, e error) {
	if space == "" {
		return nil, errors.New("FATAL ERROR: Unable to create service, space is blank.")
	}
	if app == "" {
		return nil, errors.New("FATAL ERROR: Unable to create service, the app is blank.")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("Invalid port range on CreateService: " + strconv.Itoa(port))
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
		return nil, e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	return &response, nil
}

func (rt Kubernetes) UpdateService(space string, app string, port int, labels map[string]string, features structs.Features) (c *Createspec, e error) {
	if space == "" {
		return nil, errors.New("Unable to update service, space is blank.")
	}
	if app == "" {
		return nil, errors.New("Unable to update service, the app is blank.")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("Invalid port range on UpdateService: " + strconv.Itoa(port))
	}
	existingservice, e := rt.GetService(space, app)
	if e != nil {
		return nil, e
	}
	if features.Http2EndToEndService {
		existingservice.Spec.Ports[0].Name = "http2"
	} else {
		existingservice.Spec.Ports[0].Name = "http"
	}
	existingservice.Spec.Ports[0].TargetPort = port
	for k := range labels {
		existingservice.Metadata.Labels[k] = labels[k]
	}

	resp, e := rt.k8sRequest("put", "/api/"+rt.defaultApiServerVersion+"/namespaces/"+space+"/services/"+app, existingservice)
	if e != nil {
		return nil, e
	}
	var response Createspec
	e = json.Unmarshal(resp.Body, &response)
	if e != nil {
		return nil, e
	}
	return &response, nil
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

	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

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

	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

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

	// Image Secret
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		deployment.Secrets = append(deployment.Secrets, structs.Namespec{Name: rt.imagePullSecret})
	}

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
