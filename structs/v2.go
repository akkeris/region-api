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

// UnmarshalJSON is called by json.Marshal when it is type PrettyNullString
func (s *PrettyNullString) UnmarshalJSON(data []byte) error {
	// Unmarshal into pointer so we can detect null
	var x *string
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		s.Valid = true
		s.String = *x
	} else {
		s.Valid = false
	}
	return nil
}

// PrettyNullInt64 is a pretty wrapper around sql.NullInt64
type PrettyNullInt64 struct {
	sql.NullInt64
}

// MarshalJSON is called by json.Marshal when it is type PrettyNullInt64
func (n PrettyNullInt64) MarshalJSON() ([]byte, error) {
	if n.Valid {
		return json.Marshal(n.Int64)
	}
	return []byte("null"), nil
}

// UnmarshalJSON is called by json.Marshal when it is type PrettyNullInt64
func (n *PrettyNullInt64) UnmarshalJSON(data []byte) error {
	// Unmarshal into pointer so we can detect null
	var x *int64
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		n.Valid = true
		n.Int64 = *x
	} else {
		n.Valid = false
	}
	return nil
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

// DeploySpecV2 deployment spec for V2 endpoints
type DeploySpecV2 struct {
	AppID    string            `json:"appid,omitempty"`
	Name     string            `json:"name"`
	Space    string            `json:"space"`
	Port     int               `json:"port"`
	Command  []string          `json:"command"`
	Image    string            `json:"image"`
	Features Features          `json:"features,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Filters  []HttpFilters     `json:"filters,omitempty"`
	OneOff   bool              `json:"oneoff,omitempty"`
}
