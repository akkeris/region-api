package router

import (
	"fmt"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"net"
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
	DomainRecord(domain Domain, recordType string, name string) (*DomainRecord, error)
	DomainRecords(domain Domain) ([]DomainRecord, error)
	CreateDomainRecord(domain Domain, recordType string, name string, values []string) error
	RemoveDomainRecord(domain Domain, recordType string, name string, values []string) error
}

func GetDNSRecordType(address string) string {
	recType := "A"
	if net.ParseIP(address) == nil {
		recType = "CNAME"
	}
	return recType
}

func ResolveDNS(address string, private bool) ([]string, error) {
	resolver := net.DefaultResolver
	if !private {
		resolver = &net.Resolver{
			PreferGo:true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
	            d := net.Dialer{}
	            nameserver := "8.8.8.8"
	            if os.Getenv("PUBLIC_DNS_RESOLVER") != "" {
	            	nameserver = os.Getenv("PUBLIC_DNS_RESOLVER")
	            }
	            return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
	        },
		}
	}
	result, err := resolver.LookupHost(context.Background(), address)
	if err != nil {
		r, err := resolver.LookupCNAME(context.Background(), address)
		if err != nil {
			return nil, err
		}
		result = []string{r}
	}
	return result, nil
}

func SetDomainName(config *FullIngressConfig, fqdn string, internal bool) (error) {
	dns := GetDnsProvider()
	domains, err := dns.Domain(fqdn)
	if err != nil {
		return err
	}

	// Set the DNS 
	for _, domain := range domains {
		if domain.Public && !internal {
			record, err := ResolveDNS(fqdn, false)
			if err == nil && record[0] == config.PublicExternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public external address was unnecessary, it already is set: %s == %s\n", fqdn, config.PublicExternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public external address to: %s = %s\n", fqdn, config.PublicExternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PublicExternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PublicExternal.Address), fqdn, []string{config.PublicExternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create public (external) dns: %s", err.Error())
			}
		}
		if !domain.Public && !internal {
			record, err := ResolveDNS(fqdn, true)
			if err == nil && record[0] == config.PublicInternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public internal address was unnecessary, it already is set: %s == %s\n", fqdn, config.PublicInternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting public internal address to: %s = %s\n", fqdn, config.PublicInternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PublicInternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PublicInternal.Address), fqdn, []string{config.PublicInternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create private (external) dns: %s", err.Error())
			}
		}
		if !domain.Public && internal {
			record, err := ResolveDNS(fqdn, true)
			if err == nil && record[0] == config.PrivateInternal.Address {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting private internal address was unnecessary, it already is set: %s == %s\n", fqdn, config.PrivateInternal.Address)
				}
				continue;
			} else {
				if os.Getenv("INGRESS_DEBUG") == "true" {
					fmt.Printf("[ingress] Setting private internal address to: %s = %s\n", fqdn, config.PrivateInternal.Address)
					if err != nil {
						fmt.Printf("[ingress]  - Because: %s\n", err.Error())
					} else {
						fmt.Printf("[ingress]  - Due to %#+v != %s\n", record, config.PrivateInternal.Address)
					}
				}
			}
			if err := dns.CreateDomainRecord(domain, GetDNSRecordType(config.PrivateInternal.Address), fqdn, []string{config.PrivateInternal.Address}); err != nil {
				return fmt.Errorf("Error: Failed to create private (internal) dns: %s", err.Error())
			}
		}
	}
	return nil
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
		return strings.Replace(strings.ToLower(strings.TrimRight(domain, ".")), "\\052", "*", 1)
	} else {
		return strings.Replace(strings.ToLower(domain), "\\052", "*", 1)
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

func MapAwsResourceRecordsToDomainRecords(domain Domain, results []*route53.ResourceRecordSet) []DomainRecord {
	var domainRecords []DomainRecord = make([]DomainRecord, 0)
	for _, d := range results {
		domainRecords = append(domainRecords, DomainRecord{
			Type:   StringNilToEmpty(d.Type),
			Name:   FixAwsDomainName(StringNilToEmpty(d.Name)),
			Values: MapResourceRecordToStringArray(d.ResourceRecords),
			Domain: &domain,
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

func (dnsProvider *AwsDNSProvider) DomainRecord(domain Domain, recordType string, name string) (*DomainRecord, error) {
	dnsProvider.mutex.Lock()
	defer dnsProvider.mutex.Unlock()
	t := time.NewTicker(time.Millisecond * 500)
	<-t.C // in order to not exceed our throttle rate
	result, err := dnsProvider.client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
		MaxItems:              aws.String("1"),
		HostedZoneId:          aws.String(domain.ProviderId),
		StartRecordName:       aws.String(name),
		StartRecordType:       aws.String(recordType),
		StartRecordIdentifier: nil,
	})
	if err != nil {
		return nil, err
	}
	if len(result.ResourceRecordSets) == 0 {
		return nil, errors.New("Record not found")
	}

	records := MapAwsResourceRecordsToDomainRecords(domain, result.ResourceRecordSets)
	return &records[0], nil
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
		domainRecords = append(domainRecords, MapAwsResourceRecordsToDomainRecords(domain, result.ResourceRecordSets)...)
	}
	return domainRecords, nil
}

func (dnsProvider *AwsDNSProvider) CreateDomainRecord(domain Domain, recordType string, name string, values []string) error {
	name = strings.ToLower(name)
	t := time.NewTicker(time.Millisecond * 500)
	<-t.C // in order to not exceed our throttle rate
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
			if d.Name != name && d.Type != recordType {
				domains = append(domains, d)
			}
		}
		(*provider.domainRecordsCache)[domain.ProviderId] = domains

		// now add the new record
		(*provider.domainRecordsCache)[domain.ProviderId] = append((*provider.domainRecordsCache)[domain.ProviderId], DomainRecord{
			Type:   recordType,
			Name:   name,
			Values: values,
			Domain: &domain,
		})
		dnsProvider.mutex.Unlock()
	}
	return err
}

func (dnsProvider *AwsDNSProvider) RemoveDomainRecord(domain Domain, recordType string, name string, values []string) error {
	name = strings.ToLower(name)
	t := time.NewTicker(time.Millisecond * 500)
	<-t.C // in order to not exceed our throttle rate
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
			if d.Name != name && d.Type != recordType {
				domains = append(domains, d)
			}
		}
		(*provider.domainRecordsCache)[domain.ProviderId] = domains
		dnsProvider.mutex.Unlock()
	}
	return err
}
