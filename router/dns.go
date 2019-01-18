package router

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"os"
	"regexp"
	utils "region-api/utils"
	"strings"
	"sync"
	"time"
)

type Domain struct {
	ProviderId  string            `json:"provider_id"`
	Name        string            `json:"name"`
	Public      bool              `json:"public"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      string            `json:"status"`
	RecordCount int64             `json:"record_count"`
}

type DomainRecord struct {
	Type   string   `json:"type"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Domain *Domain  `json:"domain",omitempty`
}

// Domain.Status = available

type DNSProvider interface {
	Type() string
	Domains() ([]Domain, error)
	Domain(domain string) ([]Domain, error)
	DomainRecords(domain Domain) ([]DomainRecord, error)
	CreateDomainRecord(domain Domain, recordType string, name string, values []string) error
	RemoveDomainRecord(domain Domain, recordType string, name string, values []string) error
}

func HttpGetDomains(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domains()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, domains)
}

func HttpGetDomain(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}
	r.JSON(http.StatusOK, domains)
}

func HttpGetDomainRecords(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}
	records := make([]DomainRecord, 0)
	for _, domain := range domains {
		record, err := dns.DomainRecords(domain)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, r := range record {
			r.Domain = &Domain{
				ProviderId:  domain.ProviderId,
				Name:        domain.Name,
				Public:      domain.Public,
				Metadata:    domain.Metadata,
				Status:      domain.Status,
				RecordCount: domain.RecordCount,
			}
			records = append(records, r)
		}
	}
	r.JSON(http.StatusOK, records)
}

func HttpCreateDomainRecords(params martini.Params, spec DomainRecord, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if strings.ToUpper(spec.Type) != "A" && strings.ToUpper(spec.Type) != "AAAA" && strings.ToUpper(spec.Type) != "CNAME" {
		utils.ReportInvalidRequest("Only A, AAAA and CNAME records may be added to a domain record.", r)
		return
	}
	if strings.ToUpper(strings.Trim(params["domain"], ".")) == strings.ToUpper(strings.Trim(spec.Name, ".")) {
		utils.ReportInvalidRequest("Entries cannot be created for the root domain.", r)
		return
	}
	if spec.Name == "" {
		utils.ReportInvalidRequest("Entries cannot be created for the root domain.", r)
		return
	}

	dns := GetDnsProvider()
	domains, err := dns.Domain(params["domain"])
	records := make([]DomainRecord, 0)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}

	for _, domain := range domains {
		if err = dns.CreateDomainRecord(domain, spec.Type, spec.Name, spec.Values); err != nil {
			utils.ReportError(err, r)
			return
		}
		records = append(records, DomainRecord{
			Type:   spec.Type,
			Name:   spec.Name,
			Values: spec.Values,
			Domain: &Domain{
				ProviderId:  domain.ProviderId,
				Name:        domain.Name,
				Public:      domain.Public,
				Metadata:    domain.Metadata,
				Status:      domain.Status,
				RecordCount: domain.RecordCount,
			},
		})
	}
	r.JSON(http.StatusCreated, records)
}

func HttpRemoveDomainRecords(params martini.Params, r render.Render) {
	dns := GetDnsProvider()
	if strings.Contains(strings.ToLower(params["domain"]), strings.ToLower(params["name"])) || strings.ToLower(params["name"]) == strings.ToLower(params["domain"]) {
		r.JSON(http.StatusConflict, map[string]interface{}{"error": "CONFLICT", "error_description": "The name entry to delete was the domain itself."})
		return
	}
	if strings.ToUpper(strings.Trim(params["domain"], ".")) == strings.ToUpper(strings.Trim(params["name"], ".")) {
		utils.ReportInvalidRequest("Entries cannot be removed for the root of the domain.", r)
		return
	}
	if params["name"] == "" {
		utils.ReportInvalidRequest("Entries cannot be removed for the root of the domain.", r)
		return
	}

	domains, err := dns.Domain(params["domain"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(domains) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["domain"] + " was not found."})
		return
	}

	toRemoveRecords := make([]DomainRecord, 0)
	for _, domain := range domains {
		records, err := dns.DomainRecords(domain)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		for _, record := range records {
			// we only allow removal of A or CNAME records
			if record.Name == params["name"] && (strings.ToUpper(record.Type) == "A" || strings.ToUpper(record.Type) == "AAAA" || strings.ToUpper(record.Type) == "CNAME") {
				record.Domain = &Domain{
					ProviderId:  domain.ProviderId,
					Name:        domain.Name,
					Public:      domain.Public,
					Metadata:    domain.Metadata,
					Status:      domain.Status,
					RecordCount: domain.RecordCount,
				}
				toRemoveRecords = append(toRemoveRecords, record)
			}
		}
	}

	if len(toRemoveRecords) == 0 {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NOT_FOUND", "error_description": "The domain " + params["name"] + " was not found."})
		return
	}

	for _, rrec := range toRemoveRecords {
		if err = dns.RemoveDomainRecord(*rrec.Domain, rrec.Type, rrec.Name, rrec.Values); err != nil {
			utils.ReportError(err, r)
			return
		}
	}

	r.JSON(http.StatusCreated, toRemoveRecords)
}

func GetDnsProvider() DNSProvider {
	// We could add a switch here for a different dns provider,
	// but as of now the only dns provider supported is AWS.
	return NewAwsDNSProvider()
}

type AwsDNSProvider struct {
	client             *route53.Route53
	domainCache        *[]Domain
	domainRecordsCache *map[string][]DomainRecord
	mutex              *sync.Mutex
}

var provider *AwsDNSProvider = nil
var provider_mutex = &sync.Mutex{}

func AwsRefreshCache() {
	/*
	 * BE VERY, VERY CAREFUL CHANGING THIS, IT COULD CREATE DEADLOCKS WITH THE MUTEX.
	 */
	if provider == nil {
		return
	}
	provider.mutex.Lock()
	provider.domainCache = nil
	provider.domainRecordsCache = nil
	provider.mutex.Unlock()
	domainCache, err := provider.Domains()
	provider.mutex.Lock()
	if err != nil {
		fmt.Println("Error refreshing domain cache: " + err.Error())
		provider.mutex.Unlock()
		return
	}
	domainRecordsCache := make(map[string][]DomainRecord, 0)
	for _, domain := range domainCache {
		provider.mutex.Unlock()
		domainRecordsCache[domain.ProviderId], err = provider.DomainRecords(domain)
		provider.mutex.Lock()
		if err != nil {
			fmt.Println("Error refreshing domain records cache: " + err.Error())
			provider.mutex.Unlock()
			return
		}
	}
	provider.domainCache = &domainCache
	provider.domainRecordsCache = &domainRecordsCache
	provider.mutex.Unlock()
}

func NewAwsDNSProvider() *AwsDNSProvider {
	provider_mutex.Lock()
	defer provider_mutex.Unlock()
	if provider != nil {
		return provider
	}
	provider = &AwsDNSProvider{
		client: route53.New(session.New(&aws.Config{
			Region: aws.String(os.Getenv("REGION")),
		})),
		domainCache:        nil,
		domainRecordsCache: nil,
		mutex:              &sync.Mutex{},
	}
	AwsRefreshCache()
	t := time.NewTicker(time.Minute * 60)
	go (func() {
		for {
			<-t.C
			AwsRefreshCache()
		}
	})()
	return provider
}

func StringNilToEmpty(value *string) string {
	if value == nil {
		return ""
	} else {
		return *value
	}
}
func BoolNilToFalse(value *bool) bool {
	if value == nil {
		return false
	} else {
		return *value
	}
}

func FixAwsDomainName(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return strings.ToLower(strings.TrimRight(domain, "."))
	} else {
		return strings.ToLower(domain)
	}
}

func MapResourceRecordToStringArray(records []*route53.ResourceRecord) []string {
	var values []string = make([]string, 0)
	for _, s := range records {
		values = append(values, StringNilToEmpty(s.Value))
	}
	return values
}

func MapValuesToResourceRecord(values []string) []*route53.ResourceRecord {
	resource := make([]*route53.ResourceRecord, 0)
	for _, v := range values {
		resource = append(resource, &route53.ResourceRecord{Value: aws.String(v)})
	}
	return resource
}

func IsDomainInBlacklist(identifier string) bool {
	banned := strings.Split(os.Getenv("DOMAIN_BLACKLIST"), ",")
	identifier = strings.Trim(strings.ToLower(identifier), " ")
	for _, val := range banned {
		val = strings.Trim(strings.ToLower(val), " ")
		if val != "" {
			reg, err := regexp.Compile(val)
			if err == nil && reg.MatchString(identifier) {
				return true
			}
		}
	}
	return false
}

func MapAwsHostedZonesToDomains(awsDomains []*route53.HostedZone) []Domain {
	var domains []Domain = make([]Domain, 0)
	for _, d := range awsDomains {
		domains = append(domains, Domain{
			ProviderId: StringNilToEmpty(d.Id),
			Name:       FixAwsDomainName(StringNilToEmpty(d.Name)),
			Public:     !BoolNilToFalse(d.Config.PrivateZone),
			Metadata: map[string]string{
				"CallerReference": StringNilToEmpty(d.CallerReference),
				"Comment":         StringNilToEmpty(d.Config.Comment),
			},
			Status:      "available",
			RecordCount: *d.ResourceRecordSetCount,
		})
	}
	return domains
}

func MapAwsResourceRecordsToDomainRecords(results []*route53.ResourceRecordSet) []DomainRecord {
	var domainRecords []DomainRecord = make([]DomainRecord, 0)
	for _, d := range results {
		domainRecords = append(domainRecords, DomainRecord{
			Type:   StringNilToEmpty(d.Type),
			Name:   FixAwsDomainName(StringNilToEmpty(d.Name)),
			Values: MapResourceRecordToStringArray(d.ResourceRecords),
		})
	}
	return domainRecords
}

func (dnsProvider *AwsDNSProvider) Type() string {
	return "aws"
}

func (dnsProvider *AwsDNSProvider) Domains() ([]Domain, error) {
	dnsProvider.mutex.Lock()
	defer dnsProvider.mutex.Unlock()
	if dnsProvider.domainCache != nil {
		return *dnsProvider.domainCache, nil
	}
	var domains []Domain = make([]Domain, 0)
	var Marker *string = nil
	t := time.NewTicker(time.Millisecond * 500)
	for more := true; more == true; {
		<-t.C // in order to not exceed our throttle rate
		result, err := dnsProvider.client.ListHostedZones(&route53.ListHostedZonesInput{
			Marker:   Marker,
			MaxItems: aws.String("100"),
		})
		if err != nil {
			return nil, err
		}
		if result.Marker != nil && result.IsTruncated != nil && *result.IsTruncated == true {
			Marker = result.Marker
		} else {
			more = false
		}
		for _, prop := range MapAwsHostedZonesToDomains(result.HostedZones) {
			if !IsDomainInBlacklist(prop.Name) && !IsDomainInBlacklist(prop.ProviderId) {
				domains = append(domains, prop)
			}
		}
	}
	return domains, nil
}

func (dnsProvider *AwsDNSProvider) Domain(domain string) ([]Domain, error) {
	// domain can be a full sub-domain such as www.example.com, extract only the last two parts.
	dp := strings.Split(domain, ".")
	if len(dp) > 2 {
		domain = strings.ToLower(strings.Join(dp[(len(dp)-2):], "."))
	} else {
		domain = strings.Join(dp, ".")
	}
	allDomains, err := dnsProvider.Domains()
	if err != nil {
		return nil, err
	}
	domains := make([]Domain, 0)
	for _, d := range allDomains {
		if d.Name == domain || d.ProviderId == ("/hostedzone/"+domain) || d.ProviderId == domain {
			domains = append(domains, d)
		}
	}
	return domains, nil
}

func (dnsProvider *AwsDNSProvider) DomainRecords(domain Domain) ([]DomainRecord, error) {
	dnsProvider.mutex.Lock()
	defer dnsProvider.mutex.Unlock()
	if dnsProvider.domainRecordsCache != nil {
		var cached map[string][]DomainRecord = *dnsProvider.domainRecordsCache
		return cached[domain.ProviderId], nil
	}
	var domainRecords []DomainRecord = make([]DomainRecord, 0)
	var nextRecordName *string = nil
	var nextRecordType *string = nil
	var nextRecordIdentifier *string = nil
	t := time.NewTicker(time.Millisecond * 500)
	for more := true; more == true; {
		<-t.C // in order to not exceed our throttle rate
		result, err := dnsProvider.client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
			MaxItems:              aws.String("100"),
			HostedZoneId:          aws.String(domain.ProviderId),
			StartRecordName:       nextRecordName,
			StartRecordType:       nextRecordType,
			StartRecordIdentifier: nextRecordIdentifier,
		})
		if err != nil {
			return nil, err
		}
		if result.NextRecordName != nil && result.IsTruncated != nil && *result.IsTruncated == true {
			nextRecordName = result.NextRecordName
			nextRecordType = result.NextRecordType
			nextRecordIdentifier = result.NextRecordIdentifier
		} else {
			more = false
		}
		domainRecords = append(domainRecords, MapAwsResourceRecordsToDomainRecords(result.ResourceRecordSets)...)
	}
	return domainRecords, nil
}

func (dnsProvider *AwsDNSProvider) CreateDomainRecord(domain Domain, recordType string, name string, values []string) error {
	name = strings.ToLower(name)
	_, err := dnsProvider.client.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(domain.ProviderId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            aws.String(name),
						Type:            aws.String(recordType),
						TTL:             aws.Int64(300),
						ResourceRecords: MapValuesToResourceRecord(values),
					},
				},
			},
		},
	})
	if err == nil {
		dnsProvider.mutex.Lock()
		if provider.domainRecordsCache == nil {
			dnsProvider.mutex.Unlock()
			return nil
		}
		// ensure we remove the old record, if it exists
		domains := make([]DomainRecord, 0)
		prev := (*provider.domainRecordsCache)[domain.ProviderId]
		for _, d := range prev {
			if(d.Name != name && d.Type != recordType) {
				domains = append(domains, d)
			}
		}
		(*provider.domainRecordsCache)[domain.ProviderId] = domains

		// now add the new record
		(*provider.domainRecordsCache)[domain.ProviderId] = append((*provider.domainRecordsCache)[domain.ProviderId], DomainRecord{
			Type: recordType,
			Name: name,
			Values: values,
			Domain: &domain,
		})
		dnsProvider.mutex.Unlock()
	}
	return err
}

func (dnsProvider *AwsDNSProvider) RemoveDomainRecord(domain Domain, recordType string, name string, values []string) error {
	name = strings.ToLower(name)
	_, err := dnsProvider.client.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(domain.ProviderId),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String("DELETE"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            aws.String(name),
						Type:            aws.String(recordType),
						TTL:             aws.Int64(300),
						ResourceRecords: MapValuesToResourceRecord(values),
					},
				},
			},
		},
	})
	if err == nil {
		dnsProvider.mutex.Lock()
		if provider.domainRecordsCache == nil {
			dnsProvider.mutex.Unlock()
			return nil
		}
		domains := make([]DomainRecord, 0)
		prev := (*provider.domainRecordsCache)[domain.ProviderId]
		for _, d := range prev {
			if(d.Name != name && d.Type != recordType) {
				domains = append(domains, d)
			}
		}
		(*provider.domainRecordsCache)[domain.ProviderId] = domains
		dnsProvider.mutex.Unlock()
	}
	return err
}
