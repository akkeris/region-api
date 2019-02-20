package router

import (
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	utils "region-api/utils"
	"strings"
)

type Addresses struct {
	Network string
	Internal bool `json:"internal"`
	IP string `json:"ip"`
}

type Domains struct {
	Internal bool `json:"internal"`
	Value string `json:"value"`
	IPs Addresses `json:"ips"`
	Type string `json:"type"`
}

type Certificate struct {
	Alternatives []string `json:"alternatives"`
	Name string `json:"name"`
	Expires int `json:"expires"`
	Type string `json:"type"`
	Internal bool `json:"internal"`
	External bool `json:"external"`
}

type Site struct {
	Name string `json:"name"`
	IPs []Addresses `json:"ips"`
	Domains []DomainRecord `json:"domains"`
	Certificates []Certificate `json:"certificates"`
	Valid bool `json:"valid"`
}

func WildCardMatch(wildcard string, domain string) bool {
	if strings.Contains(wildcard, "*.") {
		var d = strings.Split(domain, ".")
		if len(d) > 1 {
			d = d[1:]
		}
		if strings.Join(d, ".") == strings.Replace(wildcard, "*.", "", 1) {
			return true
		}
	} else if wildcard == domain {
		return true
	}
	return false
}

func HttpGetSite(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	var site = strings.Trim(strings.ToLower(params["site"]), " ")
	var domain = strings.Split(site, ".")
	if len(domain) > 2 {
		domain = domain[len(domain)-2:]
	}
	dzones, err := dns.Domain(strings.Join(domain, "."))
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	certs, ips, err := GetF5SiteInfo(site)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	domains := make([]DomainRecord, 0)
	for _, dz := range dzones {
		records, err := dns.DomainRecords(dz)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, record := range records {
			for _, ip := range record.Values {
				for _, vip := range ips {
					if WildCardMatch(record.Name, site) && vip.IP == ip {
						domains = append(domains, record)
					}
				}
			}
		}
	}

	r.JSON(http.StatusOK, Site{
		Name:site,
		Domains:domains,
		Certificates:certs,
		IPs:ips,
		Valid:len(certs) > 0 && len(domains) > 0 && len(ips) > 0,
	})
}
