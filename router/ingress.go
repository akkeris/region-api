package router

import (
	"database/sql"
	"errors"
	"strings"
	"net/url"
	"os"
	"time"
)

type Route struct {
	Domain      string `json:"domain"`
	Path        string `json:"path"`
	Space       string `json:"space"`
	App         string `json:"app"`
	ReplacePath string `json:"replacepath"`
	Port        string `json:"port"`
}

type Router struct {
	Domain          string           `json:"domain"`
	Internal        bool             `json:"internal"`
	VSNamespace     string           `json:"vsnamespace"`
	ResourceVersion string           `json:"resourceVersion"`
	Paths           []Route 		 `json:"paths"`
}

type Ingress interface {
	InstallOrUpdateCORSAuthFilter(domain string, path string, allowOrigin []string, allowMethods []string, allowHeaders []string, exposeHeaders []string, maxAge time.Duration, allowCredentials bool) (error)
	InstallOrUpdateJWTAuthFilter(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string, includes []string) (error)
	DeleteCORSAuthFilter(vsname string, path string) (error)
	DeleteJWTAuthFilter(appname string, space string, fqdn string, port int64) (error)
	SetMaintenancePage(app string, space string, value bool) error
	GetMaintenancePageStatus(app string, space string) (bool, error)
	DeleteRouter(domain string, internal bool) error
	CreateOrUpdateRouter(domain string, internal bool, paths []Route) (error)
	InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error
	GetInstalledCertificates(site string) ([]Certificate, error)
	Config() *IngressConfig
	Name() string
}

type IngressesConfig struct {
	AppsPublicInternal   *IngressConfig `json:"apps_public_internal"`
	AppsPublicExternal   *IngressConfig `json:"apps_public_external"`
	AppsPrivateInternal  *IngressConfig `json:"apps_private_interal"`
	SitesPublicInternal  *IngressConfig `json:"sites_public_internal"`
	SitesPublicExternal  *IngressConfig `json:"sites_public_external"`
	SitesPrivateInternal *IngressConfig `json:"sites_private_interal"`
}

type IngressConfig struct {
	Device      string `json:"device"`
	Address     string `json:"address"`
	Environment string `json:"environment"`
	Name        string `json:name`
}

func urlToIngressConfig(uri string) ([]*IngressConfig, error) {
	if strings.Contains(uri, ",") {
		uris := strings.Split(uri, ",")
		u1, err := urlToIngressConfig(uris[0])
		if err != nil {
			return nil, err
		}
		u2, err := urlToIngressConfig(uris[1])
		if err != nil {
			return nil, err
		}
		return append(u1, u2[0]), nil
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	components := strings.Split(u.Path, "/")
	if len(components) != 3 {
		return nil, errors.New("The ingress configuration provided " + uri + " was invalid.")
	}
	if components[0] != "" {
		return nil, errors.New("The ingress config provided " + uri + " was invalid.")
	}
	if strings.ToLower(u.Scheme) != "istio" {
		return nil, errors.New("The ingress " + uri + " contains an invalid ingress type, must be istio.")
	}
	if u.Host == "" {
		return nil, errors.New("The ingress " + uri + " contains an invalid address for the ingress.")
	}
	return []*IngressConfig{&IngressConfig{
		Device:      strings.ToLower(u.Scheme),
		Address:     strings.ToLower(u.Host),
		Environment: components[1],
		Name:        components[2],
	}}, nil
}

func getAppsIngressPublicInternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("APPS_PUBLIC_INTERNAL"))
}
func getAppsIngressPublicExternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("APPS_PUBLIC_EXTERNAL"))
}
func getAppsIngressPrivateInternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("APPS_PRIVATE_INTERNAL"))
}
func getSitesIngressPublicInternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("SITES_PUBLIC_INTERNAL"))
}
func getSitesIngressPublicExternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("SITES_PUBLIC_EXTERNAL"))
}
func getSitesIngressPrivateInternal() ([]*IngressConfig, error) {
	return urlToIngressConfig(os.Getenv("SITES_PRIVATE_INTERNAL"))
}

type FullIngressConfig struct {
	PublicInternal IngressConfig
	PublicExternal IngressConfig
	PrivateInternal IngressConfig
}

func GetDefaultIngressSiteAddresses() (*FullIngressConfig, error) {
	var config FullIngressConfig 
	spi, err := getSitesIngressPublicInternal()
	if err != nil {
		return nil, err
	}
	config.PublicInternal = *spi[0]
	spe, err := getSitesIngressPublicExternal()
	if err != nil {
		return nil, err
	}
	config.PublicExternal = *spe[0]
	spri, err := getSitesIngressPrivateInternal()
	if err != nil {
		return nil, err
	}
	config.PrivateInternal = *spri[0]
	return &config, nil
}

func getIngress(db *sql.DB, internal bool, configs []*IngressConfig) (Ingress, error) {
	if configs[0].Device == "istio" {
		ing, err := GetIstioIngress(db, configs[0])
		if err != nil {
			return nil, err
		}
		var in Ingress = Ingress(ing)
		return in, nil
	} else {
		return nil, errors.New("Unable to find ingress for " + configs[0].Device)
	}
}

var internalAppIngress *Ingress = nil
var externalAppIngress *Ingress = nil
var internalSiteIngress *Ingress = nil
var externalSiteIngress *Ingress = nil

func GetAppIngress(db *sql.DB, internal bool) (Ingress, error) {
	if internal {
		if internalAppIngress != nil {
			return *internalAppIngress, nil
		}
		configs, err := getAppsIngressPrivateInternal()
		if err != nil {
			return nil, err
		}
		ing, err := getIngress(db, internal, configs)
		if err != nil {
			return nil, err
		}
		internalAppIngress = &ing
		return *internalAppIngress, nil
	} else {
		if externalAppIngress != nil {
			return *externalAppIngress, nil
		}
		configs, err := getAppsIngressPublicExternal()
		if err != nil {
			return nil, err
		}
		ing, err := getIngress(db, internal, configs)
		if err != nil {
			return nil, err
		}
		externalAppIngress = &ing
		return *externalAppIngress, nil
	}
}

func GetSiteIngress(db *sql.DB, internal bool) (Ingress, error) {
	if internal {
		if internalSiteIngress != nil {
			return *internalSiteIngress, nil
		}
		configs, err := getSitesIngressPrivateInternal()
		if err != nil {
			return nil, err
		}
		ing, err := getIngress(db, internal, configs)
		if err != nil {
			return nil, err
		}
		internalSiteIngress = &ing
		return *internalSiteIngress, nil
	} else {
		if externalSiteIngress != nil {
			return *externalSiteIngress, nil
		}
		configs, err := getSitesIngressPublicExternal()
		if err != nil {
			return nil, err
		}
		ing, err := getIngress(db, internal, configs)
		if err != nil {
			return nil, err
		}
		externalSiteIngress = &ing
		return *externalSiteIngress, nil
	}
}


func getIngressType(configs []*IngressConfig, ingress string) (*IngressConfig, error) {
	if configs[0].Device == ingress {
		return configs[0], nil
	} else {
		return nil, errors.New("Cannot find ingress " + ingress +" within transition config")
	}
}

func TransitionAppToIngress(db *sql.DB, ingress string, internal bool, appFQDN string) (error) {
	publicInternals, err := getAppsIngressPublicInternal()
	if err != nil {
		return err
	}
	publicExternals, err := getAppsIngressPublicExternal()
	if err != nil {
		return err
	}
	privateInternals, err := getAppsIngressPrivateInternal()
	if err != nil {
		return err
	}
	publicInternal, err := getIngressType(publicInternals, ingress)
	if err != nil {
		return err
	}
	publicExternal, err := getIngressType(publicExternals, ingress)
	if err != nil {
		return err
	}
	privateInternal, err := getIngressType(privateInternals, ingress)
	if err != nil {
		return err
	}

	configs := FullIngressConfig{
		PublicInternal: *publicInternal,
		PublicExternal: *publicExternal,
		PrivateInternal: *privateInternal,
	}

	if err := SetDomainName(&configs, appFQDN, internal); err != nil {
		return err
	}
	return nil
}

func TransitionSiteToIngress(db *sql.DB, ingress string, internal bool, siteFQDN string) (error) {
	publicInternals, err := getSitesIngressPublicInternal()
	if err != nil {
		return err
	}
	publicExternals, err := getSitesIngressPublicExternal()
	if err != nil {
		return err
	}
	privateInternals, err := getSitesIngressPrivateInternal()
	if err != nil {
		return err
	}
	publicInternal, err := getIngressType(publicInternals, ingress)
	if err != nil {
		return err
	}
	publicExternal, err := getIngressType(publicExternals, ingress)
	if err != nil {
		return err
	}
	privateInternal, err := getIngressType(privateInternals, ingress)
	if err != nil {
		return err
	}

	configs := FullIngressConfig{
		PublicInternal: *publicInternal,
		PublicExternal: *publicExternal,
		PrivateInternal: *privateInternal,
	}

	if err := SetDomainName(&configs, siteFQDN, internal); err != nil {
		return err
	}
	return nil
}