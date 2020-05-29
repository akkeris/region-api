package certs

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"region-api/router"
	"region-api/runtime"
	"strconv"
	"strings"

	certmanager "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	uuid "github.com/nu7hatch/gouuid"
	kube "k8s.io/api/core/v1"
	kubemetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CertManagerIssuer struct {
	runtime              runtime.Runtime
	certificateNamespace string
	clusterIssuers       []certmanager.ClusterIssuer
}

func GetCertManagerIssuers(runtime runtime.Runtime) ([]Issuer, error) {
	body, code, err := runtime.GenericRequest("get", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/clusterissuers?limit=500", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("Unable to find certificate: " + string(body))
	}
	var certManagerIssuers certmanager.ClusterIssuerList
	if err = json.Unmarshal(body, &certManagerIssuers); err != nil {
		return nil, err
	}
	namespace := os.Getenv("CERT_NAMESPACE")
	if namespace == "" {
		namespace = "istio-system"
	}
	return []Issuer{&CertManagerIssuer{
		runtime:              runtime,
		certificateNamespace: namespace,
		clusterIssuers:       certManagerIssuers.Items,
	}}, nil
}

func CertificateStatusToOrder(certificate certmanager.Certificate) (CertificateOrder, error) {
	s := "pending"
	if len(certificate.Status.Conditions) > 0 && certificate.Status.Conditions[0].Type == "Ready" && certificate.Status.Conditions[0].Status == "True" {
		s = "issued"
	}
	names := make([]string, 0)
	for _, d := range certificate.Spec.DNSNames {
		if d != certificate.Spec.CommonName {
			names = append(names, d)
		}
	}
	annotations := certificate.GetAnnotations()
	labels := certificate.GetLabels()
	var comments string = ""
	var requestor string = ""
	var id string = ""
	var create = certificate.GetCreationTimestamp().UTC().String()

	var expires = ""
	if certificate.Status.NotAfter != nil {
		expires = certificate.Status.NotAfter.UTC().String()
	}
	if annotations != nil {
		comments = annotations["comments"]
		requestor = annotations["requestor"]
	}
	if labels != nil {
		id = labels["akkeris-cert-id"]
	}
	return CertificateOrder{
		Id:                      id,
		CommonName:              certificate.Spec.CommonName,
		SubjectAlternativeNames: names,
		Status:                  s,
		Comment:                 comments,
		Requestor:               requestor,
		Issued:                  create,
		Expires:                 expires,
	}, nil
}

func CertificateStatusesToOrders(certificates []certmanager.Certificate) ([]CertificateOrder, error) {
	orders := make([]CertificateOrder, 0)
	for _, e := range certificates {
		o, err := CertificateStatusToOrder(e)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (issuer *CertManagerIssuer) GetName() string {
	return "cert-manager"
}

func (issuer *CertManagerIssuer) CreateOrder(domain string, sans []string, comment string, requestor string, issuerName string) (id string, err error) {
	var found = false
	for _, cIssuer := range issuer.clusterIssuers {
		if cIssuer.GetName() == issuerName {
			found = true
		}
	}
	if found == false {
		return "", errors.New("Unable to find issuer " + issuerName)
	}
	var cert certmanager.Certificate
	cert.APIVersion = certmanager.SchemeGroupVersion.Group + "/" + certmanager.SchemeGroupVersion.Version
	cert.Kind = "Certificate"
	cert.SetName(strings.Replace(domain, "*", "star", -1))
	cert.SetNamespace(issuer.certificateNamespace)
	cert.SetAnnotations(map[string]string{"comment": comment, "requestor": requestor})
	u, _ := uuid.NewV4()
	cert.SetLabels(map[string]string{"akkeris-cert-id": u.String()})
	cert.Spec.DNSNames = make([]string, 0)
	cert.Spec.DNSNames = append(cert.Spec.DNSNames, domain)
	cert.Spec.DNSNames = append(cert.Spec.DNSNames, sans...)
	// DefaultRenewBefore is defined as time.Hour * 24 * 30
	cert.Spec.RenewBefore = &kubemetav1.Duration{Duration: certmanager.DefaultRenewBefore}
	cert.Spec.CommonName = domain
	cert.Spec.IssuerRef.Kind = "ClusterIssuer"
	cert.Spec.IssuerRef.Name = issuerName
	cert.Spec.SecretName = strings.Replace(strings.Replace(domain, "*", "star", -1), ".", "-", -1) + "-tls"
	body, code, err := issuer.runtime.GenericRequest("post", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/namespaces/"+issuer.certificateNamespace+"/certificates", cert)
	if err != nil {
		return cert.GetLabels()["akkeris-cert-id"], err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", errors.New("Unable to create new certificate. (" + string(body) + " [" + strconv.Itoa(code) + "])")
	}
	return cert.GetLabels()["akkeris-cert-id"], nil
}

func (issuer *CertManagerIssuer) GetOrderStatus(id string) (*CertificateOrder, error) {
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/namespaces/"+issuer.certificateNamespace+"/certificates?labelSelector=akkeris-cert-id%3D"+id, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatusList certmanager.CertificateList
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

func (issuer *CertManagerIssuer) GetOrders() (orders []CertificateOrder, err error) {
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/namespaces/"+issuer.certificateNamespace+"/certificates?labelSelector=akkeris-cert-id", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatus certmanager.CertificateList
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
	body, code, err := issuer.runtime.GenericRequest("get", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/namespaces/"+issuer.certificateNamespace+"/certificates?labelSelector=akkeris-cert-id%3D"+id, nil)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, errors.New("Unable to find certificate: " + string(body))
	}
	var certStatusList certmanager.CertificateList
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
	body, code, err := issuer.runtime.GenericRequest("get", "/api/v1/namespaces/"+issuer.certificateNamespace+"/secrets/"+name, nil)
	if err != nil {
		return nil, nil, err
	}
	if code != http.StatusOK {
		return nil, nil, errors.New("Certificate not found.")
	}
	var secret kube.Secret
	if err = json.Unmarshal(body, &secret); err != nil {
		return nil, nil, err
	}
	if secret.Data["tls.crt"] == nil {
		return nil, nil, errors.New("Unable to decode or get certificate, the tls.crt field was null")
	}
	if secret.Data["tls.key"] == nil {
		return nil, nil, errors.New("Unable to decode or get certificate, the tls.key field was null")
	}
	return secret.Data["tls.crt"], secret.Data["tls.key"], nil
}

// Used by unit tests, shoudn't be used outside of that.
func (issuer *CertManagerIssuer) DeleteCertificate(name string) error {
	_, code, err := issuer.runtime.GenericRequest("delete", "/apis/"+certmanager.SchemeGroupVersion.Group+"/"+certmanager.SchemeGroupVersion.Version+"/namespaces/"+issuer.certificateNamespace+"/certificates/"+name, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return errors.New("Certificate not found.")
	}
	return nil
}
