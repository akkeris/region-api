package router

import (
	"database/sql"
	"fmt"
	structs "region-api/structs"
	"os"
	"strings"
)

/*
 * This is a transition ingress for moving from F5 to istio
 */
type TransitionIngress struct {
	f5 *F5Ingress
	istio *IstioIngress
	db *sql.DB
}

func IsMatch(alts1 []string, alts2 []string) (bool) {
	for _, a := range alts1 {
		found := false
		for _, b := range alts2 {
			if a == b || "*." + a == b || a == "*." + b {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	for _, a := range alts2 {
		found := false
		for _, b := range alts1 {
			if a == b || "*." + a == b || a == "*." + b {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func PerformAudit(db *sql.DB, istio *IstioIngress, f5 *F5Ingress) {
	istioCerts, err := istio.GetInstalledCertificates("*")
	if err != nil {
		fmt.Printf("[ingress] Error during audit, unable to get installed certificates on istio: %s\n", err.Error())
	}
	f5Certs, err := f5.GetInstalledCertificates("*")
	if err != nil {
		fmt.Printf("[ingress] Error during audit, unable to get installed certificates on f5: %s\n", err.Error())
	}
	istioMissingCertificates := make([]Certificate, 0)
	f5MissingCertificates := make([]Certificate, 0)

	for i, ic := range istioCerts {
		if ic.Expired == false {
			var foundCertificate int = -1
			for _, fc := range f5Certs {
				if fc.Expired == false && IsMatch(ic.Alternatives, fc.Alternatives) && ic.Type == fc.Type {
					foundCertificate = i
				}
			}
			if foundCertificate == -1 {
				f5MissingCertificates = append(f5MissingCertificates, ic)
			}
		}
	}

	for i, fc := range f5Certs {
		if fc.Expired == false {
			var foundCertificate int = -1
			for _, ic := range istioCerts {
				if ic.Expired == false && IsMatch(ic.Alternatives, fc.Alternatives) && ic.Type == fc.Type {
					foundCertificate = i
				}
			}
			if foundCertificate == -1 {
				istioMissingCertificates = append(istioMissingCertificates, fc)
			}
		}
	}

	if len(istioMissingCertificates) != 0 {
		fmt.Printf("[ingress] === Warning, istio was missing some certificates that were on the F5: ===\n")
		for _, c := range istioMissingCertificates {
			fmt.Printf("[ingress] Name: %s Domains: %s Expires: %d Type: %s\n", c.Name, strings.Join(c.Alternatives, ","), c.Expires, c.Type)
		}
		fmt.Printf("[ingress]\n")
	}
	if len(f5MissingCertificates) != 0 {
		fmt.Printf("[ingress] === Warning, f5 was missing some certificates that were on istio: ===\n")
		for _, c := range f5MissingCertificates {
			fmt.Printf("[ingress] Name: %s Domains: %s Expires: %d Type: %s\n", c.Name, strings.Join(c.Alternatives, ","), c.Expires, c.Type)
		}
		fmt.Printf("[ingress]\n")
	}
}

func GetTransitionIngress(db *sql.DB, istio *IstioIngress, f5 *F5Ingress)  (*TransitionIngress, error) {
	if os.Getenv("DEFAULT_TRANSITION_INGRESS") == "" {
		fmt.Printf("[ingress] Warning: The default transition ingress was not specified!\n")
	}
	if os.Getenv("INGRESS_DEBUG") == "true" {
		fmt.Printf("[ingress] Initializing transition ingress with default %s\n", os.Getenv("DEFAULT_TRANSITION_INGRESS"))
	}
	go PerformAudit(db, istio, f5)
	return &TransitionIngress {
		f5: f5,
		istio: istio,
		db: db,
	}, nil
}

func (ingress *TransitionIngress) SetMaintenancePage(app string, space string, value bool) error {
	errF5 := ingress.f5.SetMaintenancePage(app, space, value)
	if errF5 != nil {
		fmt.Printf("Error in f5 trying to apply SetMaintenancePage to: %s %s %v: %s\n", app, space, value, errF5.Error())
	}
	errIstio := ingress.istio.SetMaintenancePage(app, space, value)
	if errIstio != nil {
		fmt.Printf("Error in istio trying to apply SetMaintenancePage to %s %s %v: %s\n", app, space, value, errIstio.Error())
	}
	if errF5 == nil && errIstio != nil {
		return errIstio
	} else if errF5 != nil && errIstio == nil {
		return errF5
	} else if errF5 != nil && errIstio != nil {
		return fmt.Errorf("Error in both istio and F5 trying to set maintenace page to %s %s %v: istio[%s] f5[%s]\n", app, space, value, errIstio.Error(), errF5.Error())
	} else {
		return nil
	}
}

func (ingress *TransitionIngress) GetMaintenancePageStatus(app string, space string) (bool, error) {
	status, err := ingress.f5.GetMaintenancePageStatus(app, space)
	if err != nil {
		fmt.Printf("Error in f5 trying to GetMaintenancePageStatus to: %s %s: %s\n", app, space, err.Error())
	}
	statusIstio, errIstio := ingress.istio.GetMaintenancePageStatus(app, space)
	if errIstio != nil {
		fmt.Printf("Error in istio trying to get GetMaintenancePageStatus to %s %s: %s\n", app, space, errIstio.Error())
		return false, errIstio
	}
	if status != statusIstio {
		fmt.Printf("Warning: istio disagreed with f5 as to whether the maintenance page was up or not. f5: %v, istio: %v for %s %s\n", status, statusIstio, app, space)
	}
	if err != nil {
		return false, err
	}
	if os.Getenv("DEFAULT_TRANSITION_INGRESS") == "istio" {
		return statusIstio, errIstio
	}
	return status, err
}

func (ingress *TransitionIngress) DeleteRouter(router structs.Routerspec) error {
	err := ingress.f5.DeleteRouter(router)
	if err != nil {
		fmt.Printf("Error in f5 trying to DeleteRouter to: %v: %s\n", router, err.Error())
	}
	if err := ingress.istio.DeleteRouter(router); err != nil {
		fmt.Printf("Error in istio trying to DeleteRouter to: %v: %s\n", router, err.Error())
		return err
	}
	return err
}

func (ingress *TransitionIngress) CreateOrUpdateRouter(router structs.Routerspec) error {
	err := ingress.f5.CreateOrUpdateRouter(router)
	if err != nil {
		fmt.Printf("Error in f5 trying to CreateOrUpdateRouter to: %v: %s\n", router, err.Error())
	}
	if err := ingress.istio.CreateOrUpdateRouter(router); err != nil {
		fmt.Printf("Error in istio trying to CreateOrUpdateRouter to: %v: %s\n", router, err.Error())
		return err
	}
	return err
}

func (ingress *TransitionIngress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) error {
	err := ingress.f5.InstallCertificate(server_name, pem_cert, pem_key)
	if err != nil {
		fmt.Printf("Error in f5 trying to InstallCertificate for: %s: %s\n", server_name, err.Error())
	}
	if err := ingress.istio.InstallCertificate(server_name, pem_cert, pem_key); err != nil {
		fmt.Printf("Error in istio trying to InstallCertificate for: %s: %s\n", server_name, err.Error())
		return err
	}
	return err
}

func (ingress *TransitionIngress) InstallOrUpdateJWTAuthFilter(appname string, space string, fqdn string, port int64, issuer string, jwksUri string, audiences []string, excludes []string) (error) {
	if err := ingress.f5.InstallOrUpdateJWTAuthFilter(appname, space, fqdn, port, issuer, jwksUri, audiences, excludes); err != nil {
		return err
	}
	return ingress.istio.InstallOrUpdateJWTAuthFilter(appname, space, fqdn, port, issuer, jwksUri, audiences, excludes)
}

func (ingress *TransitionIngress) DeleteJWTAuthFilter(appname string, space string, fqdn string, port int64) (error) {
	if err := ingress.f5.DeleteJWTAuthFilter(appname, space, fqdn, port); err != nil {
		return err
	}
	return ingress.istio.DeleteJWTAuthFilter(appname, space, fqdn, port)
}

func (ingress *TransitionIngress) GetInstalledCertificates(site string) ([]Certificate, error) {
	certs, err := ingress.f5.GetInstalledCertificates(site)
	if err != nil {
		fmt.Printf("Error in f5 trying to GetInstalledCertificates for: %s: %s\n", site, err.Error())
	}
	certsIstio, errIstio := ingress.istio.GetInstalledCertificates(site)
	if errIstio != nil {
		fmt.Printf("Error in istio trying to GetInstalledCertificates for: %s: %s\n", site, errIstio.Error())
		return nil, errIstio
	}
	if len(certsIstio) != len(certs) {
		fmt.Printf("Warning istio did not have the same certificate count for %s as the F5. F5 had %d != %d\n", site, len(certs), len(certsIstio))
	}
	if err != nil {
		return nil, err
	}
	if os.Getenv("DEFAULT_TRANSITION_INGRESS") == "istio" {
		return certsIstio, nil
	}
	return certs, nil
}

func (ingress *TransitionIngress) Config() *IngressConfig {
	// Generally used to determine DNS for new sites, we'll use f5
	if os.Getenv("DEFAULT_TRANSITION_INGRESS") == "istio" {
		return ingress.istio.Config()
	}
	return ingress.f5.Config()
}

func (ingress *TransitionIngress) Name() string {
	return "transition"
}

// TODO: Detail with auto install behavior of cert manager vs. digicert.