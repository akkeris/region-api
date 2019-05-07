package router

import (
	"database/sql"
	"errors"
	structs "region-api/structs"
)

type Ingress interface {
	SetMaintenancePage(app string, space string, value bool) error
	GetMaintenancePageStatus(app string, space string) (bool, error)
	DeleteRouter(router structs.Routerspec) error
	CreateOrUpdateRouter(router structs.Routerspec) error
	InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error
	GetInstalledCertificates(site string) ([]Certificate, error)
	Config() *IngressConfig
	Name() string
}

func GetAppIngress(db *sql.DB, internal bool) (Ingress, error) {
	config, err := GetAppsIngressPublicExternal()
	if err != nil {
		return nil, err
	}
	if internal {
		config, err = GetAppsIngressPrivateInternal()
		if err != nil {
			return nil, err
		}
	}
	if config.Device == "f5" {
		ing, err := GetF5Ingress(db, config)
		if err != nil {
			return nil, err
		}
		var in Ingress = Ingress(ing)
		return in, nil
	} else if config.Device == "istio" {
		ing, err := GetIstioIngress(db, config)
		if err != nil {
			return nil, err
		}
		var in Ingress = Ingress(ing)
		return in, nil
	} else {
		return nil, errors.New("Unable to find ingress for " + config.Device)
	}
}

func GetSiteIngress(db *sql.DB, internal bool) (Ingress, error) {
	config, err := GetSitesIngressPublicExternal()
	if err != nil {
		return nil, err
	}
	if internal {
		config, err = GetAppsIngressPrivateInternal()
		if err != nil {
			return nil, err
		}
	}
	if config.Device == "f5" {
		ing, err := GetF5Ingress(db, config)
		if err != nil {
			return nil, err
		}
		var in Ingress = Ingress(ing)
		return in, nil
	} else if config.Device == "istio" {
		ing, err := GetIstioIngress(db, config)
		if err != nil {
			return nil, err
		}
		var in Ingress = Ingress(ing)
		return in, nil
	} else {
		return nil, errors.New("Unable to find ingress for " + config.Device)
	}
}
