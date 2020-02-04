package runtime

import (
	"database/sql"
	structs "region-api/structs"
	"os"
)

type Runtime interface {
	GenericRequest(method string, path string, payload interface{}) ([]byte, int, error)
	Scale(space string, app string, amount int) (e error)
	GetService(space string, app string) (service KubeService, e error)
	ServiceExists(space string, app string) (bool, error)
	CreateService(space string, app string, port int, labels map[string]string, features structs.Features) (e error)
	UpdateService(space string, app string, port int, labels map[string]string, features structs.Features) (e error)
	DeleteService(space string, app string) (e error)
	GetServices() (*ServiceCollectionspec, error)
	CreateDeployment(deployment *structs.Deployment) (err error)
	UpdateDeployment(deployment *structs.Deployment) (err error)
	DeleteDeployment(space string, app string) (e error)
	DeploymentExists(space string, app string) (exists bool, e error)
	GetReplicas(space string, app string) (rs []string, e error)
	DeleteReplica(space string, app string, replica string) (e error)
	CreateOneOffPod(deployment *structs.Deployment) (e error)
	DeletePod(space string, pod string) (e error)
	DeletePods(space string, label string) (e error)
	GetPods(space string, app string) (rs []string, e error)
	CreateSpace(name string, internal bool, compliance string) (e error)
	DeleteSpace(name string) (e error)
	CreateSecret(space string, name string, data string, mimetype string) (s *Secretspec, e error)
	AddImagePullSecretToSpace(space string) (e error)
	UpdateSpaceTags(space string, compliance string) (e error)
	RestartDeployment(space string, app string) (e error)
	GetCurrentImage(space string, app string) (i string, e error)
	GetPodDetails(space string, app string) []structs.Instance
	GetPodLogs(app string, space string, pod string) (log string, err error)
	OneOffExists(space string, name string) bool
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
	CopySecret(secretName string, fromNamespace string, toNamespace string) (error)
}

var stackRuntimeCache map[string]Runtime = make(map[string]Runtime)

// Stubs, incase we ever have more than one stack in a region.
func GetRuntimeStack(db *sql.DB, stack string) (rt Runtime, e error) {
	// Cache for the win! This is also very critical to cache as some
	// config is loaded from files that can switch during runtime and
	// we'd like to force the choice of the runtime at the beginning
	// to prevent getting (accidently) a different runtime after we've
	// been running for a while.
	i, ok := stackRuntimeCache[stack]
	if ok {
		return i, nil
	}
	// At the moment we only support kubernetes, but incase this 
	// should change this would be an opportune time to grab an interface
	// to a different runtime.
	stackRuntimeCache[stack] = NewKubernetes(stack, os.Getenv("IMAGE_PULL_SECRET"))
	return stackRuntimeCache[stack], nil
}

// Stubs, incase we ever have more than one stack in a region.
func GetRuntimeFor(db *sql.DB, space string) (rt Runtime, e error) {
	return GetRuntimeStack(db, "ds1")
}

// Stubs, incase we ever have more than one stack in a region.
func GetAllRuntimes(db *sql.DB) (rt []Runtime, e error) {
	r, e := GetRuntimeStack(db, "ds1")
	if e != nil {
		return nil, e
	}
	return []Runtime{r}, nil
}
