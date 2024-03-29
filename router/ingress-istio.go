package router

import (
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"region-api/runtime"
	"region-api/structs"
	"strconv"
	"strings"
	"time"

	kube "k8s.io/api/core/v1"
	kubemetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const VS_RETRY_COUNT = 20

// Istio does not expose their internal structures, well, they do
// but they just define the Spec field as a map[string]interface{}
// so its all but useless, this sadly mimicks the structure used.

const IstioNetworkingAPIVersion = "networking.istio.io/v1beta1"
const IstioSecurityAPIVersion = "security.istio.io/v1beta1"

// Policy types for istio (https://istio.io/docs/reference/config/security/istio.authentication.v1alpha1/#Policy)
type StringMatch struct {
	Exact  string `json:"exact,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

type JWTRule struct {
	Issuer    string   `json:"issuer,omitempty"`
	Audiences []string `json:"audiences,omitempty"`
	JwksUri   string   `json:"jwksUri"`
}

type WorkloadSelector struct {
	Labels map[string]string `json:"labels,omitempty"`
}

type RequestAuthentication struct {
	kubemetav1.TypeMeta   `json:",inline"`
	kubemetav1.ObjectMeta `json:"metadata"`
	Spec                  struct {
		Selector WorkloadSelector `json:"selector"`
		JWTRules []JWTRule        `json:"jwtRules"`
	} `json:"spec"`
}

type From struct {
	Source struct {
		Principals           []string `json:"principals,omitempty"`
		NotPrincipals        []string `json:"notPrincipals,omitempty"`
		RequestPrincipals    []string `json:"requestPrincipals,omitempty"`
		NotRequestPrincipals []string `json:"notRequestPrincipals,omitempty"`
		Namespaces           []string `json:"namespaces"`
		NotNamespaces        []string `json:"notNamespaces"`
		IpBlocks             []string `json:"ipBlocks"`
		NotIpBlocks          []string `json:"notIpBlocks"`
	} `json:"source"`
}

type Operation struct {
	Hosts      []string `json:"hosts,omitempty"`
	NotHosts   []string `json:"notHosts,omitempty"`
	Ports      []string `json:"ports,omitempty"`
	NotPorts   []string `json:"notPorts,omitempty"`
	Methods    []string `json:"methods,omitempty"`
	NotMethods []string `json:"notMethods,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	NotPaths   []string `json:"notPaths,omitempty"`
}

type To struct {
	Operation Operation `json:"operation,omitempty"`
}

type Rule struct {
	From []From `json:"from,omitempty"`
	To   []To   `json:"to,omitempty"`
}

type AuthorizationPolicy struct {
	kubemetav1.TypeMeta   `json:",inline"`
	kubemetav1.ObjectMeta `json:"metadata"`
	Spec                  struct {
		Selector WorkloadSelector `json:"selector"`
		Rules    []Rule           `json:"rules"`
	} `json:"spec"`
}

// Gateway types for istio (https://istio.io/docs/reference/config/networking/gateway/)
type TLSOptions struct {
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
}

type Server struct {
	Hosts []string `json:"hosts"`
	Port  struct {
		Name     string `json:"name"`
		Number   int    `json:"number"`
		Protocol string `json:"protocol"`
	} `json:"port"`
	TLS             TLSOptions `json:"tls,omitempty"`
	DefaultEndpoint string     `json:"defaultEndpoint,omitempty"`
}

type Gateway struct {
	kubemetav1.TypeMeta   `json:",inline"`
	kubemetav1.ObjectMeta `json:"metadata"`
	Spec                  struct {
		Selector map[string]string `json:"selector"`
		Servers  []Server          `json:"servers"`
	} `json:"spec"`
}

// Virtual Service Types for istio (https://istio.io/docs/reference/config/networking/virtual-service/)
type HeaderOperations struct {
	Set    map[string]string `json:"set,omitempty"`
	Add    map[string]string `json:"add,omitempty"`
	Remove []string          `json:"remove,omitempty"`
}

type Headers struct {
	Request  HeaderOperations `json:"request,omitempty"`
	Response HeaderOperations `json:"response,omitempty"`
}

type Match struct {
	URI           StringMatch `json:"uri"`
	IgnoreUriCase bool        `json:"ignoreUriCase"`
}

type Rewrite struct {
	URI string `json:"uri,omitempty"`
}

type CorsPolicy struct {
	AllowOrigins     []StringMatch `json:"allowOrigins"`
	AllowMethods     []string      `json:"allowMethods"`
	AllowHeaders     []string      `json:"allowHeaders"`
	ExposeHeaders    []string      `json:"exposeHeaders"`
	MaxAge           string        `json:"maxAge"`
	AllowCredentials bool          `json:"allowCredentials"`
}

type Port struct {
	Number int32 `json:"number"`
}

type Destination struct {
	Host   string `json:"host"`
	Subset string `json:"subset,omitempty"`
	Port   Port   `json:"port"`
}

type Routes struct {
	Destination Destination `json:"destination"`
}

type HTTP struct {
	Match      []Match     `json:"match,omitempty"`
	Route      []Routes    `json:"route"`
	Rewrite    *Rewrite    `json:"rewrite,omitempty"`
	Headers    *Headers    `json:"headers,omitempty"`
	CorsPolicy *CorsPolicy `json:"corsPolicy,omitempty"`
}

type VirtualService struct {
	kubemetav1.TypeMeta   `json:",inline"`
	kubemetav1.ObjectMeta `json:"metadata"`
	Spec                  struct {
		Hosts    []string `json:"hosts"`
		Gateways []string `json:"gateways"`
		HTTP     []HTTP   `json:"http"`
	} `json:"spec"`
}

func removeSlashSlash(input string) string {
	toreturn := strings.Replace(input, "//", "/", -1)
	if toreturn == "" {
		return "/"
	}
	return toreturn
}

func removeSlash(input string) string {
	if input == "/" {
		return ""
	}
	return input
}

func addSlash(input string) string {
	if input[len(input)-1] == '/' {
		return input
	}
	return input + "/"
}

func removeLeadingSlash(input string) string {
	if input == "" {
		return input
	}
	if input[len(input)-1] == '/' {
		return input[:len(input)-1]
	}
	return input
}

type IstioIngress struct {
	runtime              runtime.Runtime
	config               *IngressConfig
	db                   *sql.DB
	certificateNamespace string
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
	certificateNamespace := os.Getenv("CERT_NAMESPACE")
	if certificateNamespace == "" {
		certificateNamespace = "istio-system"
	}
	return &IstioIngress{
		runtime:              runtime,
		config:               config,
		db:                   db,
		certificateNamespace: certificateNamespace,
	}, nil
}

func (ingress *IstioIngress) VirtualServiceExists(name string) (bool, string, error) {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - checking if virtual service exists %s\n", name)
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices/"+name, nil)
	if err != nil {
		return false, "", err
	}
	if code == http.StatusOK {
		var vsrv VirtualService
		err = json.Unmarshal(body, &vsrv)
		if err != nil {
			return false, "", nil
		}
		return true, vsrv.GetResourceVersion(), nil
	} else {
		return false, "", nil
	}
}

func (ingress *IstioIngress) GatewayExists(domain string) (bool, error) {
	newdomain := strings.Replace(domain, ".", "-", -1) + "-gateway"
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways/"+newdomain, nil)
	if err != nil {
		return false, err
	}
	if code == http.StatusOK {
		return true, nil
	} else {
		return false, nil
	}
}

func (ingress *IstioIngress) DeleteVirtualService(name string) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - deleting virtual service for %s\n", name)
	}
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices/"+name, nil)
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
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - deleting gateway %s\n", domain)
	}
	newdomain := strings.Replace(domain, ".", "-", -1) + "-gateway"
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways/"+newdomain, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return errors.New("Unable to delete gateway: " + string(body))
	}
	return nil
}

func (ingress *IstioIngress) GetVirtualService(name string) (*VirtualService, error) {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - getting virtual service %s\n", name)
	}
	var vs *VirtualService
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices/"+name, nil)
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

func (ingress *IstioIngress) UpdateVirtualService(vs *VirtualService, name string) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - updating virtual service %s\n", name)
	}
	body, code, err := ingress.runtime.GenericRequest("put", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices/"+name, vs)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return errors.New("Unable to update virtual service: " + string(body))
	}
	return nil
}

func (ingress *IstioIngress) AppVirtualService(space string, app string) (*VirtualService, error) {
	return ingress.GetVirtualService(app + "-" + space)
}

func (ingress *IstioIngress) UpdateAppVirtualService(vs *VirtualService, space string, app string) error {
	return ingress.UpdateVirtualService(vs, app+"-"+space)
}

func (ingress *IstioIngress) InstallOrUpdateVirtualService(domain string, vs *VirtualService, exists bool) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - starting install or update virtual service for %s\n", domain)
	}
	// Force good security practices
	for i, _ := range vs.Spec.HTTP {
		if vs.Spec.HTTP[i].Headers.Response.Set == nil {
			vs.Spec.HTTP[i].Headers.Response.Set = make(map[string]string)
		}
		vs.Spec.HTTP[i].Headers.Response.Set["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
	}

	if !exists {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices", vs)
		if err != nil {
			fmt.Printf("Failed to create virtual service due to protocol error %s: %#+v\n", err.Error(), vs)
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create virtual service %#+v due to %s - %s\n", vs, strconv.Itoa(code), string(body))
			return errors.New("Unable to create virtual service " + vs.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/virtualservices/"+domain, vs)
		if err != nil {
			fmt.Printf("Failed to update virtual service due to protocol error %s: %#+v\n", err.Error(), vs)
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to update virtual service %#+v due to %s - %s\n", vs, strconv.Itoa(code), string(body))
			return errors.New("Unable to update virtual service " + vs.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateGateway(domain string, gateway *Gateway) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - starting install or update gateway for %s\n", domain)
	}
	body, code, err := ingress.runtime.GenericRequest("put", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways/"+gateway.GetName(), gateway)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways", gateway)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
			return errors.New("Unable to create gateway " + gateway.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else if code == http.StatusConflict {
		return errors.New("conflict")
	} else if code != http.StatusOK && code != http.StatusCreated {
		fmt.Printf("Failed to update gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
		return errors.New("Unable to update gateway " + gateway.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
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

func (ingress *IstioIngress) DeleteUberSiteGateway(domain string, certificate string, internal bool, retryNumber int) error {
	if retryNumber == 7 {
		return errors.New("failed to install or update gateway due to conflict, out of retries")
	} else if retryNumber != 0 {
		<-time.NewTicker(time.Second * (time.Duration(math.Pow(2, float64(retryNumber)) - 1))).C // wait progressively longer on each retry, 0, 1, 3, 7, 15, 31
	}

	var gateway Gateway
	gatewayType := "public"
	if internal {
		gatewayType = "private"
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways/sites-"+gatewayType, nil)
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
		if err := ingress.InstallOrUpdateGateway(domain, updated_gateway); err != nil {
			if err.Error() == "conflict" {
				// try again
				return ingress.DeleteUberSiteGateway(domain, certificate, internal, retryNumber+1)
			} else {
				return err
			}
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateIstioRequestAuthentication(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string, includes []string) error {
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/requestauthentications/"+appname, nil)
	if err != nil {
		return err
	}
	var jwtPolicy RequestAuthentication
	if code == http.StatusNotFound {
		jwtPolicy.Kind = "RequestAuthentication"
		jwtPolicy.APIVersion = IstioSecurityAPIVersion
		jwtPolicy.SetName(appname)
		jwtPolicy.SetNamespace(space)
	} else {
		if err = json.Unmarshal(body, &jwtPolicy); err != nil {
			return err
		}
	}
	jwtPolicy.Spec.JWTRules = []JWTRule{
		JWTRule{
			Issuer:    issuer,
			JwksUri:   jwksUri,
			Audiences: audiences,
		},
	}
	jwtPolicy.Spec.Selector = WorkloadSelector{
		Labels: map[string]string{
			"app": appname,
		},
	}
	if code == http.StatusNotFound {
		body, code, err = ingress.runtime.GenericRequest("post", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/requestauthentications", jwtPolicy)
		if code != http.StatusOK && code != http.StatusCreated {
			return errors.New("The response for deleting a JWT auth policy failed: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err = ingress.runtime.GenericRequest("put", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/requestauthentications/"+appname, jwtPolicy)
		if code != http.StatusOK && code != http.StatusCreated {
			return errors.New("The response for deleting a JWT auth policy failed: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateIstioAuthorizationPolicies(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string, includes []string) error {
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/authorizationpolicies/"+appname, nil)
	if err != nil {
		return err
	}

	var authPolicy AuthorizationPolicy
	if code == http.StatusNotFound {
		authPolicy.Kind = "AuthorizationPolicy"
		authPolicy.APIVersion = IstioSecurityAPIVersion
		authPolicy.SetName(appname)
		authPolicy.SetNamespace(space)
	} else {
		if err = json.Unmarshal(body, &authPolicy); err != nil {
			return err
		}
	}
	authPolicy.Spec.Selector = WorkloadSelector{
		Labels: map[string]string{
			"app": appname,
		},
	}
	authPolicy.Spec.Rules = make([]Rule, 0)
	if len(excludes) > 0 {
		authPolicy.Spec.Rules = append(authPolicy.Spec.Rules, Rule{
			To: []To{
				To{
					Operation: Operation{
						NotPaths: excludes,
					},
				},
			},
		})
	}
	if len(includes) > 0 {
		authPolicy.Spec.Rules = append(authPolicy.Spec.Rules, Rule{
			To: []To{
				To{
					Operation: Operation{
						Paths: includes,
					},
				},
			},
		})
	}
	if code == http.StatusNotFound {
		body, code, err = ingress.runtime.GenericRequest("post", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/authorizationpolicies", authPolicy)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			return errors.New("The response for creating an JWT authorization policy failed: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err = ingress.runtime.GenericRequest("put", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/authorizationpolicies/"+appname, authPolicy)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			return errors.New("The response for updating a JWT authorization policy failed: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateJWTAuthFilter(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string, includes []string) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio installing or updating JWT Auth filter for %s-%s with %s\n", appname, space, jwksUri)
	}
	if err := ingress.InstallOrUpdateIstioRequestAuthentication(appname, space, fqdn, port, issuer, jwksUri, audiences, excludes, includes); err != nil {
		return err
	}
	if err := ingress.InstallOrUpdateIstioAuthorizationPolicies(appname, space, fqdn, port, issuer, jwksUri, audiences, excludes, includes); err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) DeleteJWTAuthFilter(appname string, space string, fqdn string, port int64) error {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - deleting any JWT Auth filter for %s-%s\n", appname, space)
	}
	_, _, err := ingress.runtime.GenericRequest("delete", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/requestauthentications/"+appname, nil)
	if err != nil {
		return err
	}
	_, _, err = ingress.runtime.GenericRequest("delete", "/apis/"+IstioSecurityAPIVersion+"/namespaces/"+space+"/authorizationpolicies/"+appname, nil)
	if err != nil {
		return err
	}
	return nil
}

// retryNumber should always be 0, except for when executed from within this function recursively.
func (ingress *IstioIngress) InstallOrUpdateUberSiteGateway(domain string, certificate string, internal bool, retryNumber int) error {
	if retryNumber == 7 {
		return errors.New("failed to install or update gateway due to conflict, out of retries")
	} else if retryNumber != 0 {
		<-time.NewTicker(time.Second * (time.Duration(math.Pow(2, float64(retryNumber)) - 1))).C // wait progressively longer on each retry, 0, 1, 3, 7, 15, 31
	}
	var gateway Gateway
	gatewayType := "public"
	if internal {
		gatewayType = "private"
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/"+IstioNetworkingAPIVersion+"/namespaces/sites-system/gateways/sites-"+gatewayType, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		if err = json.Unmarshal(body, &gateway); err != nil {
			return err
		}
	} else if code == http.StatusNotFound {
		// populate with defaults
		gateway.APIVersion = IstioNetworkingAPIVersion
		gateway.Kind = "Gateway"
		gateway.SetName("sites-" + gatewayType)
		gateway.SetNamespace("sites-system")
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
		if err := ingress.InstallOrUpdateGateway(domain, updated_gateway); err != nil {
			if err.Error() == "conflict" {
				// try again
				return ingress.InstallOrUpdateUberSiteGateway(domain, certificate, internal, retryNumber+1)
			} else {
				return err
			}
		}
	}
	return nil
}

func (ingress *IstioIngress) GetCertificateFromDomain(domain string) (error, string) {
	// See if any certificates are available, search is in this order:
	//
	// 1. See if a direct certificate exists for the domain name.
	// 2. See if there's a wildcard certificate installed.
	// 3. Default to the star certificate and hope it works.
	//
	certs, err := ingress.GetInstalledCertificates(domain)
	if err != nil {
		return err, ""
	}
	if len(certs) > 0 {
		return nil, strings.Replace(strings.Replace(domain, ".", "-", -1), "*", "star", -1) + "-tls"
	} else {
		starCert := "*." + strings.Join(strings.Split(domain, ".")[1:], ".")
		certs, err = ingress.GetInstalledCertificates(starCert)
		if err != nil {
			return err, ""
		}
		if len(certs) > 0 {
			return nil, strings.Replace(strings.Replace(starCert, ".", "-", -1), "*", "star", -1) + "-tls"
		}
	}
	return nil, "star-certificate"
}

func createHTTPSpecForVS(app string, space string, domain string, maintenance bool, adjustedPath string, rewritePath string, forwardedPath string, port int32, filters []structs.HttpFilters) HTTP {
	destination := app + "." + space + ".svc.cluster.local"
	if maintenance {
		destination = getDownPage()
	}

	http := HTTP{
		Match: []Match{Match{
			URI: StringMatch{
				Prefix: forwardedPath,
			},
			IgnoreUriCase: true,
		}},
		Rewrite: &Rewrite{
			URI: rewritePath,
		},
		Route: []Routes{Routes{
			Destination: Destination{
				Host: destination,
				Port: Port{
					Number: port,
				},
			},
		}},
		Headers: &Headers{
			Response: HeaderOperations{
				Set: map[string]string{
					"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
				},
			},
			Request: HeaderOperations{
				Set: map[string]string{
					"X-Forwarded-Path": forwardedPath,
					"X-Orig-Path":      removeLeadingSlash(adjustedPath),
					"X-Orig-Host":      domain,
					"X-Orig-Port":      "443",
					"X-Orig-Proto":     "https",
					"X-Request-Start":  "t=%START_TIME(%s.%3f)%",
				},
			},
		},
	}

	for _, filter := range filters {
		if filter.Type == "cors" {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Adding CORS filter to site %#+v\n", filter)
			}
			allow_origin := make([]StringMatch, 0)
			allow_methods := make([]string, 0)
			allow_headers := make([]string, 0)
			expose_headers := make([]string, 0)
			max_age := time.Second * 86400
			allow_credentials := false
			if val, ok := filter.Data["allow_origin"]; ok {
				if val != "" {
					origins := strings.Split(val, ",")
					for _, origin := range origins {
						allow_origin = append(allow_origin, StringMatch{Exact: origin})
					}
				}
			}
			if val, ok := filter.Data["allow_methods"]; ok {
				if val != "" {
					allow_methods = strings.Split(val, ",")
				}
			}
			if val, ok := filter.Data["allow_headers"]; ok {
				if val != "" {
					allow_headers = strings.Split(val, ",")
				}
			}
			if val, ok := filter.Data["expose_headers"]; ok {
				if val != "" {
					expose_headers = strings.Split(val, ",")
				}
			}
			if val, ok := filter.Data["max_age"]; ok {
				age, err := strconv.ParseInt(val, 10, 32)
				if err == nil {
					max_age = time.Second * time.Duration(age)
				} else {
					fmt.Printf("WARNING: Unable to convert max_age to value %s\n", val)
				}
			}
			if val, ok := filter.Data["allow_credentials"]; ok {
				if val == "true" {
					allow_credentials = true
				} else {
					allow_credentials = false
				}
			}
			http.CorsPolicy = &CorsPolicy{
				AllowOrigins:     allow_origin,
				AllowMethods:     allow_methods,
				AllowHeaders:     allow_headers,
				ExposeHeaders:    expose_headers,
				MaxAge:           max_age.String(),
				AllowCredentials: allow_credentials,
			}
		} else if filter.Type == "csp" {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Adding CSP filter %#+v\n", filter)
			}
			policy := ""
			if val, ok := filter.Data["policy"]; ok {
				policy = val
			}
			if policy != "" {

				if http.Headers.Response.Set == nil {
					http.Headers.Response.Set = make(map[string]string)
				}
				http.Headers.Response.Set["Content-Security-Policy"] = policy
			}
		}
	}

	return http
}

func PrepareVirtualServiceForCreateorUpdate(domain string, internal bool, paths []Route) (*VirtualService, error) {
	var defaultPort int32 = 80
	if os.Getenv("DEFAULT_PORT") != "" {
		port, err := strconv.ParseInt(os.Getenv("DEFAULT_PORT"), 10, 32)
		if err == nil {
			defaultPort = int32(port)
		} else {
			fmt.Printf("WARNING: DEFAULT_PORT was an invalid value: %s\n", os.Getenv("DEFAULT_PORT"))
		}
	}
	vs := VirtualService{}
	vs.APIVersion = IstioNetworkingAPIVersion
	vs.Kind = "VirtualService"
	vs.SetName(domain)
	vs.SetNamespace("sites-system")

	if internal {
		vs.Spec.Gateways = []string{"sites-private"}
	} else {
		vs.Spec.Gateways = []string{"sites-public"}
	}
	vs.Spec.Hosts = []string{domain}
	vs.Spec.HTTP = make([]HTTP, 0)
	for _, value := range paths {
		path := removeLeadingSlash(value.Path)
		vs.Spec.HTTP = append(vs.Spec.HTTP,
			createHTTPSpecForVS(value.App, value.Space, value.Domain, value.Maintenance, removeSlash(path), addSlash(value.ReplacePath), removeSlash(path)+"/", defaultPort, value.Filters))
		if removeSlashSlash(value.Path) == removeSlash(value.Path) {
			vs.Spec.HTTP = append(vs.Spec.HTTP,
				createHTTPSpecForVS(value.App, value.Space, value.Domain, value.Maintenance, removeSlashSlash(value.Path), value.ReplacePath, removeSlashSlash(path), defaultPort, value.Filters))
		}
	}
	return &vs, nil
}

func CertificateToSecret(server_name string, pem_cert []byte, pem_key []byte, namespace string) (*string, *kube.Secret, error) {
	block, _ := pem.Decode(pem_cert)
	if block == nil {
		fmt.Println("failed to parse PEM block containing the public certificate")
		return nil, nil, errors.New("Invalid certificate: Failed to decode PEM block")
	}
	x509_decoded_cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Println("invalid certificate provided")
		fmt.Println(err)
		return nil, nil, err
	}
	block, _ = pem.Decode([]byte(pem_key))
	if block == nil {
		fmt.Println("failed to parse PEM block containing the private key")
		return nil, nil, errors.New("Invalid key: Failed to decode PEM block")
	}
	main_server_name := strings.Replace(x509_decoded_cert.Subject.CommonName, "*.", "star.", -1)
	name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"
	secret := kube.Secret{}
	secret.Kind = "Secret"
	secret.APIVersion = "v1"
	secret.Type = kube.SecretTypeTLS
	secret.SetName(name)
	secret.SetNamespace(namespace)
	secret.SetAnnotations(map[string]string{
		"akkeris.k8s.io/alt-names":   main_server_name,
		"akkeris.k8s.io/common-name": main_server_name,
	})
	secret.SetLabels(map[string]string{
		"akkeris.k8s.io/certificate-name": main_server_name,
	})
	secret.Data = make(map[string][]byte, 0)
	secret.Data["tls.key"] = []byte(pem_key)
	secret.Data["tls.crt"] = []byte(pem_cert)
	return &name, &secret, nil
}

func getDownPage() string {
	if os.Getenv("ISTIO_DOWNPAGE") != "" {
		return os.Getenv("ISTIO_DOWNPAGE")
	}
	return "downpage.akkeris-system.svc.cluster.local"
}

func (ingress *IstioIngress) CreateOrUpdateRouter(domain string, internal bool, paths []Route) error {

	// Ensure a certificate exists and a hosts entry is registered with the gateway
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - create or update router firing for %s\n", domain)
	}
	err, cert_secret_name := ingress.GetCertificateFromDomain(domain)
	if err != nil {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Cannot obtain cert secret name (on push) for site %s because %s\n", domain, err.Error())
		}
		return err
	}
	if cert_secret_name == "" {
		panic("We should never have a cert_secret_name thats blank.")
	}
	if err = ingress.InstallOrUpdateUberSiteGateway(domain, cert_secret_name, internal, 0); err != nil {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Cannot install or update uber site gateway for site %s using cert %s because %s\n", domain, cert_secret_name, err.Error())
		}
		return err
	}

	// Ensure a virtual services exists
	exists, version, err := ingress.VirtualServiceExists(domain)
	if err != nil {
		fmt.Printf("[ingress] Istio - an error received from virtual service exists during create/update router: %s: %s\n", err.Error(), domain)
		return err
	}
	vs, err := PrepareVirtualServiceForCreateorUpdate(domain, internal, paths)
	if err != nil {
		fmt.Printf("[ingress] Istio - unable to prepare virtual service for create or update: %s: %s\n", err.Error(), domain)
		return err
	}
	if exists && version != "" {
		vs.SetResourceVersion(version)
	}
	if err = ingress.InstallOrUpdateVirtualService(domain, vs, exists); err != nil {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Cannot install or update virtualservice for site %s using cert %s using because %s, virtual service is %#+v\n", domain, cert_secret_name, err.Error(), vs)
		}
		return err
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateCORSAuthFilter(vsname string, path string, allowOrigin []string, allowMethods []string, allowHeaders []string, exposeHeaders []string, maxAge time.Duration, allowCredentials bool) error {
	virtualService, err := ingress.GetVirtualService(vsname)
	if err != nil {
		// not yet deployed
		return nil
	}
	var dirty = false
	allowOrigins := make([]StringMatch, 0)
	for _, origin := range allowOrigin {
		allowOrigins = append(allowOrigins, StringMatch{Exact: origin})
	}
	for i, http := range virtualService.Spec.HTTP {
		if http.Match == nil || len(http.Match) == 0 {
			virtualService.Spec.HTTP[0].CorsPolicy = &CorsPolicy{
				AllowOrigins:     allowOrigins,
				AllowMethods:     allowMethods,
				AllowHeaders:     allowHeaders,
				ExposeHeaders:    exposeHeaders,
				MaxAge:           maxAge.String(),
				AllowCredentials: allowCredentials,
			}
			dirty = true
		} else {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to add CORS policy, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if strings.HasPrefix(match.URI.Prefix, path) || match.URI.Exact == path || match.URI.Prefix == path {
					virtualService.Spec.HTTP[i].CorsPolicy = &CorsPolicy{
						AllowOrigins:     allowOrigins,
						AllowMethods:     allowMethods,
						AllowHeaders:     allowHeaders,
						ExposeHeaders:    exposeHeaders,
						MaxAge:           maxAge.String(),
						AllowCredentials: allowCredentials,
					}
					dirty = true
				}
			}
		}
	}
	if dirty == true {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Istio - Installing or updating CORS auth filter for %s at path %s: %#+v\n", vsname, path, virtualService)
		}
		if err = ingress.UpdateVirtualService(virtualService, vsname); err != nil {
			return err
		}
	}
	return nil
}

func (ingress *IstioIngress) DeleteCORSAuthFilter(vsname string, path string) error {
	virtualService, err := ingress.GetVirtualService(vsname)
	if err != nil {
		if err.Error() == "virtual service was not found" {
			// Go ahead and ignore setting this.  We can't as there's no deployment yet.
			return nil
		}
		return err
	}
	var dirty = false
	for i, http := range virtualService.Spec.HTTP {
		if http.Match == nil || len(http.Match) == 0 {
			virtualService.Spec.HTTP[i].CorsPolicy = nil
			dirty = true
		} else {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to remove CORS policy, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if strings.HasPrefix(match.URI.Prefix, path) || match.URI.Exact == path || match.URI.Prefix == path {
					virtualService.Spec.HTTP[i].CorsPolicy = nil
					dirty = true
				}
			}
		}
	}
	if dirty == true {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Removing CORS filter for %s at path %s\n", vsname, path)
		}
		if err = ingress.UpdateVirtualService(virtualService, vsname); err != nil {
			return err
		}
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateCSPFilter(vsname string, path string, policy string) error {
	virtualService, err := ingress.GetVirtualService(vsname)
	if err != nil {
		// not yet deployed
		return nil
	}
	var dirty = false
	for i, http := range virtualService.Spec.HTTP {
		if http.Match == nil || len(http.Match) == 0 {
			if virtualService.Spec.HTTP[0].Headers == nil {
				virtualService.Spec.HTTP[0].Headers = &Headers{}
			}
			if virtualService.Spec.HTTP[0].Headers.Response.Set == nil {
				virtualService.Spec.HTTP[0].Headers.Response.Set = make(map[string]string)
			}
			virtualService.Spec.HTTP[0].Headers.Response.Set["Content-Security-Policy"] = policy
			dirty = true
		} else {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to add CSP policy, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if strings.HasPrefix(match.URI.Prefix, path) || match.URI.Exact == path || match.URI.Prefix == path {
					if virtualService.Spec.HTTP[i].Headers == nil {
						virtualService.Spec.HTTP[i].Headers = &Headers{}
					}
					if virtualService.Spec.HTTP[i].Headers.Response.Set == nil {
						virtualService.Spec.HTTP[i].Headers.Response.Set = make(map[string]string)
					}
					virtualService.Spec.HTTP[i].Headers.Response.Set["Content-Security-Policy"] = policy
					dirty = true
				}
			}
		}
	}
	if dirty == true {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Istio - Installing or updating CSP policy for %s at path %s: %#+v\n", vsname, path, virtualService)
		}
		if err = ingress.UpdateVirtualService(virtualService, vsname); err != nil {
			return err
		}
	}
	return nil
}

func (ingress *IstioIngress) DeleteCSPFilter(vsname string, path string) error {
	virtualService, err := ingress.GetVirtualService(vsname)
	if err != nil {
		if err.Error() == "virtual service was not found" {
			// Go ahead and ignore setting this.  We can't as there's no deployment yet.
			return nil
		}
		return err
	}
	var dirty = false
	for i, http := range virtualService.Spec.HTTP {
		if http.Match == nil || len(http.Match) == 0 {
			if virtualService.Spec.HTTP[i].Headers != nil && virtualService.Spec.HTTP[i].Headers.Response.Set != nil {
				virtualService.Spec.HTTP[i].Headers.Response.Set["Content-Security-Policy"] = ""
				dirty = true
			}
		} else {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to remove CSP policy, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if strings.HasPrefix(match.URI.Prefix, path) || match.URI.Exact == path || match.URI.Prefix == path {
					if virtualService.Spec.HTTP[i].Headers != nil && virtualService.Spec.HTTP[i].Headers.Response.Set != nil {
						virtualService.Spec.HTTP[i].Headers.Response.Set["Content-Security-Policy"] = ""
						dirty = true
					}
				}
			}
		}
	}
	if dirty == true {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Removing CSP policy for %s at path %s\n", vsname, path)
		}
		if err = ingress.UpdateVirtualService(virtualService, vsname); err != nil {
			return err
		}
	}
	return nil
}

func setMaintenancePage(ingress *IstioIngress, vsname string, app string, space string, path string, value bool) error {
	virtualService, err := ingress.GetVirtualService(vsname)
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

	var dirty = false
	if len(virtualService.Spec.HTTP) == 1 && (virtualService.Spec.HTTP[0].Match == nil || len(virtualService.Spec.HTTP[0].Match) == 0 && path == "") {
		if value {
			virtualService.Spec.HTTP[0].Route[0].Destination.Host = getDownPage()
		} else {
			virtualService.Spec.HTTP[0].Route[0].Destination.Host = app + "." + space + ".svc.cluster.local"
		}
		dirty = true
	} else {
		for i, http := range virtualService.Spec.HTTP {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to set maintenance page, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if (match.URI.Exact == path && path != "") || (match.URI.Prefix == path && path != "") || match.URI.Prefix == (path+"/") || match.URI.Exact == (path+"/") {
					if os.Getenv("INGRESS_DEBUG") == "true" {
						fmt.Printf("[ingress] Setting maintenance page, updated path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
					}
					if value {
						virtualService.Spec.HTTP[i].Route[0].Destination.Host = getDownPage()
					} else {
						virtualService.Spec.HTTP[i].Route[0].Destination.Host = app + "." + space + ".svc.cluster.local"
					}
					dirty = true
				}
			}
		}
	}

	if dirty == true {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] update virtual services %s at path %s\n", vsname, path)
		}
		if err = ingress.UpdateVirtualService(virtualService, vsname); err != nil {
			return err
		}
	}

	return nil
}

func (ingress *IstioIngress) SetMaintenancePage(vsname string, app string, space string, path string, value bool) error {

	// There could be a bit of a race condition here.
	// If another service updates the virtualservice after we retrieve it, but before we modify it, Kubernetes will throw an error
	// If we encounter this race condition (409 Conflict), retry this up to VS_RETRY_COUNT times before giving up
	// In the future, we could only update one VirtualService at a time or something like that via a semaphore or some sort of lock

	for i := 0; i < VS_RETRY_COUNT; i++ {
		err := setMaintenancePage(ingress, vsname, app, space, path, value)
		if err == nil {
			// Successful update!
			return nil
		} else {
			if strings.Contains(err.Error(), "\"code\":409") && strings.Contains(err.Error(), "\"reason\":\"Conflict\"") {
				// If 409 conflict and at end of loop, return "limit reached" error message
				if i == VS_RETRY_COUNT-1 {
					return errors.New(fmt.Sprintf("Retry limit (%d) for 409 Conflict errors reached on creating virtual service %s", VS_RETRY_COUNT, vsname))
				}
				// Otherwise, print debug message and retry
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Istio - retrying update to virtual service %s (%d)\n", vsname, i)
				}
			} else {
				// Other unexpected error
				return err
			}
		}
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
	if len(virtualService.Spec.HTTP) == 0 || len(virtualService.Spec.HTTP[0].Route) == 0 {
		return false, errors.New("The specified maintenance page could not be found or did not have a routable virtual service.")
	}
	if virtualService.Spec.HTTP[0].Route[0].Destination.Host == getDownPage() {
		return true, nil
	} else {
		return false, nil
	}
}

func (ingress *IstioIngress) DeleteRouter(domain string, internal bool) error {
	if err := ingress.DeleteVirtualService(domain); err != nil {
		if err.Error() == "virtual service was not found" {
			// if we do not have a virtual service bail out without
			// attempting to remove the gateway.
			return nil
		}
		return err
	}
	err, cert_secret_name := ingress.GetCertificateFromDomain(domain)
	if err != nil {
		if os.Getenv("INGRESS_DEBUG") == "true" {
			fmt.Printf("[ingress] Cannot obtain cert secret name for site %s because %s\n", domain, err.Error())
		}
		return err
	}
	if cert_secret_name == "" {
		panic("We should never have a cert_secret_name thats blank.")
	}
	return ingress.DeleteUberSiteGateway(domain, cert_secret_name, internal, 0)
}

func (ingress *IstioIngress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error {
	name, secret, err := CertificateToSecret(server_name, pem_cert, pem_key, ingress.config.Environment)
	if err != nil {
		return err
	}
	_, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+ingress.certificateNamespace+"/secrets/"+*name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		_, _, err = ingress.runtime.GenericRequest("put", "/api/v1/namespaces/"+ingress.certificateNamespace+"/secrets/"+*name, secret)
		return err
	} else {
		_, _, err = ingress.runtime.GenericRequest("post", "/api/v1/namespaces/"+ingress.certificateNamespace+"/secrets", secret)
		return err
	}
}

func (ingress *IstioIngress) GetInstalledCertificates(site string) ([]Certificate, error) {
	var certList kube.SecretList
	if site != "*" {
		main_server_name := strings.Replace(site, "*.", "star.", -1)
		main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+ingress.certificateNamespace+"/secrets/"+main_certs_name, nil)
		if err != nil {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Cannot obtain secret for site %s because %s\n", site, err.Error())
			}
			return nil, err
		}
		if code == http.StatusNotFound {
			return []Certificate{}, nil
		} else if code != http.StatusOK {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Looking for certificate returned invalid code for site: %s, %d %s\n", site, code, err.Error())
			}
			return nil, errors.New("Failure to lookup certificate: " + string(body))
		}
		var t kube.Secret
		if err = json.Unmarshal(body, &t); err != nil {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Failed to unmarshal tls certificate: %s, %s, actually received: %s\n", site, err.Error(), string(body))
			}
			return nil, err
		}
		certList.Items = make([]kube.Secret, 0)
		certList.Items = append(certList.Items, t)
	} else {
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/"+ingress.certificateNamespace+"/secrets?fieldSelector=type%3Dkubernetes.io%2Ftls", nil)
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
		x509_decoded_cert, _, _, err := DecodeCertificateBundle(site, t.Data["tls.crt"])
		if err != nil {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Certificate bundle decode failed for %s: %s, original data body: %s\n", site, err.Error(), string(t.Data["tls.crt"]))
			}
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
