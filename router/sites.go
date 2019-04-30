package router

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	utils "region-api/utils"
	"strings"
	"crypto/x509"
	"fmt"
	"errors"
	"encoding/pem"
)

type Addresses struct {
	Network string	`json:"network"`
	Internal bool 	`json:"internal"`
	Address string 		`json:"address"`
}

type Domains struct {
	Internal bool `json:"internal"`
	Value string `json:"value"`
	Addresses Addresses `json:"ips"`
	Type string `json:"type"`
}

type Certificate struct {
	Alternatives []string `json:"alternatives"`
	Name string `json:"name"`
	Expires int64 `json:"expires"`
	Type string `json:"type"`
	Expired bool `json:"expired"`
	Address string `json:"address"`
}

type SiteStatus struct {
	Public bool `json:"public"`
	DomainRecordPointingToSite bool `json:"domain_record_pointing_to_site"`
	CertificateExists bool `json:"certificate_exists"`
	CertificateIsInstalled bool `json:"certificate_is_installed"`
	CertificateIsNotExpired bool `json:"certificate_is_not_expired"`
	Valid bool `json:"valid"`
}

type Site struct {
	Name string `json:"name"`
	Addresses []Addresses `json:"ips"`
	Domains []Domain `json:"domains"`
	DomainRecords []DomainRecord `json:"records"`
	Certificates []Certificate `json:"certificates"`
	Status []SiteStatus `json:"status"`
}

func DecodeCertificateBundle(server_name string, pem_certs []byte) (x509_decoded_cert *x509.Certificate, pem_cert []byte, pem_chain []byte, err error) {
	x509_decoded_certs := make([]*x509.Certificate, 0)
	var block *pem.Block = nil
	// Sanity checks before we begin installing, ensure the certifices
	// and keys are valid in addition to their names include the target
	// server_name.
	for ok := true; ok; ok = (block != nil && len(pem_certs) > 0) {
		block, pem_certs = pem.Decode(pem_certs)
		if block == nil {
			fmt.Println("failed to parse PEM block containing the public certificate")
			return nil, nil, nil, errors.New("Invalid certificate: Failed to decode PEM block")
		}
		c, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			fmt.Println("invalid certificates provided")
			fmt.Println(err)
			return nil, nil, nil, err
		}
		x509_decoded_certs = append(x509_decoded_certs, c...)
	}
	pem_chain = make([]byte, 0)
	for _, cert := range x509_decoded_certs {
		if cert.Subject.CommonName == server_name {
			x509_decoded_cert = cert
		} else {
			data := pem.EncodeToMemory(&pem.Block{
				Type:    "CERTIFICATE",
				Headers: nil,
				Bytes:   cert.Raw,
			})
			pem_chain = append(pem_chain, data...)
		}
	}
	if x509_decoded_cert == nil {
		return nil, nil, nil, errors.New("Unable to find certificate in bundle!")
	}
	pem_cert = pem.EncodeToMemory(&pem.Block{
		Type:    "CERTIFICATE",
		Headers: nil,
		Bytes:   x509_decoded_cert.Raw,
	})
	return x509_decoded_cert, pem_cert, pem_chain, nil
}

func WildCardMatch(wildcard string, domain string) bool {
	if wildcard == "*" {
		return true
	} else if strings.Contains(wildcard, "*") {
		var d = strings.Split(domain, ".")
		var w = strings.Split(strings.Replace(strings.Replace(wildcard, "*.", "", 1), "*", "", 1), ".")
		if len(d) > len(w) {
			d = d[len(d)-len(w):]
		}
		if strings.Join(d, ".") == strings.Join(w, ".") {
			return true
		}
	} else if wildcard == domain {
		return true
	}
	return false
}

func HttpGetSite(db *sql.DB, params martini.Params, r render.Render) {
	var site = strings.Trim(strings.ToLower(params["site"]), " ")
	var domain = strings.Split(site, ".")

	internalIngress, err := GetSiteIngress(db, false)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	internalCertificates, err := internalIngress.GetInstalledCertificates(site)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	externalIngress, err := GetSiteIngress(db, true)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	externalCertificates, err := externalIngress.GetInstalledCertificates(site)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	siteStatusPublic := SiteStatus{Public:true, DomainRecordPointingToSite:false, CertificateExists:false, CertificateIsInstalled:false, CertificateIsNotExpired: false, Valid:false}
	siteStatusPrivate := SiteStatus{Public:false, DomainRecordPointingToSite:false, CertificateExists:false, CertificateIsInstalled:false, CertificateIsNotExpired: false, Valid:false}
	for _, cert := range internalCertificates {
		siteStatusPrivate.CertificateExists = true
		siteStatusPrivate.CertificateIsInstalled = true
		if !cert.Expired {
			siteStatusPrivate.CertificateIsNotExpired = true
		}
	}
	for _, cert := range externalCertificates {
		siteStatusPublic.CertificateExists = true
		siteStatusPublic.CertificateIsInstalled = true
		if !cert.Expired {
			siteStatusPublic.CertificateIsNotExpired = true
		}
	}

	dns := GetDnsProvider()
	domainRecords := make([]DomainRecord, 0)
	if len(domain) > 2 {
		domain = domain[len(domain)-2:]
	}
	dzones, err := dns.Domain(strings.Join(domain, "."))
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	for _, dz := range dzones {
		records, err := dns.DomainRecords(dz)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, record := range records {
			for _, ip := range record.Values {
				if WildCardMatch(record.Name, site) {
					if dz.Public && ip == externalIngress.Config().Address {
						siteStatusPublic.DomainRecordPointingToSite = true
					} else if !dz.Public && ip == internalIngress.Config().Address {
						siteStatusPrivate.DomainRecordPointingToSite = true
					}
					domainRecords = append(domainRecords, record)
				}	
			}
		}
	}

	if siteStatusPublic.DomainRecordPointingToSite && siteStatusPublic.CertificateExists && siteStatusPublic.CertificateIsInstalled && siteStatusPublic.CertificateIsNotExpired {
		siteStatusPublic.Valid = true
	}
	if siteStatusPrivate.DomainRecordPointingToSite && siteStatusPrivate.CertificateExists && siteStatusPrivate.CertificateIsInstalled && siteStatusPrivate.CertificateIsNotExpired {
		siteStatusPrivate.Valid = true
	}

	internalAddress := Addresses{Network:"", Internal:true, Address:internalIngress.Config().Address}
	externalAddress := Addresses{Network:"", Internal:false, Address:externalIngress.Config().Address}

	r.JSON(http.StatusOK, Site{
		Name:site,
		Domains:dzones,
		DomainRecords:domainRecords,
		Addresses:[]Addresses{internalAddress, externalAddress},
		Certificates:append(internalCertificates, externalCertificates...),
		Status:[]SiteStatus{siteStatusPublic, siteStatusPrivate},
	})
}
