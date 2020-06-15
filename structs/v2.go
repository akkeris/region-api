package structs

import (
	"database/sql"
	"encoding/json"
)

// PrettyNullString is a pretty wrapper around sql.NullString
type PrettyNullString struct {
	sql.NullString
}

// MarshalJSON is called by json.Marshal when it is type PrettyNullString
func (s PrettyNullString) MarshalJSON() ([]byte, error) {
	if s.Valid {
		return json.Marshal(s.String)
	}
	return []byte("null"), nil
}

// PrettyNullInt64 is a pretty wrapper around sql.NullInt64
type PrettyNullInt64 struct {
	sql.NullInt64
}

// MarshalJSON is called by json.Marshal when it is type PrettyNullString
func (n PrettyNullInt64) MarshalJSON() ([]byte, error) {
	if n.Valid {
		return json.Marshal(n.Int64)
	}
	return []byte("null"), nil
}

// AppDeploymentSpec This represents an app deployment (from the v2.deployments table)
type AppDeploymentSpec struct {
	AppID       PrettyNullString `json:"appid,omitempty"`
	Name        string           `json:"name"`
	Space       string           `json:"space"`
	Instances   PrettyNullInt64  `json:"instances"`
	Plan        string           `json:"plan"`
	Healthcheck PrettyNullString `json:"healthcheck,omitempty"`
	Bindings    []Bindspec       `json:"bindings,omitempty"`
	Image       PrettyNullString `json:"image,omitempty"`
}

// AppRenameSpec - Parameters for renaming an app
type AppRenameSpec struct {
	Name string `json:"name"`
}
