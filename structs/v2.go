package structs

import "database/sql"

// AppDeploymentSpec This represents an app deployment (from the v2.deployments table)
type AppDeploymentSpec struct {
	AppID       sql.NullString `json:"appid"`
	Name        string         `json:"name"`
	Space       string         `json:"space"`
	Instances   int            `json:"instances"`
	Plan        string         `json:"plan"`
	Healthcheck sql.NullString `json:"healthcheck,omitempty"`
	Bindings    []Bindspec     `json:"bindings,omitempty"`
	Image       sql.NullString `json:"image,omitempty"`
}
