package structs

// AppDeploymentSpec This represents an app deployment (from the v2.deployments table)
type AppDeploymentSpec struct {
	AppID       string     `json:"appid"`
	Name        string     `json:"name"`
	Space       string     `json:"space"`
	Instances   int        `json:"instances"`
	Plan        string     `json:"plan"`
	Healthcheck string     `json:"healthcheck,omitempty"`
	Bindings    []Bindspec `json:"bindings,omitempty"`
	Image       string     `json:"image,omitempty"`
}
