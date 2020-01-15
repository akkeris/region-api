package certs

import (
	"database/sql"
	"region-api/router"
	"region-api/runtime"
	"region-api/structs"
	"errors"
)

type Issuer interface {
	GetName() string
	CreateOrder(domain string, sans []string, comment string, requestor string) (id string, err error)
	GetOrderStatus(id string) (order *structs.CertificateOrder, err error)
	GetOrders() (orders []structs.CertificateOrder, err error)
	IsOrderAutoInstalled(ingress router.Ingress) (bool, error)
	IsOrderReady(id string) (bool, error)
	GetCertificate(id string, domain string) (pem_cert []byte, pem_key []byte, err error)
	DeleteCertificate(name string) (error)
}

func GetIssuers(db *sql.DB) ([]Issuer, error) {
	runtimes, err := runtime.GetAllRuntimes(db)
	// TODO: This is obvious we don't yet support multi-cluster regions.
	//       and this is an artifact of that, we shouldn't have a 'stack' our
	//       certificates or ingress are issued from.
	if err != nil {
		return nil, err
	}
	if len(runtimes) == 0 {
		return nil, errors.New("No runtime was found.")
	}
	// For now we only support cert-manager Issuer types, we could support others outside of
	// what cert manager/acme supports but its not yet implemented.
	return GetCertManagerIssuers(runtimes[0]);
}


func GetIssuer(db *sql.DB, name string) (Issuer, error) {
	issuers, err := GetIssuers(db)
	if err != nil {
		return nil, err
	}
	for _, x := range issuers {
		if name == x.GetName() {
			return x, nil
		}
	}
	return nil, errors.New("Unable to find issuer by name " + name)
}
