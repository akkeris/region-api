package certs

import (
	"database/sql"
	"os"
	"region-api/router"
	"region-api/structs"
)

type Issuer interface {
	CreateOrder(domain string, sans []string, comment string, requestor string) (id string, err error)
	GetOrderStatus(id string) (order *structs.CertificateOrder, err error)
	GetOrders() (orders []structs.CertificateOrder, err error)
	IsOrderAutoInstalled(ingress router.Ingress) (bool, error)
	IsOrderReady(id string) (bool, error)
	GetCertificate(id string, domain string) (pem_cert []byte, pem_key []byte, err error)
}

func GetIssuer(db *sql.DB) (Issuer, error) {
	// TODO: Figure out a better way of managing who the certificate issuer is.
	if os.Getenv("DIGICERT_SECRET") == "" {
		lsIssuer, err := GetCertManagerIssuer(db)
		if err != nil {
			return nil, err
		}
		var in Issuer = Issuer(lsIssuer)
		return in, nil
	} else {
		dcIssuer, err := GetDigiCertIssuer(db)
		if err != nil {
			return nil, err
		}
		var in Issuer = Issuer(dcIssuer)
		return in, nil
	}
}
