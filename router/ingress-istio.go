package router

import (
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
	"strconv"
	"strings"
	"time"
	"region-api/structs"
	kube "k8s.io/api/core/v1"
	kubemetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


// Istio does not expose their internal structures, well, they do
// but they just define the Spec field as a map[string]interface{}
// so its all but useless, this sadly mimicks the structure used.

const IstioNetworkingAPIVersion = "networking.istio.io/v1alpha3"
const IstioAuthenticationAPIVersion = "authentication.istio.io/v1alpha1"

// Policy types for istio (https://istio.io/docs/reference/config/security/istio.authentication.v1alpha1/#Policy)
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
	JwtHeaders []string `json:"jwtHeaders"`
	JwtParams []string `json:"jwtParams"`
	TriggerRules []TriggerRule `json:"triggerRules,omitempty"` 
}

type OriginAuthenticationMethod struct {
	Jwt Jwt `json:"jwt"`
}

type TargetSelector struct {
	Name string `json:"name"`
}

type Policy struct {
	kubemetav1.TypeMeta 	`json:",inline"`
	kubemetav1.ObjectMeta	`json:"metadata"`
	Spec struct {
		Origins []OriginAuthenticationMethod `json:"origins"`
		PrincipalBinding string `json:"principalBinding"` /* Can be USE_PEER or USE_ORIGIN, set to USE_ORIGIN */
		Targets []TargetSelector `json:"targets"`
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
	TLS TLSOptions `json:"tls,omitempty"`
	DefaultEndpoint string `json:"defaultEndpoint,omitempty"`
}

type Gateway struct {
	kubemetav1.TypeMeta 	`json:",inline"`
	kubemetav1.ObjectMeta	`json:"metadata"`
	Spec struct {
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
	URI StringMatch `json:"uri"`
	IgnoreUriCase bool `json:"ignoreUriCase"`
}

type Rewrite struct {
	URI string `json:"uri,omitempty"`
}

type CorsPolicy struct {
	AllowOrigin []string 	`json:"allowOrigin"`
	AllowMethods []string 	`json:"allowMethods"`
	AllowHeaders []string 	`json:"allowHeaders"`
	ExposeHeaders []string 	`json:"exposeHeaders"`
	MaxAge string `json:"maxAge"`
	AllowCredentials bool 	`json:"allowCredentials"`
}

type Port struct {
	Number int32 `json:"number"`
}

type Destination struct {
	Host string `json:"host"`
	Subset string `json:"subset,omitempty"`
	Port Port `json:"port"`
}

type Routes struct {
	Destination Destination `json:"destination"`
}

type HTTP struct {
	Match      []Match      `json:"match"`
	Route      []Routes 	`json:"route"`
	Rewrite    *Rewrite 	`json:"rewrite,omitempty"`
	Headers    *Headers 	`json:"headers,omitempty"`
	CorsPolicy *CorsPolicy 	`json:"corsPolicy,omitempty"`
}

type VirtualService struct {
	kubemetav1.TypeMeta 	`json:",inline"`
	kubemetav1.ObjectMeta	`json:"metadata"`
	Spec struct {
		Hosts    []string   `json:"hosts"`
		Gateways []string   `json:"gateways"`
		HTTP     []HTTP `json:"http"`
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
	if input[len(input) - 1] == '/' {
		return input
	}
	return input + "/"
}

func removeLeadingSlash(input string) string {
	if input == "" {
		return input
	}
	if input[len(input) - 1] == '/' {
		return input[:len(input) - 1]
	}
	return input
}

type IstioIngress struct {
	runtime runtime.Runtime
	config  *IngressConfig
	db      *sql.DB
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
		runtime: runtime,
		config:  config,
		db:      db,
		certificateNamespace: certificateNamespace,
	}, nil
}

func (ingress *IstioIngress) VirtualServiceExists(name string) (bool, string, error) {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - checking if virtual service exists %s\n", name)
	}
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices/" + name, nil)
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
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/"+newdomain, nil)
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
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices/" + name, nil)
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
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/"+newdomain, nil)
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
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices/" + name, nil)
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

func (ingress *IstioIngress) UpdateVirtualService(vs *VirtualService, name string) (error) {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - updating virtual service %s\n", name)
	}
	body, code, err := ingress.runtime.GenericRequest("put", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices/" + name, vs)
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
	return ingress.UpdateVirtualService(vs, app + "-" + space)
}

func (ingress *IstioIngress) InstallOrUpdateVirtualService(domain string, vs *VirtualService, exists bool) error {
	var err error = nil

	// Force good security practices
	for i, _ := range vs.Spec.HTTP {
		if vs.Spec.HTTP[i].Headers.Response.Set == nil {
			vs.Spec.HTTP[i].Headers.Response.Set = make(map[string]string)
		}
		vs.Spec.HTTP[i].Headers.Response.Set["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
	}

	if !exists {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices", vs)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create virtual service %#+v due to %s - %s\n", vs, strconv.Itoa(code), string(body))
			return errors.New("Unable to create virtual service " + vs.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/virtualservices/" + domain, vs)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to update virtual service %#+v due to %s - %s\n", vs, strconv.Itoa(code), string(body))
			return errors.New("Unable to update virtual service " + vs.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateGateway(domain string, gateway *Gateway) error {
	_, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/" + gateway.GetName(), nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		body, code, err := ingress.runtime.GenericRequest("put", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/" + gateway.GetName(), gateway)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to update gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
			return errors.New("Unable to update gateway " + gateway.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
		}
	} else {
		body, code, err := ingress.runtime.GenericRequest("post", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways", gateway)
		if err != nil {
			return err
		}
		if code != http.StatusOK && code != http.StatusCreated {
			fmt.Printf("Failed to create gateway %#+v due to %s - %s\n", gateway, strconv.Itoa(code), string(body))
			return errors.New("Unable to create gateway " + gateway.GetName() + " due to error: " + strconv.Itoa(code) + " " + string(body))
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
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/sites-" + gatewayType, nil)
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

func (ingress *IstioIngress) InstallOrUpdateJWTAuthFilter(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string, includes []string) (error) {
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio installing or updating JWT Auth filter for %s-%s with %s\n", appname, space, jwksUri)
	}
	
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioAuthenticationAPIVersion +  "/namespaces/" + space + "/policies/" + appname, nil)
	if err != nil {
		return err
	}

	var jwtPolicy Policy
	if code == http.StatusNotFound {
		jwtPolicy.Kind = "Policy"
		jwtPolicy.APIVersion = IstioAuthenticationAPIVersion
		jwtPolicy.SetName(appname)
		jwtPolicy.SetNamespace(space)
	} else {
		if err = json.Unmarshal(body, &jwtPolicy); err != nil {
			return err
		}
	}
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
	
	if len(excludes) > 0 || len(includes) > 0 {
		jwtPolicy.Spec.Origins[0].Jwt.TriggerRules = make([]TriggerRule, 1)
		jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths = make([]StringMatch, 0)
		jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].IncludedPaths = make([]StringMatch, 0)
		for _, exclude := range excludes {
			jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths = append(jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].ExcludedPaths, StringMatch{Prefix:exclude})
		}
		for _, include := range includes {
			jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].IncludedPaths = append(jwtPolicy.Spec.Origins[0].Jwt.TriggerRules[0].IncludedPaths, StringMatch{Prefix:include})
		}
	}
	if code == http.StatusNotFound {
		body, code, err = ingress.runtime.GenericRequest("post", "/apis/" + jwtPolicy.APIVersion +  "/namespaces/" + space + "/policies", jwtPolicy)
	} else {
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
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Istio - deleting any JWT Auth filter for %s-%s\n", appname, space)
	}
	body, code, err := ingress.runtime.GenericRequest("delete", "/apis/" + IstioAuthenticationAPIVersion + "/namespaces/" + space + "/policies/" + appname, nil)
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
	body, code, err := ingress.runtime.GenericRequest("get", "/apis/" + IstioNetworkingAPIVersion + "/namespaces/sites-system/gateways/sites-" + gatewayType, nil)
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

func createHTTPSpecForVS(app string, space string, domain string, adjustedPath string, rewritePath string, forwardedPath string, port int32, filters []structs.HttpFilters) HTTP {
	http := HTTP{
		Match: []Match{Match{
			URI:StringMatch{
				Prefix: forwardedPath,
			},
			IgnoreUriCase: true,
		}},
		Rewrite: &Rewrite{
			URI: rewritePath,
		},
		Route: []Routes{Routes{
			Destination: Destination{
				Host: app + "." + space + ".svc.cluster.local",
				Port: Port{
					Number: port,
				},
			},
		}},
		Headers: &Headers{
			Response: HeaderOperations{
				Set: map[string]string{
					"Strict-Transport-Security":"max-age=31536000; includeSubDomains",
				},
			},
			Request: HeaderOperations{
				Set: map[string]string{
					"X-Forwarded-Path": forwardedPath,
					"X-Orig-Path": removeLeadingSlash(adjustedPath),
					"X-Orig-Host": domain,
					"X-Orig-Port": "443",
					"X-Orig-Proto": "https",
					"X-Request-Start": "t=%START_TIME(%s.%3f)%",
				},
			},
		},
	}

	for _, filter := range filters {
		if filter.Type == "cors" {
			if os.Getenv("INGRESS_DEBUG") == "true" {
				fmt.Printf("[ingress] Adding CORS filter to site %#+v\n", filter)
			}
			allow_origin := make([]string, 0)
			allow_methods := make([]string, 0)
			allow_headers := make([]string, 0)
			expose_headers := make([]string, 0)
			max_age := time.Second * 86400
			allow_credentials := false
			if val, ok := filter.Data["allow_origin"]; ok {
				if val != "" {
					allow_origin = strings.Split(val, ",")
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
				AllowOrigin:allow_origin,
				AllowMethods:allow_methods,
				AllowHeaders:allow_headers,
				ExposeHeaders:expose_headers,
				MaxAge:max_age.String(),
				AllowCredentials:allow_credentials,
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
	
	if(internal) {
		vs.Spec.Gateways = []string{"sites-private"}
	} else {
		vs.Spec.Gateways = []string{"sites-public"}
	}
	vs.Spec.Hosts = []string{domain}
	vs.Spec.HTTP = make([]HTTP, 0)
	for _, value := range paths {
		path := removeLeadingSlash(value.Path)
		vs.Spec.HTTP = append(vs.Spec.HTTP, 
			createHTTPSpecForVS(value.App, value.Space, value.Domain, removeSlash(path), addSlash(value.ReplacePath), removeSlash(path) + "/", defaultPort, value.Filters))
		if removeSlashSlash(value.Path) == removeSlash(value.Path) {
			vs.Spec.HTTP = append(vs.Spec.HTTP, 
				createHTTPSpecForVS(value.App, value.Space, value.Domain, removeSlashSlash(value.Path), value.ReplacePath, removeSlashSlash(path), defaultPort, value.Filters))
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
		"akkeris.k8s.io/alt-names": main_server_name,
		"akkeris.k8s.io/common-name": main_server_name,
	})
	secret.SetLabels(map[string]string{
		"akkeris.k8s.io/certificate-name": main_server_name,
	})
	secret.Data = make(map[string][]byte, 0)
	secret.Data["tls.key"] = []byte(base64.StdEncoding.EncodeToString(pem_key))
	secret.Data["tls.crt"] = []byte(base64.StdEncoding.EncodeToString(pem_cert))
	return &name, &secret, nil
}

func getDownPage() string {
	if os.Getenv("ISTIO_DOWNPAGE") != "" {
		return os.Getenv("ISTIO_DOWNPAGE")
	}
	return "downpage.akkeris-system.svc.cluster.local"
}

func (ingress *IstioIngress) CreateOrUpdateRouter(domain string, internal bool, paths []Route) error {
	exists, version, err := ingress.VirtualServiceExists(domain)
	if err != nil {
		return err
	}
	vs, err := PrepareVirtualServiceForCreateorUpdate(domain, internal, paths)
	if err != nil {
		return err
	}
	if(exists && version != "") {
		vs.SetResourceVersion(version)
	}
	if err = ingress.InstallOrUpdateUberSiteGateway(domain, ingress.GetCertificateFromDomain(domain), internal); err != nil {
		return err
	}
	if err = ingress.InstallOrUpdateVirtualService(domain, vs, exists); err != nil {
		return err
	}
	return nil
}

func (ingress *IstioIngress) InstallOrUpdateCORSAuthFilter(vsname string, path string, allowOrigin []string, allowMethods []string, allowHeaders []string, exposeHeaders []string, maxAge time.Duration, allowCredentials bool) (error) {
	virtualService, err := ingress.GetVirtualService(vsname)
	if err != nil {
		// not yet deployed
		return nil
	}
	var dirty = false
	for i, http := range virtualService.Spec.HTTP {
		if http.Match == nil || len(http.Match) == 0 {
			virtualService.Spec.HTTP[0].CorsPolicy = &CorsPolicy{
				AllowOrigin:allowOrigin,
				AllowMethods:allowMethods,
				AllowHeaders:allowHeaders,
				ExposeHeaders:exposeHeaders,
				MaxAge:maxAge.String(),
				AllowCredentials:allowCredentials,
			}
			dirty = true
		} else {
			for _, match := range http.Match {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Looking to add CORS policy, comparing path: %s with match prefix %s and match exact %s\n", path, match.URI.Prefix, match.URI.Exact)
				}
				if strings.HasPrefix(match.URI.Prefix, path) || match.URI.Exact == path || match.URI.Prefix == path {
					virtualService.Spec.HTTP[i].CorsPolicy = &CorsPolicy{
						AllowOrigin:allowOrigin,
						AllowMethods:allowMethods,
						AllowHeaders:allowHeaders,
						ExposeHeaders:exposeHeaders,
						MaxAge:maxAge.String(),
						AllowCredentials:allowCredentials,
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

func (ingress *IstioIngress) DeleteCORSAuthFilter(vsname string, path string) (error) {
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

	if value {
		virtualService.Spec.HTTP[0].Route[0].Destination.Host = getDownPage()
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
	return ingress.DeleteUberSiteGateway(domain, ingress.GetCertificateFromDomain(domain), internal)
}

func (ingress *IstioIngress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error {
	name, secret, err := CertificateToSecret(server_name, pem_cert, pem_key, ingress.config.Environment)
	if err != nil {
		return err
	}
	_, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/" + ingress.certificateNamespace + "/secrets/" + *name, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		_, _, err = ingress.runtime.GenericRequest("put", "/api/v1/namespaces/" + ingress.certificateNamespace + "/secrets/" + *name, secret)
		return err
	} else {
		_, _, err = ingress.runtime.GenericRequest("post", "/api/v1/namespaces/" + ingress.certificateNamespace + "/secrets", secret)
		return err
	}
}

func (ingress *IstioIngress) GetInstalledCertificates(site string) ([]Certificate, error) {
	var certList kube.SecretList
	if site != "*" {
		main_server_name := strings.Replace(site, "*.", "star.", -1)
		main_certs_name := strings.Replace(main_server_name, ".", "-", -1) + "-tls"
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/" + ingress.certificateNamespace + "/secrets/" + main_certs_name, nil)

		if err != nil {
			return nil, err
		}
		if code == http.StatusNotFound {
			return []Certificate{}, nil
		} else if code != http.StatusOK {
			return nil, errors.New("Failure to lookup certificate: " + string(body))
		}
		var t kube.Secret
		if err = json.Unmarshal(body, &t); err != nil {
			return nil, err
		}
		certList.Items = make([]kube.Secret, 0)
		certList.Items = append(certList.Items, t)
	} else {
		body, code, err := ingress.runtime.GenericRequest("get", "/api/v1/namespaces/" + ingress.certificateNamespace + "/secrets?fieldSelector=type%3Dkubernetes.io%2Ftls", nil)
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
		pem_certs, err := base64.StdEncoding.DecodeString(string(t.Data["tls.crt"]))
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
