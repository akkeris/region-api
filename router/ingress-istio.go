package router

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"region-api/runtime"
	structs "region-api/structs"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type HeaderOperationsspec struct {
	Set    map[string]string `json:"set,omitempty"`
	Add    map[string]string `json:"add,omitempty"`
	Remove []string          `json:"remove,omitempty"`
}

type Headersspec struct {
	Request  HeaderOperationsspec `json:"request,omitempty"`
	Response HeaderOperationsspec `json:"response,omitempty"`
}

type Matchspec struct {
	URI struct {
		Prefix string `json:"prefix"`
	} `json:"uri"`
}

type Rewritespec struct {
	URI string `json:"uri,omitempty"`
}

type Routespec struct {
	Destination struct {
		Host string `json:"host"`
		Port struct {
			Number int32 `json:"number"`
		} `json:"port"`
	} `json:"destination"`
}

type HTTPSpec struct {
	Match   []Matchspec `json:"match"`
	Route   []Routespec `json:"route"`
	Rewrite *Rewritespec `json:"rewrite,omitempty"`
	Headers *Headersspec `json:"headers,omitempty"`
}

type StringMatch struct {
	Exact string `json:"exact,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
	Regex string `json:"regex,omitempty"`
}

type TriggerRule struct {
	ExcludedPaths []StringMatch `json:"excludedPaths,omitempty"`
	IncludedPaths []StringMatch `json:"includedPaths,omitempty"`
}

type Jwt struct {
	Issuer string `json:"issuer,omitempty"`
	Audiences []string `json:"audiences,omitempty"`
	JwksUri string `json:"jwksUri"`
	TriggerRules []TriggerRule `json:"triggerRules,omitempty"` 
}

type OriginAuthenticationMethod struct {
	Jwt Jwt `json:"jwt"`
}

type TargetSelector struct {
	Name string `json:"name"`
}

type Policy struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Origins []OriginAuthenticationMethod `json:"origins"`
		PrincipalBinding string `json:"principalBinding"` /* Can be USE_PEER or USE_ORIGIN, set to USE_ORIGIN */
		Targets []TargetSelector `json:"targets"`
	} `json:"spec"`
}

type VirtualService struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Spec struct {
		Gateways []string   `json:"gateways"`
		Hosts    []string   `json:"hosts"`
		HTTP     []HTTPSpec `json:"http"`
	} `json:"spec"`
}

type Gateway struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name            string            `json:"name"`
		Namespace       string            `json:"namespace"`
		ResourceVersion string            `json:"resourceVersion,omitempty"`
		Labels          map[string]string `json:"labels"`
		Annotations     map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Selector map[string]string `json:"selector"`
		Servers  []Server          `json:"servers"`
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
		HttpsRedirect           bool     `json:"httpsRedirect,omitempty"`
		Mode                    string   `json:"mode,omitempty"`
		ServerCertificate       string   `json:"serverCertificate,omitempty"`
		PrivateKey              string   `json:"privateKey,omitempty"`
		CaCertificates          string   `json:"caCertificates,omitempty"`
		CredentialName          string   `json:"credentialName,omitempty"`
		SubjectAlternativeNames []string `json:"subjectAltNames,omitempty"`
		MinProtocolVersion      string   `json:"minProtocolVersion,omitempty"`
		MaxProtocolVersion      string   `json:"maxProtocolVersion,omitempty"`
		CipherSuites            []string `json:"cipherSuites,omitempty"`
	} `json:"tls,omitempty"`
	DefaultEndpoint string `json:"defaultEndpoint,omitempty"`
}

type TLSSecretData struct {
	Certificate string
	Key         string
	Name        string
	CertName    string
	Space       string
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
                ],
		        "headers": {
		        	"response": {
		        		"set": {
		        			"Strict-Transport-Security":"max-age=31536000; includeSubDomains"
		        		}
		        	}
		        }
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
                ],
		        "headers": {
		        	"response": {
		        		"set": {
		        			"Strict-Transport-Security":"max-age=31536000; includeSubDomains"
		        		}
		        	}
		        }
            }{{end}}
{{end}}
        ]
    }
}
`

type kubernetesSecretTLS struct {
	ApiVersion string `json:"apiVersion"`
	Data       struct {
		CaCrt  string `json:"ca.crt,omitempty"`
		TlsCrt string `json:"tls.crt,omitempty"`
		TlsKey string `json:"tls.key,omitempty"`
	} `json:"data"`
	Kind     string `json:"kind"`
	Metadata struct {
		Annotations struct {
			AltNames    string `json:"akkeris.k8s.io/alt-names"`
			CommonNames string `json:"akkeris.k8s.io/common-name"`
		} `json:"annotations"`
		Labels struct {
			CertificateName string `json:"akkeris.k8s.io/certificate-name"`
		} `json:"labels"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Type string `json:"type"`
}



type kubernetesSecretTLSList struct {
	Items []kubernetesSecretTLS `json:"items"`
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

type IstioIngress struct {
	runtime runtime.Runtime
	config  *IngressConfig
	db      *sql.DB
}

func GetIstioIngress(db *sql.DB, config *IngressConfig) (*IstioIngress, error) {
	if config.Device != "istio" {
		return nil, errors.New("Unable to initialize the istio ingress, the config is not for Istio: " + config.Device)
	}
	runtimes, err := runtime.GetAllRuntimes(db)
	// TODO: This is obvious we don't yet support multi-cluster regions.
	//       and this is an artifact of that, we shouldn't have a 'stack' our
	//       certificates or ingress are issued from.
	if err != nil {
		return nil, err
	}
	if len(runtimes) == 0 {
		return nil, errors.New("No runtime was found wilhe trying to init istio ingress.")
	}
	runtime := runtimes[0]

	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio initialized with %s for namespace %s and gateway %s\n", config.Address, config.Environment, config.Name)
	}
	return &IstioIngress{
		runtime: runtime,
		config:  config,
		db:      db,
	}, nil
}

func (ingress *IstioIngress) VirtualServiceExists(domain string) (bool, string, error) {
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/"+domain, nil)
	if err != nil {
		return false, "", err
	}
	if code == http.StatusOK {
		var vsrv VirtualService
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
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/"+newdomain, nil)
	if err != nil {
		return false, err
	}
	if code == http.StatusOK {
		return true, nil
	} else {
		return false, nil
	}
}

func (ingress *IstioIngress) DeleteVirtualService(domain string) error {
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/"+domain, nil)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		return errors.New("virtual service was not found")
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return errors.New("Unable to delete virtual service: " + string(body))
	}
	return nil
}

func (ingress *IstioIngress) DeleteGateway(domain string) error {
	newdomain := strings.Replace(domain, ".", "-", -1) + "-gateway"
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/"+newdomain, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return errors.New("Unable to delete gateway: " + string(body))
	}
	return nil
}

func (ingress *IstioIngress) AppVirtualService(space string, app string) (*VirtualService, error) {
	var vs *VirtualService
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/"+app+"-"+space, nil)
	if err != nil {
		return vs, err
	}
	if code == http.StatusNotFound {
		return vs, errors.New("virtual service was not found")
	}
	if err = json.Unmarshal(body, &vs); err != nil {
		return vs, err
	}
	return vs, nil
}

func (ingress *IstioIngress) UpdateAppVirtualService(vs *VirtualService, space string, app string) error {
	body, code, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/"+app+"-"+space, vs)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return errors.New("Unable to update virtual service: " + string(body))
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateVirtualService(router structs.Routerspec, sitevs *VirtualService, exists bool) error {
	var err error = nil

	// Force good security practices
	for i, _ := range sitevs.Spec.HTTP {
		if sitevs.Spec.HTTP[i].Headers.Response.Set == nil {
			sitevs.Spec.HTTP[i].Headers.Response.Set = make(map[string]string)
		}
		sitevs.Spec.HTTP[i].Headers.Response.Set["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
	}

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
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/virtualservices/"+router.Domain, sitevs)
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
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/"+gateway.Metadata.Name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/"+gateway.Metadata.Name, gateway)
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

func RemoveHostsAndServers(domain string, certificate string, gateway *Gateway) (err error, dirty bool, out *Gateway) {
	// copy information
	var onstack Gateway
	out = &onstack
	*out = *gateway
	dirty = false

	// remove TLS server
	var removeServerWithTLSPortName string = ""
	var removeServerWithHttpPortName string = ""
	for i, server := range out.Spec.Servers {
		var removeHostRecord string = ""
		for _, host := range server.Hosts {
			if host == domain {
				// Remove host only if server has more than one, remove server if only one.
				if len(server.Hosts) == 1 {
					// Remove server
					if server.Port.Number == 443 && server.TLS.CredentialName == certificate {
						removeServerWithTLSPortName = server.Port.Name
					} else if server.Port.Number == 80 {
						removeServerWithHttpPortName = server.Port.Name
					}
				} else {
					// Remove host record
					removeHostRecord = host
				}
			}
		}
		if removeHostRecord != "" {
			var newHosts []string
			for _, host := range server.Hosts {
				if host != removeHostRecord {
					newHosts = append(newHosts, host)
				} else {
					dirty = true
				}
			}
			if dirty {
				out.Spec.Servers[i].Hosts = newHosts
			} else {
				return fmt.Errorf("Error: were instructed to remove host (%s) but it was not found on server %#+v\n", removeHostRecord, server), false, nil
			}
		}
	}

	if removeServerWithTLSPortName != "" {
		var newServers []Server
		for _, server := range out.Spec.Servers {
			if server.Port.Name != removeServerWithTLSPortName {
				newServers = append(newServers, server)
			} else {
				dirty = true
			}
		}
		if dirty {
			out.Spec.Servers = newServers
		} else {
			return fmt.Errorf("Error: We were instructed to remove server (%s) but it could not be found on gateway %#+v\n", removeServerWithTLSPortName, gateway), false, nil
		}
	}
	if removeServerWithHttpPortName != "" {
		var newServers []Server
		for _, server := range out.Spec.Servers {
			if server.Port.Name != removeServerWithHttpPortName {
				newServers = append(newServers, server)
			} else {
				dirty = true
			}
		}
		if dirty {
			out.Spec.Servers = newServers
		} else {
			return fmt.Errorf("Error: We were instructed to remove server (%s) but it could not be found on gateway %#+v\n", removeServerWithHttpPortName, gateway), false, nil
		}
	}
	return nil, dirty, out
}

func AddHostsAndServers(domain string, certificate string, gateway *Gateway) (err error, dirty bool, out *Gateway) {
	var onstack Gateway
	out = &onstack
	*out = *gateway
	var addTlsServer = true
	var addHttpServer = true
	for i, server := range out.Spec.Servers {
		if server.Port.Number == 443 && server.TLS.CredentialName == certificate {
			addTlsServer = false
			var addTlsHost = true
			// certificate server already exists, see if we need to add hosts
			for _, host := range server.Hosts {
				if host == domain {
					addTlsHost = false
					// we have a host already
				}
			}
			if addTlsHost {
				// referencing by index we can modify the hosts inline, rather than modifying a copy of the
				// the array pointer (which will not affect the output)
				out.Spec.Servers[i].Hosts = append(out.Spec.Servers[i].Hosts, domain)
				dirty = true
			}
		}
		if server.Port.Number == 80 {
			for _, host := range server.Hosts {
				if host == domain {
					addHttpServer = false
				}
			}
		}
	}
	if addTlsServer {
		var httpsServer Server
		httpsServer.Hosts = append(httpsServer.Hosts, domain)
		httpsServer.Port.Name = "https-" + certificate
		httpsServer.Port.Number = 443
		httpsServer.Port.Protocol = "HTTPS"
		httpsServer.TLS.CredentialName = certificate
		httpsServer.TLS.MinProtocolVersion = "TLSV1_2"
		httpsServer.TLS.Mode = "SIMPLE"
		httpsServer.TLS.PrivateKey = "/etc/istio/" + certificate + "/tls.key"
		httpsServer.TLS.ServerCertificate = "/etc/istio/" + certificate + "/tls.crt"
		out.Spec.Servers = append(out.Spec.Servers, httpsServer)
		dirty = true
	}
	if addHttpServer {
		var httpServer Server
		httpServer.Hosts = append(httpServer.Hosts, domain)
		httpServer.Port.Name = "http-" + strings.Replace(domain, ".", "-", -1)
		httpServer.Port.Number = 80
		httpServer.Port.Protocol = "HTTP"
		httpServer.TLS.HttpsRedirect = true
		out.Spec.Servers = append(out.Spec.Servers, httpServer)
		dirty = true
	}

	return nil, dirty, out
}

func (ingress *IstioIngress) DeleteUberSiteGateway(domain string, certificate string, internal bool) error {
	var gateway Gateway
	gatewayType := "public"
	if internal {
		gatewayType = "private"
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/sites-"+gatewayType, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		if err = json.Unmarshal(body, &gateway); err != nil {
			return err
		}
	} else {
		return errors.New("The specified gateway sites-" + gatewayType + " was not found " + strconv.Itoa(code))
	}

	err, dirty, updated_gateway := RemoveHostsAndServers(domain, certificate, &gateway)
	if err != nil {
		return err
	}
	if dirty {
		return ingress.InstallOrUpdateGateway(domain, updated_gateway)
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateJWTAuthFilter(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string) (error) {
	var jwtPolicy Policy
	jwtPolicy.Kind = "Policy"
	jwtPolicy.APIVersion = "authentication.istio.io/v1alpha1"
	jwtPolicy.Metadata.Name = appname
	jwtPolicy.Metadata.Namespace = space
	jwtPolicy.Spec.Origins = []OriginAuthenticationMethod{ 
		OriginAuthenticationMethod{
			Jwt:Jwt{
				Issuer: issuer,
				JwksUri: jwksUri,
				Audiences: audiences,
			},
		},
	}
	jwtPolicy.Spec.PrincipalBinding = "USE_ORIGIN"
	jwtPolicy.Spec.Targets = []TargetSelector{
		TargetSelector{
			Name: appname,
		},
	}

	if len(excludes) > 0 {
		jwtPolicy.Spec.Origins[0].Jwt.TriggerRules = make([]TriggerRule, 1)
		jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths = make([]StringMatch, 0)
		for _, exclude := range excludes {
			jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths = append(jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths, StringMatch{Prefix:exclude})
		}
	}

	body, code, err := ingress.runtime.GenericRequest("post", "/apis/" + jwtPolicy.APIVersion +  "/namespaces/" + space + "/policies", jwtPolicy)
	if err != nil {
		body, code, err = ingress.runtime.GenericRequest("put", "/apis/" + jwtPolicy.APIVersion +  "/namespaces/" + space + "/policies/" + appname, jwtPolicy)
	}
	if err != nil {
		return err
	}
	if code == http.StatusOK || code == http.StatusCreated {
		return nil
	} else {
		return errors.New("The response for creating/updating JWT auth policy failed: " + strconv.Itoa(code) + " " + string(body))
	}
}

func (ingress *IstioIngress) DeleteJWTAuthFilter(appname string, space string, fqdn string, port int64) (error) {
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/authentication.istio.io/v1alpha1/namespaces/" + space + "/policies/" + appname, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		return nil
	} else {
		return errors.New("The response for deleting a JWT auth policy failed: " + strconv.Itoa(code) + " " + string(body))
	}
}

func (ingress *IstioIngress) InstallOrUpdateUberSiteGateway(domain string, certificate string, internal bool) error {
	var gateway Gateway
	gatewayType := "public"
	if internal {
		gatewayType = "private"
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/networking.istio.io/v1alpha3/namespaces/sites-system/gateways/sites-"+gatewayType, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		if err = json.Unmarshal(body, &gateway); err != nil {
			return err
		}
	} else if code == http.StatusNotFound {
		// populate with defaults
		gateway.APIVersion = "networking.istio.io/v1alpha3"
		gateway.Kind = "Gateway"
		gateway.Metadata.Name = "sites-" + gatewayType
		gateway.Metadata.Namespace = "sites-system"
		gateway.Spec.Selector = make(map[string]string)
		gateway.Spec.Selector["istio"] = "sites-" + gatewayType + "-ingressgateway"
	} else {
		return errors.New("Response from request for sites gateway did not make sense: " + strconv.Itoa(code) + " " + string(body))
	}
	err, dirty, updated_gateway := RemoveHostsAndServers(domain, certificate, &gateway)
	if err != nil {
		return err
	}
	err, dirty, updated_gateway = AddHostsAndServers(domain, certificate, updated_gateway)
	if err != nil {
		return err
	}
	if dirty {
		return ingress.InstallOrUpdateGateway(domain, updated_gateway)
	}
	return nil
}

func (ingress *IstioIngress) GetCertificateFromDomain(domain string) string {
	// See if any certificates are available, search is in this order:
	//
	// 1. See if a direct certificate exists for the domain name.
	// 2. See if there's a wildcard certificate installed.
	// 3. Default to the star certificate and hope it works.
	//
	var certName = "star-certificate"
	certs, err := ingress.GetInstalledCertificates(domain)
	if err == nil && len(certs) > 0 {
		certName = strings.Replace(strings.Replace(domain, ".", "-", -1), "*", "star", -1) + "-tls"
	} else {
		starCert := "*." + strings.Join(strings.Split(domain, ".")[1:], ".")
		certs, err = ingress.GetInstalledCertificates(starCert)
		if err == nil && len(certs) > 0 {
			certName = strings.Replace(strings.Replace(starCert, ".", "-", -1), "*", "star", -1) + "-tls"
		}
	}
	return certName
}

// Standard Ingress Methods:

/*
 * We need to replicate the behavior of the F5, when installing a new site,
 * can likely assume that a certificate is installed, we can look for a list
 * of appropriate certificates and the most specific, unexpired one wins.
 */
func (ingress *IstioIngress) CreateOrUpdateRouter(router structs.Routerspec) error {
	exists, version, err := ingress.VirtualServiceExists(router.Domain)
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
	var sitevs VirtualService
	if err = json.Unmarshal([]byte(strings.Replace(string(b.Bytes()), "\n", " ", -1)), &sitevs); err != nil {
		return err
	}
	if err = ingress.InstallOrUpdateUberSiteGateway(router.Domain, ingress.GetCertificateFromDomain(router.Domain), router.Internal); err != nil {
		return err
	}
	return ingress.InstallOrUpdateVirtualService(router, &sitevs, exists)
}

func (ingress *IstioIngress) SetMaintenancePage(app string, space string, value bool) error {
	virtualService, err := ingress.AppVirtualService(space, app)
	if err != nil {
		if err.Error() == "virtual service was not found" {
			// Go ahead and ignore setting the maintenace page.  We can't as there's no deployment yet.
			return nil
		}
		return err
	}
	if len(virtualService.Spec.HTTP) == 0 || len(virtualService.Spec.HTTP[0].Route) == 0 {
		return errors.New("The specified maintenance page could not be found or did not have a routable virtual service.")
	}
	
	downpage := "akkeris404.akkeris-system.svc.cluster.local"
	if os.Getenv("ISTIO_DOWNPAGE") != "" {
		downpage = os.Getenv("ISTIO_DOWNPAGE")
	}

	if value {
		virtualService.Spec.HTTP[0].Route[0].Destination.Host = downpage
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
		if err.Error() == "virtual service was not found" {
			// Go ahead and ignore setting the maintenace page.  We can't as there's no deployment yet.
			return false, nil
		}
		return false, err
	}
	
	downpage := "akkeris404.akkeris-system.svc.cluster.local"
	if os.Getenv("ISTIO_DOWNPAGE") != "" {
		downpage = os.Getenv("ISTIO_DOWNPAGE")
	}

	if len(virtualService.Spec.HTTP) == 0 || len(virtualService.Spec.HTTP[0].Route) == 0 {
		return false, errors.New("The specified maintenance page could not be found or did not have a routable virtual service.")
	}
	if virtualService.Spec.HTTP[0].Route[0].Destination.Host == downpage {
		return true, nil
	} else {
		return false, nil
	}
}

func (ingress *IstioIngress) DeleteRouter(router structs.Routerspec) error {
	if err := ingress.DeleteVirtualService(router.Domain); err != nil {
		if err.Error() == "virtual service was not found" {
			// if we do not have a virtual service bail out without
			// attempting to remove the gateway.
			return nil
		}
		return err
	}
	return ingress.DeleteUberSiteGateway(router.Domain, ingress.GetCertificateFromDomain(router.Domain), router.Internal)
}

func (ingress *IstioIngress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error {
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
		Certificate: base64.StdEncoding.EncodeToString(pem_cert),
		Key:         base64.StdEncoding.EncodeToString(pem_key),
		Name:        main_server_name,
		CertName:    main_certs_name,
		Space:       ingress.config.Environment,
	}
	if err := t.Execute(wr, tlsSecret); err != nil {
		return err
	}
	wr.Flush()

	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}

	_, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+certificateNamespace+"/secrets/"+main_certs_name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		_, _, err = ingress.runtime.GenericRequest("put", "/api/v1/namespaces/"+certificateNamespace+"/secrets/"+main_certs_name, b.Bytes())
		return err
	} else {
		_, _, err = ingress.runtime.GenericRequest("post", "/api/v1/namespaces/"+certificateNamespace+"/secrets", b.Bytes())
		return err
	}
}

func (ingress *IstioIngress) GetInstalledCertificates(site string) ([]Certificate, error) {
	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}

	var certList kubernetesSecretTLSList

	if site != "*" {
		main_server_name := strings.Replace(site, "*.", "star.", -1)
		main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+certificateNamespace+"/secrets/"+main_certs_name, nil)

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
		certList.Items = make([]kubernetesSecretTLS, 0)
		certList.Items = append(certList.Items, t)
	} else {
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+certificateNamespace+"/secrets?fieldSelector=type%3Dkubernetes.io%2Ftls", nil)
		if err != nil {
			return nil, err
		}
		if code == http.StatusNotFound {
			return []Certificate{}, nil
		} else if code != http.StatusOK {
			return nil, errors.New("Failure to lookup certificate: " + string(body))
		}
		if err = json.Unmarshal(body, &certList); err != nil {
			return nil, err
		}
	}

	certificates := make([]Certificate, 0)
	for _, t := range certList.Items {
		pem_certs, err := base64.StdEncoding.DecodeString(t.Data.TlsCrt)
		if err != nil {
			return nil, err
		}
		x509_decoded_cert, _, _, err := DecodeCertificateBundle(site, pem_certs)
		if err != nil {
			return nil, err
		}
		
		main_server_name := strings.Replace(x509_decoded_cert.Subject.CommonName, "*.", "star.", -1)
		main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"

		var certType string = "normal"
		if len(x509_decoded_cert.DNSNames) > 1 {
			certType = "sans"
		}
		for _, n := range x509_decoded_cert.DNSNames {
			if strings.Contains(n, "*") {
				certType = "wildcard"
			}
		}

		certificates = append(certificates, Certificate{
			Type:         certType,
			Name:         main_certs_name,
			Expires:      x509_decoded_cert.NotAfter.Unix(),
			Alternatives: x509_decoded_cert.DNSNames,
			Expired:      x509_decoded_cert.NotAfter.Before(time.Now()),
			Address:      ingress.config.Address,
		})
	}

	return certificates, nil
}

func (ingress *IstioIngress) Config() *IngressConfig {
	return ingress.config
}

func (ingress *IstioIngress) Name() string {
	return "istio"
}
