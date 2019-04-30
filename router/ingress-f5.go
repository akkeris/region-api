package router

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	vault "github.com/akkeris/vault-client"
	"github.com/bitly/go-simplejson"
	f5client "github.com/octanner/f5er/f5"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	"strconv"
	"strings"
	"sync"
	"log"
	"time"
	"text/template"
)


type F5Request struct {
	Status     string
	StatusCode int
	Body       []byte
}

type f5creds struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}

type f5Virtualspec struct {
	Rules []string `json:"rules"`
}

type f5Rulespec struct {
	Name         string `json:"name"`
	Partition    string `json:"partition"`
	ApiAnonymous string `json:"apiAnonymous"`
}

type f5Rule struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Partition    string `json:"partition"`
	FullPath     string `json:"fullPath"`
	Generation   int    `json:"generation"`
	SelfLink     string `json:"selfLink"`
	APIAnonymous string `json:"apiAnonymous"`
}

type F5Client struct {
	Url 		string
	Token 		string
	Username    string
	Password    string
	Debug		bool
	mutex       *sync.Mutex
	client      *http.Client
	device		*f5client.Device
}

type LBClientSsl struct {
	Name         string           `json:"name"`
	Partition    string           `json:"partition"`
	Ciphers      string           `json:"ciphers"`
	DefaultsFrom string           `json:"defaultsFrom"`
	Mode         string           `json:"mode"`
	SniDefault   string           `json:"sniDefault"`
	ServerName   string           `json:"serverName"`
	CertKeyChain []LBCertKeyChain `json:"certKeyChain"`
}

type LBCertKeyChain struct {
	Name  string `json:"name"`
	Cert  string `json:"cert"`
	Chain string `json:"chain"`
	Key   string `json:"key"`
}

type Profile struct {
	Name string
	ServerName string
	CertName string
}

func LeadingZero(num int) string {
	if num < 10 {
		return "0" + strconv.Itoa(num)
	} else {
		return strconv.Itoa(num)
	}
}

const SniruleUnipool = `
{{ $domain := .Domain }}
when HTTP_REQUEST {
	switch [string tolower [HTTP::host]] {
		"{{.Domain}}" {
			set xri [HTTP::header "x-request-id"]
			set hrt [clock format [clock seconds] -gmt 0 -format "%m-%d-%YT%H:%M:%S%z"]
			set http_request_start_time [clock clicks -milliseconds]
			if {$xri eq ""} {
			    binary scan [md5 "[IP::client_addr][TCP::client_port][IP::local_addr][TCP::local_port][string range [AES::key 256] 8 end]"] H* xri junk
			}
			set LogString "timestamp=$hrt fwd=[IP::client_addr] method=[HTTP::method] path=[HTTP::uri] request_id=$xri site_domain={{$domain}} "
		    HTTP::header insert x-request-id $xri
			HTTP::header insert X-Orig-Proto [HTTP::header "X-Forwarded-Proto"]
			HTTP::header insert X-Orig-Host [HTTP::header "Host"]
			HTTP::header insert X-Orig-Port [TCP::local_port]
			HTTP::header insert X-Forwarded-Path [HTTP::path]
			switch -glob [string tolower [HTTP::uri]] {
{{ range $value := .Switches }}
"{{$value.Path}}/*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::path]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::path [string map -nocase {"{{$value.Path}}/" "{{$value.ReplacePath}}/"} [HTTP::path]]
if {[regsub -all "//" [HTTP::path] "/" newpath] > 0} { HTTP::path $newpath }
#oldpool:pool {{$value.Pool}}
set new_port "{{$value.Nodeport}}"
pool {{$value.Unipool}}

}
"{{$value.Path}}*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::uri]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::uri [string map -nocase {"{{$value.Path}}" "{{$value.ReplacePath}}"} [HTTP::uri]]
#oldpool:pool {{$value.Pool}}
set new_port "{{$value.Nodeport}}"
pool {{$value.Unipool}}
}
{{end}}
			}
		}
	}
}
`

const Snirule = `
{{ $domain := .Domain }}
when HTTP_REQUEST {
        switch [string tolower [HTTP::host]] {
                "{{.Domain}}" {
                        set xri [HTTP::header "x-request-id"]
                        set hrt [clock format [clock seconds] -gmt 0 -format "%m-%d-%YT%H:%M:%S%z"]
                        set http_request_start_time [clock clicks -milliseconds]
                        if {$xri eq ""} {
                            binary scan [md5 "[IP::client_addr][TCP::client_port][IP::local_addr][TCP::local_port][string range [AES::key 256] 8 end]"] H* xri junk
                        }
                        set LogString "timestamp=$hrt fwd=[IP::client_addr] method=[HTTP::method] path=[HTTP::uri] request_id=$xri site_domain={{$domain}} "
                    HTTP::header insert x-request-id $xri
                        HTTP::header insert X-Orig-Proto [HTTP::header "X-Forwarded-Proto"]
                        HTTP::header insert X-Orig-Host [HTTP::header "Host"]
                        HTTP::header insert X-Orig-Port [TCP::local_port]
                        HTTP::header insert X-Forwarded-Path [HTTP::path]
                        switch -glob [string tolower [HTTP::uri]] {
{{ range $value := .Switches }}
"{{$value.Path}}/*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::path]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::path [string map -nocase {"{{$value.Path}}/" "{{$value.ReplacePath}}/"} [HTTP::path]]
if {[regsub -all "//" [HTTP::path] "/" newpath] > 0} { HTTP::path $newpath }
pool {{$value.Pool}}

}
"{{$value.Path}}*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::uri]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::uri [string map -nocase {"{{$value.Path}}" "{{$value.ReplacePath}}"} [HTTP::uri]]
pool {{$value.Pool}}
}
{{end}}
                        }
                }
        }
}
`

func (f5 *F5Client) Request(method string, path string, payload interface{}) (r *F5Request, err error) {
	var req *http.Request
	var body string
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			if f5.Debug {
				log.Printf("** f5: invalid payload/body supplied: %s %s - %s\n", method, f5.Url + path, err)
			}
			return nil, err
		}
		body = bytes.Replace(body, []byte("\\u003e"), []byte(">"), -1)
		req, err = http.NewRequest(strings.ToUpper(method), f5.Url + path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
	} else {
		req, err = http.NewRequest(strings.ToUpper(method), f5.Url + path, nil)
		if err != nil {
			return nil, err
		}
	}
	if f5.Token == "" {
		return nil, fmt.Errorf("The f5 client is not yet ready or has an empty token.")
	}
	req.Header.Add("X-F5-Auth-Token", f5.Token)
	req.Header.Add("Content-Type", "application/json")
	if f5.Debug {
		log.Printf("-> f5: %s %s with headers [%s] with payload [%s]\n", method, f5.Url + path, req.Header, body)
	}
	f5.mutex.Lock()
	resp, err := f5.client.Do(req)
	f5.mutex.Unlock()
	if err != nil {
		if f5.Debug {
			log.Printf("<- f5 ERROR: %s %s - %s\n", method, f5.Url + path, err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if f5.Debug {
			log.Printf("<- f5 ERROR: %s %s - %s\n", method, f5.Url + path, err)
		}
		return nil, err
	}
	if resp.StatusCode == 401 {
		// Wait a second (or two), then retry the request, but only if were able to successfully
		// get a new token. Do not allow anyone to use the connection in the meantime.
		f5.mutex.Lock()
		time.Sleep(2 * time.Second)
		f5.mutex.Unlock()
		if err := f5.getToken(); err != nil {
			fmt.Printf("** FATAL ERROR ** Unable to obtain token in NewF5Client: %s\n", err.Error())
			return nil, err
		}
		return f5.Request(method, path, payload)
	}
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		err = fmt.Errorf("%s (%d)", resp.Status, resp.StatusCode)
		if f5.Debug {
			log.Printf("<- f5 ERROR: %s %s - %s\n", method, f5.Url + path, err)
		}
		return nil, err
	}
	if f5.Debug {
		log.Printf("<- f5: %s %s - %s\n", method, f5.Url + path, resp.Status)
	}
	return &F5Request{Body: respBody, Status: resp.Status, StatusCode: resp.StatusCode}, nil
}

func (f5 *F5Client) getToken() error {
	str, err := json.Marshal(f5creds{Username:f5.Username, Password:f5.Password, LoginProviderName:"tmos"})
	if err != nil {
		log.Printf("Error creating new F5 client, unable to marshal data: %s\n", err.Error())
		return err
	}
	req, err := http.NewRequest("POST", f5.Url + "/mgmt/shared/authn/login", bytes.NewBuffer([]byte(string(str))))
	req.Header.Add("Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte(f5.Username + ":" + f5.Password)))
	req.Header.Add("Content-Type", "application/json")
	f5.mutex.Lock()
	resp, err := f5.client.Do(req)
	defer f5.mutex.Unlock()
	if err != nil {
		log.Printf("Error creating new F5 client, unable to login to F5: %s\n", err.Error())
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err = fmt.Errorf("Login returned %s", resp.Status)
		log.Printf("Error creating new F5 client, unable to login to F5: %s\n", err.Error())
		return err
	}
	defer resp.Body.Close()
	bodyj, err := simplejson.NewFromReader(resp.Body)
	if err != nil {
		log.Printf("Error creating new F5 client, unable to unmarshal response: %s\n", err.Error())
		return err
	}
	token, err := bodyj.Get("token").Get("token").String()
	if err != nil {
		log.Printf("Error creating new F5 client, unable to find token field in login response: %s\n", err.Error())
		return err
	}
	f5.Token = token
	return nil
}

func (f5 *F5Client) GetRule(partition string, rulename string) (*f5Rulespec, error) {
	resp, err := f5.Request("get", "/mgmt/tm/ltm/rule/~" + partition + "~" + rulename, nil)
	if err != nil {
		return nil, err
	}
	var rule f5Rulespec
	if err = json.Unmarshal(resp.Body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

func (f5 *F5Client) GetRules(partition string) ([]string, error) {
	res, err := f5.Request("get", "/mgmt/tm/ltm/rule?$filter=partition+eq+" + partition, nil)
	if err != nil {
		log.Printf("Unable to getRules from F5 (http call failed): %s\n", err.Error())
		return []string{}, err
	}
	type Rulesitems struct {
		Items []f5Rulespec `json:"items"`
	}
	var ruleso Rulesitems
	err = json.Unmarshal([]byte(res.Body), &ruleso)
	if err != nil {
		log.Printf("Unable to getRules from F5 (marshal failed): %s\n", err.Error())
		return []string{}, err
	}
	var elements []string
	for _, element := range ruleso.Items {
		elements = append(elements, element.Name)
	}
	return elements, nil
}

func (f5 *F5Client) AddRule(rule f5Rulespec) (error) {
	if f5.Debug {
		log.Printf("f5 -> Adding rule: %#+v\n", rule)
	}
	if _, err := f5.Request("post", "/mgmt/tm/ltm/rule", rule); err != nil {
		log.Printf("Unable to addRule to the F5 (http call failed): %s\n", err.Error())
		return err
	}
	return nil
}

func (f5 *F5Client) GetRulesAttached(partition string, virtual string) ([]string, error) {
	res, err := f5.Request("get", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, nil)
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (http call to F5 failed): %s\n", err.Error())
		return nil, err
	}
	bodyj, err := simplejson.NewJson(res.Body)
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (response could not marshal): %s\n", err.Error())
		return nil, err
	}
	var rulesa []string
	rules, err := bodyj.Get("rules").Array()
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (response did not have rules): %s\n", err.Error())
		return nil, err
	}
	for index, _ := range rules {
		value := rules[index]
		rulesa = append(rulesa, value.(string))
	}
	return rulesa, nil
}

func (f5 *F5Client) UpdateRule(partition string, rulename string, rule f5Rulespec) (error) {
	if f5.Debug {
		log.Printf("f5 -> Updating rule: %#+v\n", rule)
		log.Printf("=== iRule [" + "/mgmt/tm/ltm/rule/~" + partition + "~" + rulename + "] ===\n")
		str, err := json.Marshal(rule)
		if err != nil {
			log.Printf("!!! Error Rule Failed to Serialize !!! %s\n", err.Error())
		} else {
			log.Printf("%s\n", str)
		}
		log.Printf("=== iRule ===\n")
	}
	_, err := f5.Request("patch", "/mgmt/tm/ltm/rule/~" + partition + "~" + rulename, rule)
	if err != nil {
		return err
	}
	return nil
}

func (f5 *F5Client) DetachRule(rule string, partition string, virtual string) (e error) {
	rules, err := f5.GetRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to detachRule from F5 (getRulesAttached failed): %s\n", err.Error())
		return err
	}
	var newrules []string
	for _, element := range rules {
		if element != "/"+partition+"/"+rule {
			newrules = append(newrules, element)
		}
	}
	_, err = f5.Request("patch", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, f5Virtualspec{Rules:newrules})
	if err != nil {
		log.Printf("Untable to detachRule, http call to F5 failed: %s\n", err.Error())
		return err
	}
	return nil
}

func (f5 *F5Client) DeleteRule(rulename string, partition string, virtual string) (e error) {
	newrulename := strings.Replace(rulename, "/", "~", -1)
	newpartition := "~" + strings.Replace(partition, "/", "~", -1)
	if _, err := f5.Request("delete", "/mgmt/tm/ltm/rule/" + newpartition + "~" + newrulename, nil); err != nil {
		log.Printf("Unable to deleteRule to the F5 router (http request failed): %s\n", err.Error())
		return err
	}
	return nil
}

func (f5 *F5Client) BuildRule(db *sql.DB, router structs.Routerspec, partition string, virtual string) (f5Rulespec, error) {
	rule := f5Rulespec{Name:router.Domain + "-rule", Partition:partition}
	ruleinfo := structs.RuleInfo{Domain:router.Domain}
	var switches []structs.Switch
	for _, element := range router.Paths {
		var sw structs.Switch
		if element.Space != "default" {
			sw.Pool = element.App + "-" + element.Space + "-pool"
		} else {
			sw.Pool = element.App + "-pool"
		}
		nodeport, err := GetNodePort(db, element.Space, element.App)
		if err != nil {
			log.Printf("Unable to get app url while building rule (getting nodeport): %s\n", err.Error())
			return rule, err
		}
		sw.Nodeport = nodeport
		sw.Path = element.Path
		sw.ReplacePath = element.ReplacePath
		sw.Unipool = "/"+partition+"/unipool"
		appurl, err := GetAppUrl(db, element.App, element.Space)
		if err != nil {
			log.Printf("Unable to get app url while building rule: %s\n", err.Error())
			return rule, err
		}
		sw.NewHost = appurl
		switches = append(switches, sw)
	}
	ruleinfo.Switches = switches

	var t *template.Template
	if os.Getenv("UNIPOOL") != "" || os.Getenv("F5_UNIPOOL") != "" {
		t = template.Must(template.New("snirule").Parse(SniruleUnipool))
	}
	if os.Getenv("UNIPOOL") == "" && os.Getenv("F5_UNIPOOL") == "" {
		t = template.Must(template.New("snirule").Parse(Snirule))
	}
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	err := t.Execute(wr, ruleinfo)
	if err != nil {
		log.Printf("Unable to build rule from template: %s\n", err.Error())
		return rule, err
	}
	wr.Flush()
	rule.ApiAnonymous = string(b.Bytes())
	return rule, nil
}

func (f5 *F5Client) RuleExists(partition string, virtual string, ruleName string) (bool, error) {
	var exists bool = false
	rules, err := f5.GetRules(partition)
	if err != nil {
		log.Printf("Unable to run ruleExists to the F5 (getRules failed): %s\n", err.Error())
		return false, err
	}
	for _, element := range rules {
		if element == ruleName {
			exists = true
		}
	}
	return exists, nil
}

func (f5 *F5Client) IsRuleAttached(router structs.Routerspec, partition string, virtual string) (bool, error) {
	rules, err := f5.GetRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to get ruleAttached to the F5 (http call failed): %s\n", err.Error())
		return false, err
	}
	for _, element := range rules {
		if element == "/" + partition + "/" + router.Domain + "-rule" {
			return true, nil
		}
	}
	return false, nil
}

func (f5 *F5Client) AttachRule(rule f5Rulespec, partition string, virtual string) (error) {
	rules, err := f5.GetRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to attachRule to the F5 (getRulesAttached failed): %s\n", err.Error())
		return err
	}
	rules = append(rules, "/" + partition + "/" + rule.Name)
	_, err = f5.Request("patch", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, f5Virtualspec{Rules:rules})
	if err != nil {
		log.Printf("Unable to addRule to the F5 (http call failed): %s\n", err.Error())
		return err
	}
	return nil
}

func (f5 *F5Client) InstallCertificate(partition string, vip string, server_name string, pem_certs []byte, pem_key []byte) error {
	vserver := "/" + partition + "/" + vip

	if f5.Debug {
		log.Printf("-> f5: Installing certificate on partition %s for vip %s with server name %s\n", partition, vip, server_name)
	}
	x509_decoded_cert, pem_cert, pem_chain, err := DecodeCertificateBundle(server_name, pem_certs);
	if err != nil {
		fmt.Println(err)
		return err
	}
	block, _ := pem.Decode(pem_key)
	if block == nil {
		fmt.Printf("failed to parse PEM block containing the private key\n")
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
	main_certs_name := main_server_name + "_" + LeadingZero(x509_decoded_cert.NotAfter.Year()) + LeadingZero(int(x509_decoded_cert.NotAfter.Month())) + LeadingZero(x509_decoded_cert.NotAfter.Day())

	// Upload certificate and key to F5
	if len(pem_chain) != 0 {
		if err = f5.device.UploadFile(main_certs_name + "_chain.crt", pem_chain); err != nil {
			fmt.Println(err)
			return err
		}
	}
	if err = f5.device.UploadFile(main_certs_name + ".crt", pem_cert); err != nil {
		fmt.Println(err)
		return err
	}
	if err = f5.device.UploadFile(main_certs_name + ".key", pem_key); err != nil {
		fmt.Println(err)
		return err
	}

	// Create the certificate and key objects in the F5
	if len(pem_chain) != 0 {
		err, _ = f5.device.CreateCertificateFromLocalFile(main_certs_name + "_chain", partition, main_certs_name + "_chain.crt")
		if err != nil {
			fmt.Println("Failed to create cert chain from local file")
			fmt.Println(main_certs_name + "_chain partition: " + partition)
			fmt.Println(err)
			return err
		}
	}
	err, _ = f5.device.CreateCertificateFromLocalFile(main_certs_name, partition, main_certs_name + ".crt")
	if err != nil {
		fmt.Println("Failed to create cert from local file")
		fmt.Println(main_certs_name + " partition: " + partition + " key: " + main_certs_name + ".key")
		fmt.Println(err)
		return err
	}
	err, _ = f5.device.CreateKeyFromLocalFile(main_certs_name, partition, main_certs_name + ".key")
	if err != nil {
		fmt.Println("Failed to create key from local file")
		fmt.Println(main_certs_name + " partition: " + partition + " key: " + main_certs_name + ".key")
		fmt.Println(err)
		return err
	}

	for _, server_name := range x509_decoded_cert.DNSNames {
		err = x509_decoded_cert.VerifyHostname(server_name)
		if err != nil {
			fmt.Println("Invalid, the server name does not match whats in the cert")
			fmt.Println(err)
			return err
		}
		profile_name := strings.Replace(server_name, "*.", "star.", -1) + "_" + LeadingZero(x509_decoded_cert.NotAfter.Year()) + LeadingZero(int(x509_decoded_cert.NotAfter.Month())) + LeadingZero(x509_decoded_cert.NotAfter.Day())

		f5_cipher_list := "!SSLv2:!SSLv3:!MD5:!EXPORT:!RSA+3DES:!RSA+RC4:!ECDHE+RC4:!ECDHE+3DES:ECDHE+AES:RSA+AES"
		if os.Getenv("F5_CIPHER_LIST") != "" {
			f5_cipher_list = os.Getenv("F5_CIPHER_LIST")
		}

		// Install the SSL/TLS Profile
		profile := LBClientSsl{
			Name:       profile_name,
			Partition:  partition,
			Ciphers:    f5_cipher_list, // MUST BE THIS LIST!
			SniDefault: "false",
			ServerName: server_name,
			Mode:       "enabled",
			CertKeyChain: []LBCertKeyChain{
				LBCertKeyChain{
					Name:  profile_name,
					Cert:  "/" + partition + "/" + main_certs_name + ".crt",
					Chain: "/" + partition + "/" + main_certs_name + "_chain.crt",
					Key:   "/" + partition + "/" + main_certs_name + ".key",
				},
			},
		}
		b, err := json.Marshal(profile)
		if err != nil {
			fmt.Println("== failed to marshal SSL profile:")
			fmt.Println(profile)
			fmt.Println(err)
			return err
		}
		z := json.RawMessage(b)
		err, _ = f5.device.AddClientSsl(&z)
		if err != nil {
			fmt.Println("== failed to add client ssl profile:")
			fmt.Println(z)
			fmt.Println(err)
			return err
		}
		// Attach the client ssl profile to the virtual server
		vprofile := f5client.LBVirtualProfile{
			Name:      "/" + partition + "/" + profile_name,
			Partition: partition,
			FullPath:  "/" + partition + "/" + profile_name,
			Context:   "clientside",
		}

		err, _ = f5.device.AddVirtualProfile(vserver, &vprofile)

		if err != nil {
			fmt.Println("== failed to add client ssl profile to virtual server:")
			fmt.Println(err)
			return err
		}
	}
	return nil
}

func (f5 *F5Client) GetTLSCerts() ([]f5client.SSLCertificate, error) {
	err, certs := f5.device.GetCertificates()
	if err != nil {
		return nil, err
	}
	return certs.Items, nil
}

func (f5 *F5Client) GetVIPProfiles(partition string, vipName string) ([]f5client.LBVirtualProfile, error) {
	err, vip := f5.device.ShowVirtual("/" + partition + "/" + vipName)
	if err != nil {
		return nil, err
	}
	return vip.Profiles.Items, nil
}

func (f5 *F5Client) IsCertificateAttachedToVip(cert_profile_name string, partition string, vip string) (bool, error) {
	ssl_profile, err := f5.GetTLSProfile(cert_profile_name, partition)
	if err != nil {
		fmt.Println("Error occured on GetTLSProfile with " + cert_profile_name)
		return false, err
	}
	vipProfiles, err := f5.GetVIPProfiles(partition, vip)
	if err != nil {
		fmt.Println("Error occured on GetVIPProfiles with " + partition + " " + vip)
		return false, err
	}
	for _, attachedProfile := range vipProfiles {
		if attachedProfile.Partition == partition && attachedProfile.Name == ssl_profile.Name {
			return true, nil
		}
	}
	return false, nil
}

func (f5 *F5Client) GetTLSProfile(server_name string, partition string) (*f5client.LBClientSsl, error) {
	err, ssl := f5.device.ShowClientSsl("/" + partition + "/" + server_name)
	if err != nil {
		return nil, err
	}
	return ssl, nil
}

func (f5 *F5Client) GetTLSCert(cert_name string, partition string) (*f5client.SSLCertificate, error) {
	err, cert := f5.device.GetCertificate(partition, cert_name)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func NewF5Client(url string, username string, password string) (*F5Client, error) {
	if url == "" {
		return nil, fmt.Errorf("F5 url was undefined or blank.")
	}
	if username == "" {
		return nil, fmt.Errorf("F5 username was undefined or blank.")
	}
	if password == "" {
		return nil, fmt.Errorf("F5 password was undefined or blank.")
	}
	f5 := &F5Client{}
	f5.mutex = &sync.Mutex{}
	f5.Debug = os.Getenv("DEBUG_F5") == "true"
	f5.Username = username
	f5.Password = password
	f5.client = &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	f5.Url = url
	f5.device = f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	if f5.Debug {
		f5.device.SetDebug(true)
	}
	if err := f5.getToken(); err != nil {
		fmt.Printf("** FATAL ERROR ** Unable to obtain token in NewF5Client: %s\n", err.Error())
		return nil, err
	}
	return f5, nil
}

var f5Client *F5Client = nil;

func GetClient() (*F5Client, error) {
	if f5Client != nil {
		return f5Client, nil
	}
	f5secret := os.Getenv("F5_SECRET")
	if f5secret == "" {
		err := fmt.Errorf("no F5_SECRET was found in environment.")
		log.Printf("Error initializing F5 client: %s\n", err.Error())
		return nil, err
	}
	f5url := vault.GetField(f5secret, "url")
	if f5url == "" {
		err := fmt.Errorf("F5 Url was not in vault or blank.")
		log.Printf("Error initializing F5 client: %s\n", err.Error())
		return nil, err
	}
	vaulttoken := os.Getenv("VAULT_TOKEN")
	if vaulttoken == "" {
		err := fmt.Errorf("VAULT_TOKEN was blank or does not exist")
		log.Printf("Error initializing F5 client: %s\n", err.Error())
		return nil, err
	}
	vaultaddr := os.Getenv("VAULT_ADDR")
	if vaulttoken == "" {
		err := fmt.Errorf("VAULT_ADDR was blank or does not exist")
		log.Printf("Error initializing F5 client: %s\n", err.Error())
		return nil, err
	}
	vaultaddruri := vaultaddr + "/v1/" + f5secret
	vreq, err := http.NewRequest("GET", vaultaddruri, nil)
	vreq.Header.Add("X-Vault-Token", vaulttoken)
	vclient := &http.Client{}
	vresp, err := vclient.Do(vreq)
	if err != nil {
		log.Printf("Error initializing F5 client, unable to get vault secret: %s\n", err.Error())
		return nil, err
	}
	defer vresp.Body.Close()
	bodyj, err := simplejson.NewFromReader(vresp.Body)
	if err != nil {
		log.Printf("Error initializing F5 client, unable to read vault secret: %s\n", err.Error())
		return nil, err
	}
	f5username, err := bodyj.Get("data").Get("username").String()
	if err != nil {
		log.Printf("Error initializing F5 client, unable to read f5 username: %s\n", err.Error())
		return nil, err
	}
	f5password, err := bodyj.Get("data").Get("password").String()
	if err != nil {
		log.Printf("Error initializing F5 client, unable to read f5 password: %s\n", err.Error())
		return nil, err
	}
	f5c, err := NewF5Client(f5url, f5username, f5password)
	if err != nil {
		log.Printf("Error initializing F5 client: %s\n", err.Error())
		return nil, err
	}
	f5Client = f5c
	return f5Client, nil
}

type F5Ingress struct {
	client *F5Client
	config *IngressConfig
	db *sql.DB
}

func GetF5Ingress(db *sql.DB, config *IngressConfig) (*F5Ingress, error) {
	client, err := GetClient()
	if err != nil {
		return nil, err
	}
	return &F5Ingress{
		client:client,
		config:config,
		db:db,
	}, nil
}

func (ingress *F5Ingress) CreateOrUpdateRouter(router structs.Routerspec) (error) {
	config, err := GetSitesIngressPublicExternal()
	if err != nil {
		return err
	}
	if router.Internal {
		config, err = GetSitesIngressPrivateInternal()
		if err != nil {
			return err
		}
	}
	rule, err := ingress.client.BuildRule(ingress.db, router, config.Environment, config.Name)
	if err != nil {
		log.Printf("Unable to update F5 router (buildRule failed): %s\n", err.Error())
		return err	
	}
	exists, err := ingress.client.RuleExists(config.Environment, config.Name, router.Domain + "-rule")
	if err != nil {
		log.Printf("Unable to update F5 router (ruleExists failed): %s\n", err.Error())
		return err	
	}
	if exists {
		if err = ingress.client.UpdateRule(config.Environment, router.Domain + "-rule", rule); err != nil {
			log.Printf("Unable to update F5 router (updateRule failed): %s\n", err.Error())
			return err	
		}
	} else {
		if err = ingress.client.AddRule(rule); err != nil {
			log.Printf("Unable to update F5 router (addRule failed): %s\n", err.Error())
			return err	
		}
	}
	attached, err := ingress.client.IsRuleAttached(router, config.Environment, config.Name)
	if err != nil {
		log.Printf("Unable to update F5 router (ruleAttached failed): %s\n", err.Error())
		return err
	}
	if !attached {
		if err = ingress.client.AttachRule(rule, config.Environment, config.Name); err != nil {
			log.Printf("Unable to update F5 router (attachRule failed): %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (ingress *F5Ingress) DeleteRouter(router structs.Routerspec) (error) {
	config, err := GetSitesIngressPublicExternal()
	if err != nil {
		return err
	}
	if router.Internal {
		config, err = GetSitesIngressPrivateInternal()
		if err != nil {
			return err
		}
	}
	rule, err := ingress.client.BuildRule(ingress.db, router, config.Environment, config.Name)
	ruleName := router.Domain + "-rule"
	if err != nil {
		log.Printf("Unable to delete F5 router (buildRule failed): %s\n", err.Error())
		return err
	}
	err = ingress.client.DetachRule(rule.Name, config.Environment, config.Name)
	if err != nil {
		log.Printf("Unable to delete F5 router (detachRule failed): %s\n", err.Error())
		return err
	}
	exists, err := ingress.client.RuleExists(config.Environment, config.Name, ruleName)
	if err != nil {
		log.Printf("Unable to delete F5 router (ruleExists failed): %s\n", err.Error())
		return err
	}
	if exists {
		err = ingress.client.DeleteRule(ruleName, config.Environment, config.Name)
		if err != nil {
			log.Printf("Unable to delete F5 router (deleteRule failed): %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (ingress *F5Ingress) SetMaintenancePage(app string, space string, value bool) (error) {
	status, err := ingress.GetMaintenancePageStatus(app, space)
	if err != nil {
		return err
	}
	if status == value {
		return nil
	}
	ruleName := app + "-" + space + "-rule"
	if space == "default" {
		ruleName = app + "-rule"
	}
	if value {
		rule, err := ingress.client.GetRule(ingress.config.Environment, ruleName)
		if err != nil {
			return err
		}
		rulearray := strings.Split(rule.ApiAnonymous, "\n")
		var poolindex int
		for index, element := range rulearray {
			if strings.HasPrefix(element, "pool /") {
				poolindex = index
			}
		}
		rulearray = append(rulearray[:poolindex], append([]string{`HTTP::respond 200 content [ifile get maintpage] "Content-Type" "text/html" "Connection" "Close" return`}, rulearray[poolindex:]...)...)
		rule.ApiAnonymous = strings.Join(rulearray, "\n")
		if err = ingress.client.UpdateRule(ingress.config.Environment, ruleName, *rule); err != nil {
			return err
		}
	} else {
		rule, err := ingress.client.GetRule(ingress.config.Environment, ruleName)
		if err != nil {
			return err
		}
		rulearray := strings.Split(rule.ApiAnonymous, "\n")
		var maintindex int
		for index, element := range rulearray {
			if strings.HasPrefix(element, "HTTP::respond") {
				maintindex = index
			}
		}
		rulearray = append(rulearray[:maintindex], rulearray[maintindex+1:]...)
		rule.ApiAnonymous = strings.Join(rulearray, "\n")
		if err = ingress.client.UpdateRule(ingress.config.Environment, ruleName, *rule); err != nil {
			return err
		}
	}
	return nil
}

func (ingress *F5Ingress) GetMaintenancePageStatus(app string, space string) (bool, error) {
	ruleName := app + "-" + space + "-rule"
	if space == "default" {
		ruleName = app + "-rule"
	}
	rule, err := ingress.client.GetRule(ingress.config.Environment, ruleName)
	if err != nil {
		return false, err
	}
	if strings.Contains(rule.ApiAnonymous, "ifile get maintpage") {
		return true, nil
	}
	return false, nil
}

func (ingress *F5Ingress) GetCertificateInfo(server_name string) (string, error) {
	client, err := ingress.client.GetTLSCert(server_name, ingress.config.Environment)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(client)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (ingress *F5Ingress) InstallCertificate(server_name string, pem_cert []byte, pem_key []byte) (error) {
	return ingress.client.InstallCertificate(ingress.config.Environment, ingress.config.Name, server_name, pem_cert, pem_key)
}

// on or attached to a vip
// ip address its pointing to

func (ingress *F5Ingress) GetInstalledCertificates(site string) ([]Certificate, error) {
	ssl_certs, err := ingress.client.GetTLSCerts()
	if err != nil {
		return nil, err
	}

	var certs []Certificate = make([]Certificate, 0)
	for _, cert := range ssl_certs {
		var matchedSiteDomain = false
		var onCorrectPartition = false
		var isAttachedToVip = false
		var notExpired = false
		sans := strings.Split(strings.Replace(cert.SubjectAlternativeName, "DNS:", "", -1), ",")
		var certType string = "normal"
		if len(sans) > 1 {
			certType = "sans"
		} 
		destsans := make([]string, 0)
		for _, san := range sans {
			san = strings.Replace(san, " ", "", -1)
			destsans = append(destsans, san)
			if strings.Contains(san, "*") {
				certType = "wildcard"
			}
			if !matchedSiteDomain {
				matchedSiteDomain = WildCardMatch(san, site)
			}
		}
		notExpired = int64(cert.Expiration) > time.Now().Unix()
		onCorrectPartition = cert.Partition == ingress.config.Environment
		// Dont bother unless we match
		if matchedSiteDomain && onCorrectPartition {
			isAttachedToVip, err = ingress.client.IsCertificateAttachedToVip(strings.Replace(cert.Name, ".crt", "", -1), ingress.config.Environment, ingress.config.Name)
			if err != nil {
				isAttachedToVip = false
				fmt.Printf("An error occured querying if the certifiate was attached: %s %s\n", cert.Name, err.Error())
			}
		}
		if matchedSiteDomain && onCorrectPartition && isAttachedToVip {
			certs = append(certs, Certificate{
				Type: certType,
				Name: cert.Name,
				Expires: int64(cert.Expiration),
				Alternatives: destsans,
				Expired: !notExpired,
				Address: ingress.config.Address,
			})
		}
	}
	return certs, nil
}

func (ingress *F5Ingress) Config() *IngressConfig {
	return ingress.config
}

func (ingress *F5Ingress) Name() string {
	return "f5"
}

