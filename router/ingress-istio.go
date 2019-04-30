package router

import (
	"bytes"
	"bufio"
	"database/sql"
	structs "region-api/structs"
	"region-api/runtime"
	"encoding/base64"
	"encoding/pem"
	"text/template"
	"strings"
	"os"
	"net/http"
	"encoding/json"
	"errors"
	"fmt"
	"crypto/x509"
	"time"
	"strconv"
)


type SiteIstioVirtualService struct {
    APIVersion string `json:"apiVersion"`
    Kind       string `json:"kind"`
    Metadata   struct {
        Name            string `json:"name"`
        Namespace       string `json:"namespace"`
        ResourceVersion string `json:"resourceVersion"`
    } `json:"metadata"`
    Spec struct {
        Gateways []string `json:"gateways"`
        Hosts    []string `json:"hosts"`
        HTTP     []struct {
            Match []struct {
                URI struct {
                    Prefix string `json:"prefix"`
                } `json:"uri"`
            } `json:"match"`
            Rewrite struct {
                URI string `json:"uri"`
            } `json:"rewrite"`
            Route []struct {
                Destination struct {
                    Host string `json:"host"`
                    Port struct {
                        Number int `json:"number"`
                    } `json:"port"`
                } `json:"destination"`
            } `json:"route"`
        } `json:"http"`
    } `json:"spec"`
}

type HTTPSpec struct {
    Route []Routespec `json:"route"`
}

type Routespec struct {
    Destination struct {
        Host string `json:"host"`
        Port struct {
                Number int32 `json:"number"`
        } `json:"port"`
    } `json:"destination"`
}

type AppIstioVirtualService struct {
    APIVersion string `json:"apiVersion"`
    Kind       string `json:"kind"`
    Metadata   struct {
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
        ResourceVersion string `json:"resourceVersion"`
    } `json:"metadata"`
    Spec struct {
        Gateways []string   `json:"gateways"`
        Hosts    []string   `json:"hosts"`
        HTTP     []HTTPSpec `json:"http"`
    } `json:"spec"`
}

type IstioIngress struct {
	runtime runtime.Runtime
	config *IngressConfig
	db *sql.DB
}

type VSRV struct {
    Metadata   struct {
        ResourceVersion   string    `json:"resourceVersion"`
    } `json:"metadata"`
}

type Gateway struct {
    APIVersion string `json:"apiVersion"`
    Kind       string `json:"kind"`
    Metadata   struct {
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
        ResourceVersion string `json:"resourceVersion,omitempty"`
    } `json:"metadata"`
    Spec struct {
        Selector struct {
            Istio string `json:"istio"`
        } `json:"selector"`
        Servers []Server `json:"servers"`
    } `json:"spec"`
}

type Server struct {
    Hosts []string `json:"hosts"`
    Port  struct {
        Name     string `json:"name"`
        Number   int    `json:"number"`
        Protocol string `json:"protocol"`
    } `json:"port"`
    TLS struct {
		MinProtocolVersion string `json:"minProtocolVersion,omitempty"`
        CredentialName     string `json:"credentialName,omitempty"`
        Mode               string `json:"mode,omitempty"`
        PrivateKey         string `json:"privateKey,omitempty"`
        ServerCertificate  string `json:"serverCertificate,omitempty"`
        HttpsRedirect      bool   `json:"httpsRedirect,omitempty"`
    } `json:"tls,omitempty"`
}

type TLSSecretData struct {
	Certificate string
	Key string
	Name string
	CertName string
	Space string
}

var vstemplate = `{
    "apiVersion": "networking.istio.io/v1alpha3",
    "kind": "VirtualService",
    "metadata": {
        "name": "{{.Domain}}",
        "namespace": "{{.VSNamespace}}"
{{if .ResourceVersion}}
        , "resourceVersion": "{{.ResourceVersion}}"
{{end}}
    },
    "spec": {
        "gateways": [
            "{{ if .Internal }}sites-private{{else}}sites-public{{end}}"
        ],
        "hosts": [
            "{{.Domain}}"
        ],
        "http": [
{{$c := counter}}           
{{ range $value := .Paths }}
{{if call $c}}, {{end}}
            {
                "match": [
                    {
                        "uri": {
                            "prefix": "{{ removeslash $value.Path }}/"
                        }
                    }
                ],
                "rewrite": {
                    "uri": "{{$value.ReplacePath}}"
                },
                "route": [
                    {
                        "destination": {
                            "host": "{{$value.App}}.{{$value.Space}}.svc.cluster.local",
                            "port": {
                                "number": {{$value.Port}}
                            }
                        }
                    }
                ]
            }
            {{ if eq (removeslashslash $value.Path) (removeslash $value.Path) }},
            {
                "match": [
                    {
                        "uri": {
                            "prefix": "{{ removeslashslash $value.Path}}"
                        }
                    }
                ],
                "rewrite": {
                    "uri": "{{$value.ReplacePath}}"
                },
                "route": [
                    {
                        "destination": {
                            "host": "{{$value.App}}.{{$value.Space}}.svc.cluster.local",
                            "port": {
                                "number": {{$value.Port}}
                            }
                        }
                    }
                ]
            }{{end}}
{{end}}
        ]
    }
}
`

type kubernetesSecretTLS struct {
	ApiVersion string `json:"apiVersion"`
	Data struct {
		CaCrt string `json:"ca.crt,omitempty"`
		TlsCrt string `json:"tls.crt,omitempty"`
		TlsKey string `json:"tls.key,omitempty"`
	} `json:"data"`
	Kind string `json:"kind"`
	Metadata struct {
		Annotations struct {
			AltNames string `json:"akkeris.k8s.io/alt-names"`
			CommonNames string `json:"akkeris.k8s.io/common-name"`
		} `json:"annotations"`
		Labels struct {
			CertificateName string `json:"akkeris.k8s.io/certificate-name"`
		} `json:"labels"`
		Name string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Type string `json:"type"`
}

var tlsSecretTemplate = `
{
    "apiVersion": "v1",
    "data": {
        "ca.crt": null,
        "tls.crt": "{{.Certificate}}",
        "tls.key": "{{.Key}}"
    },
    "kind": "Secret",
    "metadata": {
        "annotations": {
            "akkeris.k8s.io/alt-names": "{{.Name}}",
            "akkeris.k8s.io/common-name": "{{.Name}}",
        },
        "labels": {
            "akkeris.k8s.io/certificate-name": "{{.Name}}"
        },
        "name": "{{.CertName}}",
        "namespace": "{{.Space}}",
    },
    "type": "kubernetes.io/tls"
}
`

func counter() func() int {
	i := -1
	return func() int {
		i++
		return i
	}
}

func appToService(input string) string {
	inputa := strings.Split(input, "-")
	app := strings.Join(inputa[:len(inputa)-1], "-")
	space := inputa[len(inputa)-1]
	return app + "." + space + ".svc.cluster.local"

}

func newHostToService(input string) string {
	domaina := strings.Split(input, ".")
	hostname := domaina[0]
	hostnamea := strings.Split(hostname, "-")
	app := strings.Join(hostnamea[:len(hostnamea)-1], "-")
	space := hostnamea[len(hostnamea)-1]
	return app + "." + space + ".svc.cluster.local"
}

func removeSlashSlash(input string) string {
	toreturn := strings.Replace(input, "//", "/", -1)
	if toreturn == "" {
		toreturn = "/"
	}
	return toreturn
}

func removeSlash(input string) string {
	if input == "/" {
		return ""
	}
	return input
}

func GetIstioIngress(db *sql.DB, config *IngressConfig) (*IstioIngress, error) {
	runtime, err := runtime.GetRuntimeStack(db, os.Getenv("DEFAULT_STACK"))
	// TODO: This is obvious we don't yet support multi-cluster regions.
	//       and this is an artifact of that, we shouldn't have a 'stack' our
	//       certificates or ingress are issued from.
	if err != nil {
		return nil, err
	}
	return &IstioIngress{
		runtime:runtime,
		config:config,
		db:db,
	}, nil
}

func (ingress *IstioIngress) VirtualServiceExists(domain string) (bool, string, error) {
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/" + domain, nil)
	if err != nil {
		return false, "", err
	}
	if code == http.StatusOK {
		var vsrv VSRV
		err = json.Unmarshal(body, &vsrv)
		if err != nil {
			return false, "", nil
		}
		return true, vsrv.Metadata.ResourceVersion, nil
	} else {
		return false, "", nil
	}
}

func (ingress *IstioIngress) GatewayExists(domain string) (bool, error) {
	newdomain := strings.Replace(domain, ".", "-", -1) + "-gateway"
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/" + newdomain, nil)
	if err != nil {
		return false, err
	}
	if code == http.StatusOK {
		return true, nil
	} else {
		return false, nil
	}
}

func (ingress *IstioIngress) DeleteVirtualService(domain string) (error) {
	_, _, err := ingress.runtime.GenericRequest("delete", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/" + domain, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) DeleteGateway(domain string) (error) {
	newdomain := strings.Replace(domain, ".", "-", -1) + "-gateway"
	_, _, err := ingress.runtime.GenericRequest("delete", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/" + newdomain, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) AppVirtualService(space string, app string) (*AppIstioVirtualService, error) {
	var vs *AppIstioVirtualService
	body, _, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/" + app + "-" + space, nil)
	if err != nil {
		return vs, err
	}
	if err = json.Unmarshal(body, &vs); err != nil {
		return vs, err
	}
	return vs, nil
}

func (ingress *IstioIngress) UpdateAppVirtualService(vs *AppIstioVirtualService, space string, app string) (error) {
	_, _, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/" + app + "-" + space, vs)
	if err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateVirtualService(router structs.Routerspec, sitevs *SiteIstioVirtualService, exists bool) error {
	var err error = nil
	if !exists {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices", sitevs)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create virtual service %#+v due to %s - %s\n", sitevs, strconv.Itoa(code), string(body))
			return errors.New("Unable to create virtual service " + sitevs.Metadata.Name + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/" + router.Domain, sitevs)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to update virtual service %#+v due to %s - %s\n", sitevs, strconv.Itoa(code), string(body))
			return errors.New("Unable to update virtual service " + sitevs.Metadata.Name + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateGateway(domain string, gateway *Gateway) error {
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/" + gateway.Metadata.Name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/" + gateway.Metadata.Name, gateway)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to update gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
			return errors.New("Unable to update gateway " + gateway.Metadata.Name + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways", gateway)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
			return errors.New("Unable to create gateway " + gateway.Metadata.Name + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateUberSiteGateway(domain string, certificate string, internal bool) error {
	var gateway Gateway
	gatewayType := "public"
	if internal {
		gatewayType = "private"
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/sites-" + gatewayType, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		err = json.Unmarshal(body, &gateway)
		if err != nil {
			return err
		}
	} else if code == http.StatusNotFound {
		// populate with defaults
		gateway.APIVersion = "networking.istio.io/v1alpha3"
		gateway.Kind = "Gateway"
		gateway.Metadata.Name = "sites-" + gatewayType
		gateway.Metadata.Namespace = "sites-system"
		gateway.Spec.Selector.Istio = "sites-" + gatewayType + "-ingressgateway"
	} else {
		return errors.New("Response from request for sites gateway did not make sense: " + strconv.Itoa(code) + " " + string(body))
	}

	// See if the gateway already has this host on the same certificate (if tls).
	//urlName := strings.Replace(strings.Replace(certificate, ".", "-", -1), "*", "star", -1)
	portName := "https-" + certificate
	tlsCredentialName := certificate
	var foundHttpsServerObj *Server = nil
	var foundHttpServerObj *Server = nil
	var hostHttpsFound = false
	var removeServerRecordPortName string = ""
	for _, server := range gateway.Spec.Servers {
		if server.Port.Number == 443 && server.TLS.CredentialName == tlsCredentialName {
			foundHttpsServerObj = &server
		}
		var removeHostRecord *string = nil
		for _, host := range server.Hosts {
			if host == domain {
				if server.Port.Number == 443 && server.TLS.CredentialName == tlsCredentialName {
					hostHttpsFound = true
				} else if server.Port.Number == 443 && server.TLS.CredentialName != tlsCredentialName {
					// Remove this host. And if its the only host, remove the server record.
					if len(server.Hosts) == 1 {
						// Remove server
						removeServerRecordPortName = server.Port.Name
					} else {
						// Remove host record
						removeHostRecord = &host
					}
				}
				if server.Port.Number == 80 {
					foundHttpServerObj = &server
				}
			}
		}
		if removeHostRecord != nil {
			var newHosts []string
			for _, host := range server.Hosts {
				if host != *removeHostRecord {
					newHosts = append(newHosts, host)
				}
			}
			server.Hosts = newHosts
		}
	}

	var dirty = false
	if removeServerRecordPortName != "" {
		var newServers []Server
		for _, server := range gateway.Spec.Servers {
			if server.Port.Name != removeServerRecordPortName {
				newServers = append(newServers, server)
			}
		}
		gateway.Spec.Servers = newServers
		dirty = true
	}
	if foundHttpsServerObj == nil {
		var httpsServer Server
		httpsServer.Hosts = append(httpsServer.Hosts, domain)
		httpsServer.Port.Name = portName
		httpsServer.Port.Number = 443
		httpsServer.Port.Protocol = "HTTPS"
		httpsServer.TLS.CredentialName = tlsCredentialName
		httpsServer.TLS.MinProtocolVersion = "TLSV1_2"
		httpsServer.TLS.Mode = "SIMPLE"
		httpsServer.TLS.PrivateKey = "/etc/istio/" + certificate + "/tls.key"
		httpsServer.TLS.ServerCertificate = "/etc/istio/" + certificate + "/tls.crt"
		gateway.Spec.Servers = append(gateway.Spec.Servers, httpsServer)
		dirty = true
	} else if foundHttpsServerObj != nil && !hostHttpsFound {
		for i, server := range gateway.Spec.Servers {
			if server.Port.Name == foundHttpsServerObj.Port.Name {
				gateway.Spec.Servers[i].Hosts = append(server.Hosts, domain)
			}
		}
		dirty = true
	}

	if foundHttpServerObj != nil {
		var httpServer Server
		httpServer.Hosts = append(httpServer.Hosts, domain)
		httpServer.Port.Name = "http-" + strings.Replace(domain, ".", "-", -1)
		httpServer.Port.Number = 80
		httpServer.Port.Protocol = "HTTP"
		httpServer.TLS.HttpsRedirect = true
		gateway.Spec.Servers = append(gateway.Spec.Servers, httpServer)
		dirty = true
	}

	if dirty {
		return ingress.InstallOrUpdateGateway(domain, &gateway)
	}
	return nil
}


// Standard Ingress Methods:

/*
 * We need to replicate the behavior of the F5, when installing a new site,
 * can likely assume that a certificate is installed, we can look for a list
 * of appropriate certificates and the most specific, unexpired one wins.
 */

func (ingress *IstioIngress) CreateOrUpdateRouter(router structs.Routerspec) (error) {
	exists, version, err := ingress.VirtualServiceExists(router.Domain);
	if err != nil {
		return err
	}
	for index, _ := range router.Paths {
		if os.Getenv("DEFAULT_PORT") == "" {
			router.Paths[index].Port = "80"
		} else {
			router.Paths[index].Port = os.Getenv("DEFAULT_PORT")
		}
	}
	router.VSNamespace = "sites-system"
	var t *template.Template
	funcMap := template.FuncMap{
		"replace":          strings.Replace,
		"counter":          counter,
		"apptoservice":     appToService,
		"newhosttoservice": newHostToService,
		"removeslashslash": removeSlashSlash,
		"removeslash":      removeSlash,
	}
	if exists {
		router.ResourceVersion = version
	}
	t = template.Must(template.New("vstemplate").Funcs(funcMap).Parse(vstemplate))
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	if err := t.Execute(wr, router); err != nil {
		return err
	}
	wr.Flush()
	var sitevs SiteIstioVirtualService
	if err = json.Unmarshal([]byte(strings.Replace(string(b.Bytes()), "\n", " ", -1)), &sitevs); err != nil {
		return err
	}

	// See if any certificates are available, search is in this order:
	//
	// 1. See if a direct certificate exists for the domain name.
	// 2. See if there's a wildcard certificate installed.
	// 3. Default to the star certificate and hope it works.
	//
	var certName = "star-certificate"
	certs, err := ingress.GetInstalledCertificates(router.Domain)
	if err == nil && len(certs) > 0 {
		certName = strings.Replace(strings.Replace(router.Domain, ".", "-", -1), "*", "star", -1) + "-tls"
	} else {
		starCert := "*." + strings.Join(strings.Split(router.Domain, ".")[1:], ".")
		certs, err = ingress.GetInstalledCertificates(starCert)
		if err == nil  && len(certs) > 0 {
			certName = strings.Replace(strings.Replace(starCert, ".", "-", -1), "*", "star", -1) + "-tls"
		}
	}
	if err = ingress.InstallOrUpdateUberSiteGateway(router.Domain, certName, router.Internal); err != nil {
		return err
	}
	return ingress.InstallOrUpdateVirtualService(router, &sitevs, exists)
}

func (ingress *IstioIngress) SetMaintenancePage(app string, space string, value bool) (error) {
	virtualService, err := ingress.AppVirtualService(space, app)
	if err != nil {
		return err
	}
	if len(virtualService.Spec.HTTP) == 0 || len(virtualService.Spec.HTTP[0].Route) == 0 {
		return errors.New("The specified maintenance page could not be found or did not have a routable virtual service.")
	}
	if value {
		virtualService.Spec.HTTP[0].Route[0].Destination.Host = "akkeris-404-page.default.svc.cluster.local"
	} else {
		virtualService.Spec.HTTP[0].Route[0].Destination.Host = app + "." + space + ".svc.cluster.local"
	}
	if err = ingress.UpdateAppVirtualService(virtualService, space, app); err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) GetMaintenancePageStatus(app string, space string) (bool, error) {
	virtualService, err := ingress.AppVirtualService(space, app)
	if err != nil {
		return false, err
	}
	if len(virtualService.Spec.HTTP) == 0 || len(virtualService.Spec.HTTP[0].Route) == 0 {
		return false, errors.New("The specified maintenance page could not be found or did not have a routable virtual service.")
	}
	if virtualService.Spec.HTTP[0].Route[0].Destination.Host == "akkeris-404-page.default.svc.cluster.local" {
		return true, nil
	} else {
		return false, nil 
	}
}

func (ingress *IstioIngress) DeleteRouter(router structs.Routerspec) (error) {
	return ingress.DeleteVirtualService(router.Domain)
	return ingress.DeleteGateway(router.Domain)
}

func (ingress *IstioIngress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) (error) {
	block, _ := pem.Decode([]byte(pem_cert))
	if block == nil {
		fmt.Println("failed to parse PEM block containing the public certificate")
		return errors.New("Invalid certificate: Failed to decode PEM block")
	}
	x509_decoded_cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Println("invalid certificate provided")
		fmt.Println(err)
		return err
	}
	block, _ = pem.Decode([]byte(pem_key))
	if block == nil {
		fmt.Println("failed to parse PEM block containing the private key")
		return errors.New("Invalid key: Failed to decode PEM block")
	}
	x509_decoded_key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Println("invalid key provided")
		fmt.Println(err)
		return err
	}
	err = x509_decoded_key.Validate()
	if err != nil {
		fmt.Println("x509 decoded key was invalid")
		fmt.Println(err)
		return err
	}

	main_server_name := strings.Replace(x509_decoded_cert.Subject.CommonName, "*.", "star.", -1)
	main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"

	var t *template.Template
	t = template.Must(template.New("tlsSecretTemplate").Parse(tlsSecretTemplate))
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	tlsSecret := TLSSecretData{
		Certificate:base64.StdEncoding.EncodeToString(pem_cert),
		Key:base64.StdEncoding.EncodeToString(pem_key),
		Name:main_server_name,
		CertName:main_certs_name,
		Space:ingress.config.Environment,
	}
	if err := t.Execute(wr, tlsSecret); err != nil {
		return err
	}
	wr.Flush()

	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}

	_, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/" + certificateNamespace + "/secrets/" + main_certs_name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		_, _, err = ingress.runtime.GenericRequest("put", "/api/v1/namespaces/" + certificateNamespace + "/secrets/" + main_certs_name, b.Bytes())
		return err
	} else {
		_, _, err = ingress.runtime.GenericRequest("post", "/api/v1/namespaces/" + certificateNamespace + "/secrets", b.Bytes())
		return err
	}
}

func (ingress *IstioIngress) GetInstalledCertificates(site string) ([]Certificate, error) {
	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}

	main_server_name := strings.Replace(site, "*.", "star.", -1)
	main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"
	body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/" + certificateNamespace + "/secrets/" + main_certs_name, nil)

	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return []Certificate{}, nil
	} else if code != http.StatusOK {
		return nil, errors.New("Failure to lookup certificate: " + string(body))
	}
	var t kubernetesSecretTLS 
	if err = json.Unmarshal(body, &t); err != nil {
		return nil, err
	}
	pem_certs, err := base64.StdEncoding.DecodeString(t.Data.TlsCrt)
	if err != nil {
		return nil, err
	}
	x509_decoded_cert, _, _, err := DecodeCertificateBundle(site, pem_certs);
	if err != nil {
		return nil, err
	}

	exists, err := ingress.GatewayExists(site)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []Certificate{}, nil
	}
	exists, _, err = ingress.VirtualServiceExists(site)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []Certificate{}, nil
	}
	var certType string = "normal"
	var altNames []string = strings.Split(t.Metadata.Annotations.AltNames, ",")
	if len(altNames) > 1 {
		certType = "sans"
	}
	for _, n := range altNames {
		if strings.Contains(n, "*") {
			certType = "wildcard"
		}
	}

	return []Certificate{Certificate{
		Type: certType,
		Name: main_certs_name,
		Expires: x509_decoded_cert.NotAfter.Unix(),
		Alternatives: altNames,
		Expired: !x509_decoded_cert.NotAfter.Before(time.Now()),
		Address: ingress.config.Address,
	}}, nil
}

func (ingress *IstioIngress) Config() *IngressConfig {
	return ingress.config
}

func (ingress *IstioIngress) Name() string {
	return "istio"
}
