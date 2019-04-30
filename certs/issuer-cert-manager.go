package certs

import (
	"database/sql"
	"region-api/structs"
	"region-api/runtime"
	"region-api/router"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"errors"
	"os"
	"strings"
	"github.com/nu7hatch/gouuid"
	"strconv"
)

type certManagerACMEConfig struct {
	DNS01 struct {
	        Provider string `json:"provider"`
	} `json:"dns01"`
	Domains []string `json:"domains"`
}

type certManagerACMECertificate struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
		Annotations struct {
			Comments string `json:"comments,omitempty"`
			Requestor string `json:"requestor,omitempty"`
		} `json:"annotations,omitempty"`
		Labels struct {
			Id string `json:"akkeris-cert-id,omitempty"`
		} `json:"labels,omitempty"`
	} `json:"metadata"`
	Spec struct {
        Acme struct {
            Config []certManagerACMEConfig `json:"config"`
        } `json:"acme"`
        CommonName string   `json:"commonName"`
        DNSNames   []string `json:"dnsNames"`
        IssuerRef  struct {
            Kind string `json:"kind"`
            Name string `json:"name"`
        } `json:"issuerRef"`
        SecretName string `json:"secretName"`
	} `json:"spec"`
}

type certManagerCertificateStatus struct {
	Metadata   struct {
		Annotations struct {
			Comments string `json:"comments,omitempty"`
			Requestor string `json:"requestor,omitempty"`
		} `json:"annotations,omitempty"`
		Labels struct {
			Id string `json:"akkeris-cert-id,omitempty"`
		} `json:"labels,omitempty"`
	} `json:"metadata"`
	Spec struct {
    	CommonName	string 		`json:"commonName"`
    	DNSNames	[]string	`json:"dnsNames"`
        IssuerRef  struct {
            Kind string `json:"kind"`
            Name string `json:"name"`
        } `json:"issuerRef"`
        SecretName string `json:"secretName"`
    } `json:"spec"`
    Status struct {
        Conditions []struct {
            Message            string    `json:"message"`
            Reason             string    `json:"reason"`
            Status             string    `json:"status"`
            Type               string    `json:"type"`
        } `json:"conditions"`
    } `json:"status"`
    
}

type certManagerCertificateStatusList struct {
	Items []certManagerCertificateStatus `json:"items"`
}

type kubeTlsSecret struct {
	Kind string `json:"kind"`
	Type string `json:"type"` //"type":"kubernetes.io/tls"
	Metadata struct {
		Name string	`json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Data struct {
		CaCrt *string `json:"ca.crt,omitempty"`
		Crt *string `json:"tls.crt,omitempty"`
		Key *string `json:"tls.key,omitempty"` 
	} `json:"data"`
}

type CertManagerIssuer struct {
	runtime runtime.Runtime
	issuerName string
	certificateNamespace string
	providerName string
}


func GetCertManagerIssuer(db *sql.DB) (*CertManagerIssuer, error) {
	issuerName := os.Getenv("CERTMANAGER_ISSUER_NAME")
	if issuerName == "" {
		issuerName = "letsencrypt"
	}
	providerName := os.Getenv("CERTMANAGER_PROVIDER_NAME")
	if providerName == "" {
		providerName = "aws"
	}
	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}
	runtime, err := runtime.GetRuntimeStack(db, os.Getenv("DEFAULT_STACK"))
	// TODO: This is obvious we don't yet support multi-cluster regions.
	//       and this is an artifact of that, we shouldn't have a 'stack' our
	//       certificates or ingress are issued from.
	if err != nil {
		return nil, err
	}
	return &CertManagerIssuer{
		issuerName:issuerName,
		runtime:runtime,
		certificateNamespace:certificateNamespace,
		providerName:providerName,
	}, nil
}

func CertificateStatusToOrder(status certManagerCertificateStatus) (structs.CertificateOrder, error) {
	s := "pending"
	if len(status.Status.Conditions) > 0 && status.Status.Conditions[0].Type == "Ready" && status.Status.Conditions[0].Status == "True" {
		s = "issued"
	}
	names := make([]string, 0)
	for _, d := range status.Spec.DNSNames {
		if d != status.Spec.CommonName {
			names = append(names, d)
		}
	}
	return structs.CertificateOrder{
		Id: status.Metadata.Labels.Id,
		CommonName: status.Spec.CommonName,
		SubjectAlternativeNames: names,
		Status: s,
		Comment: status.Metadata.Annotations.Comments,
		Requestor: status.Metadata.Annotations.Requestor,
	}, nil
}

func CertificateStatusesToOrders(statuses []certManagerCertificateStatus) ([]structs.CertificateOrder, error) {
	orders := make([]structs.CertificateOrder, 0)
	for _, e := range statuses {
		o, err := CertificateStatusToOrder(e)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (issuer *CertManagerIssuer) CreateOrder(domain string, sans []string, comment string, requestor string) (id string, err error) {
	var cert certManagerACMECertificate
	cert.APIVersion = "certmanager.k8s.io/v1alpha1"
	cert.Kind = "Certificate"
	cert.Metadata.Name = domain
	cert.Metadata.Namespace = issuer.certificateNamespace
	cert.Metadata.Annotations.Comments = comment
	cert.Metadata.Annotations.Requestor = requestor
	newuuid, _ := uuid.NewV4()
	cert.Metadata.Labels.Id = newuuid.String()
	var ac certManagerACMEConfig
	ac.Domains = append(ac.Domains, domain)
	ac.Domains = append(ac.Domains, sans...)
	ac.DNS01.Provider = issuer.providerName
	cert.Spec.Acme.Config = append(cert.Spec.Acme.Config, ac)
	cert.Spec.CommonName = domain
	cert.Spec.DNSNames = append(cert.Spec.DNSNames, domain)
	cert.Spec.DNSNames = append(cert.Spec.DNSNames, sans...)
	cert.Spec.IssuerRef.Kind = "ClusterIssuer"
	cert.Spec.IssuerRef.Name = issuer.issuerName
	cert.Spec.SecretName = strings.Replace(domain, ".", "-", -1) + "-tls"
	body, code, err := issuer.runtime.GenericRequest("post", "/apis/certmanager.k8s.io/v1alpha1/namespaces/" + issuer.certificateNamespace + "/certificates", cert)
	if err != nil {
		return cert.Metadata.Labels.Id, err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", errors.New("Unable to create new certificate. (" + string(body) + " [" + strconv.Itoa(code) + "])")
	}
	return cert.Metadata.Labels.Id, nil
}


func (issuer *CertManagerIssuer) GetOrderStatus(id string) (*structs.CertificateOrder, error) {
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/certmanager.k8s.io/v1alpha1/namespaces/" + issuer.certificateNamespace + "/certificates?labelSelector=akkeris-cert-id%3D" + id, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatusList certManagerCertificateStatusList
	if err = json.Unmarshal(body, &certStatusList); err != nil {
		return nil, err
	}
	if len(certStatusList.Items) != 1 {
		return nil, errors.New("More than one (or none) certificates were returned.")
	}
	order, err := CertificateStatusToOrder(certStatusList.Items[0])
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (issuer *CertManagerIssuer) GetOrders() (orders []structs.CertificateOrder, err error) {
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/certmanager.k8s.io/v1alpha1/namespaces/" + issuer.certificateNamespace + "/certificates?labelSelector=akkeris-cert-id", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatus certManagerCertificateStatusList
	err = json.Unmarshal(body, &certStatus)
	if err != nil {
		return nil, err
	}
	return CertificateStatusesToOrders(certStatus.Items)
}

func (issuer *CertManagerIssuer) IsOrderAutoInstalled(ingress router.Ingress) (bool, error) {
	if ingress.Name() == "istio" {
		return true, nil
	} else {
		return false, nil
	}
}

func (issuer *CertManagerIssuer) IsOrderReady(id string) (bool, error) {
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/certmanager.k8s.io/v1alpha1/namespaces/" + issuer.certificateNamespace + "/certificates?labelSelector=akkeris-cert-id%3D" + id, nil)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatusList certManagerCertificateStatusList
	if err = json.Unmarshal(body, &certStatusList); err != nil {
		return false, err
	}
	if len(certStatusList.Items) != 1 {
		return false, errors.New("More than one (or none) certificates were returned.")
	}
	cs := certStatusList.Items[0]
	if len(cs.Status.Conditions) == 0 {
		return false, err
	}
	if cs.Status.Conditions[0].Type == "Ready" && cs.Status.Conditions[0].Status == "True" {
		return true, nil
	} else {
		return false, nil
	}
}


func (issuer *CertManagerIssuer) GetCertificate(id string, domain string) (pem_cert []byte, pem_key []byte, err error) {
	name := strings.Replace(domain, "*.", "star.", -1)
	name = strings.Replace(name, ".", "-", -1) + "-tls"
	body, code, err := issuer.runtime.GenericRequest("get", "/api/v1/namespaces/" + issuer.certificateNamespace + "/secrets/" + name, nil)
	if err != nil {
		return nil, nil, err
	}
	if code != http.StatusOK {
		return nil, nil, errors.New("Certificate not found.")
	}
	var secret kubeTlsSecret
	if err = json.Unmarshal(body, &secret); err != nil {
		return nil, nil, err
	}
	if secret.Data.Crt == nil {
		return nil, nil, errors.New("Unable to decode or get certificate, the tls.crt field was null")
	}
	if secret.Data.Key == nil {
		return nil, nil, errors.New("Unable to decode or get certificate, the tls.key field was null")
	}
	pem_cert, err = base64.StdEncoding.DecodeString(*secret.Data.Crt)
	if err != nil {
		return nil, nil, err
	}
	pem_key, err = base64.StdEncoding.DecodeString(*secret.Data.Key)
	if err != nil {
		return nil, nil, err
	}
	return pem_cert, pem_key, nil
}
