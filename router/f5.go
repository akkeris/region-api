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
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	f5client "github.com/octanner/f5er/f5"
	"io/ioutil"
	"net/http"
	"os"
	spacepack "region-api/space"
	structs "region-api/structs"
	templates "region-api/templates"
	"strconv"
	"strings"
	"text/template"
	"sync"
	"log"
	"time"
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

type F5Client struct {
	Url 		string
	Token 		string
	Username    string
	Password    string
	Debug		bool
	mutex       *sync.Mutex
	client      *http.Client
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

func LeadingZero(num int) string {
	if num < 10 {
		return "0" + strconv.Itoa(num)
	} else {
		return strconv.Itoa(num)
	}
}


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

func InstallNewCert(partition string, vip string, pem_cert []byte, pem_key []byte) error {
	vserver := "/" + partition + "/" + vip

	// Sanity checks before we begin installing, ensure the certifices
	// and keys are valid in addition to their names include the target
	// server_name.
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
	main_certs_name := main_server_name + "_" + LeadingZero(x509_decoded_cert.NotAfter.Year()) + LeadingZero(int(x509_decoded_cert.NotAfter.Month())) + LeadingZero(x509_decoded_cert.NotAfter.Day())

	f5, err := GetClient()
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	//device.SetDebug(true)

	// Upload certificate and key to F5
	err = device.UploadFile(main_certs_name+".crt", pem_cert)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = device.UploadFile(main_certs_name+".key", pem_key)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Create the certificate and key objects in the F5
	err, _ = device.CreateCertificateFromLocalFile(main_certs_name, partition, main_certs_name+".crt")
	if err != nil {
		fmt.Println("Failed to create cert from local file")
		fmt.Println(main_certs_name + " partition: " + partition + " key: " + main_certs_name + ".key")
		fmt.Println(err)
		return err
	}
	err, _ = device.CreateKeyFromLocalFile(main_certs_name, partition, main_certs_name+".key")
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

		// Install the SSL/TLS Profile
		profile := LBClientSsl{
			Name:       profile_name,
			Partition:  partition,
			Ciphers:    "!SSLv2:!SSLv3:!MD5:!EXPORT:!RSA+3DES:!RSA+RC4:!ECDHE+RC4:!ECDHE+3DES:ECDHE+AES:RSA+AES", // MUST BE THIS LIST!
			SniDefault: "false",
			ServerName: server_name,
			Mode:       "enabled",
			CertKeyChain: []LBCertKeyChain{
				LBCertKeyChain{
					Name:  profile_name + "_DigiCertCA",
					Cert:  "/" + partition + "/" + main_certs_name + ".crt",
					Chain: "/" + partition + "/DigiCertCA.crt",
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
		err, _ = device.AddClientSsl(&z)
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

		err, _ = device.AddVirtualProfile(vserver, &vprofile)

		if err != nil {
			fmt.Println("== failed to add client ssl profile to virtual server:")
			fmt.Println(err)
			return err
		}
	}
	return nil
}


func ListVIPs() ([]f5client.LBVirtual, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, vips := device.ShowExpandedVirtuals()
	if err != nil {
		return nil, err
	}
	return vips.Items, nil
}

func GetVIPProfile(partition string, vipName string) ([]f5client.LBVirtualProfile, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, vip := device.ShowVirtual("/" + partition + "/" + vipName)
	if err != nil {
		return nil, err
	}
	return vip.Profiles.Items, nil
}

/*
func GetTLSProfile(partition string, name string) (f5client.SSLCertificate, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, cert := device.ShowClientSsl("/" + partition + "/" + name)
	if err != nil {
		return nil, err
	}
	return cert, nil
}
*/

func ListTLSProfiles() ([]f5client.LBClientSsl, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, ssls := device.ShowClientSsls()
	if err != nil {
		return nil, err
	}
	return ssls.Items, nil
}

/*
func GetTLSCert(partition string, name string) (f5client.SSLCertificate, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, cert := device.GetCertificate(partition, name)
	if err != nil {
		return nil, err
	}
	return cert, nil
}
*/

func ListTLSCerts() ([]f5client.SSLCertificate, error) {
	f5, err := GetClient()
	if err != nil {
		return nil, err
	}
	device := f5client.New(strings.Replace(f5.Url, "https://", "", 1), f5.Username, f5.Password, f5client.BASIC_AUTH)
	err, certs := device.GetCertificates()
	if err != nil {
		return nil, err
	}
	return certs.Items, nil
}


func DeleteF5(router structs.Routerspec, db *sql.DB) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	partition, virtual, err := getF5pv(router)
	if err != nil {
		log.Printf("Unable to delete F5 router (getF5pv failed): %s\n", err.Error())
		return msg, err
	}
	rule, err := buildRule(router, partition, virtual, db)
	if err != nil {
		log.Printf("Unable to delete F5 router (buildRule failed): %s\n", err.Error())
		return msg, err
	}
	msg, err = detachRule(rule.Name, partition, virtual)
	if err != nil {
		log.Printf("Unable to delete F5 router (detachRule failed): %s\n", err.Error())
		return msg, err
	}
	exists, err := ruleExists(router)
	if err != nil {
		log.Printf("Unable to delete F5 router (ruleExists failed): %s\n", err.Error())
		return msg, err
	}
	if exists {
		msg, err = deleteRule(rule.Name, partition, virtual)
		if err != nil {
			log.Printf("Unable to delete F5 router (deleteRule failed): %s\n", err.Error())
			return msg, err
		}
	}
	msg.Status = 200
	msg.Message = "Removed from F5"
	return msg, nil
}

func UpdateF5(router structs.Routerspec, db *sql.DB) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	partition, virtual, err := getF5pv(router)
	if err != nil {
		log.Printf("Unable to update F5 router (getF5pv failed): %s\n", err.Error())
		return msg, err
	}
	rule, err := buildRule(router, partition, virtual, db)
	if err != nil {
		log.Printf("Unable to update F5 router (buildRule failed): %s\n", err.Error())
		return msg, err	
	}
	ruleexists, err := ruleExists(router)
	if err != nil {
		log.Printf("Unable to update F5 router (ruleExists failed): %s\n", err.Error())
		return msg, err	
	}
	if ruleexists {
		err = updateRule(rule)
		if err != nil {
			log.Printf("Unable to update F5 router (updateRule failed): %s\n", err.Error())
			return msg, err	
		}
	} else {
		err = addRule(rule)
		if err != nil {
			log.Printf("Unable to update F5 router (addRule failed): %s\n", err.Error())
			return msg, err	
		}
	}
	ruleattached, err := ruleAttached(router, partition, virtual)
	if err != nil {
		log.Printf("Unable to update F5 router (ruleAttached failed): %s\n", err.Error())
		return msg, err
	}
	if !ruleattached {
		err = attachRule(rule, partition, virtual)
		if err != nil {
			log.Printf("Unable to update F5 router (attachRule failed): %s\n", err.Error())
			return msg, err
		}
	}
	return msg, nil
}

func addRule(rule structs.Rulespec) (error) {
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to addRule F5 router (GetClient failed): %s\n", err.Error())
		return err
	}
	_, err = f5.Request("post", "/mgmt/tm/ltm/rule", rule)
	if err != nil {
		log.Printf("Unable to addRule to the F5 (http call failed): %s\n", err.Error())
		return err
	}
	return nil
}

func updateRule(rule structs.Rulespec) (error) {
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to updateRule F5 router (GetClient failed): %s\n", err.Error())
		return err
	}
	_, err = f5.Request("patch", "/mgmt/tm/ltm/rule/~" + rule.Partition + "~" + rule.Name, rule)
	if err != nil {
		log.Printf("Unable to addRule to the F5 (http call failed): %s\n", err.Error())
		return err
	}
	return nil
}

func attachRule(rule structs.Rulespec, partition string, virtual string) (error) {
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to attachRule to the F5 router (GetClient failed): %s\n", err.Error())
		return err
	}
	rules, err := getRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to attachRule to the F5 (getRulesAttached failed): %s\n", err.Error())
		return err
	}
	rules = append(rules, "/" + partition + "/" + rule.Name)
	_, err = f5.Request("patch", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, structs.Virtualspec{Rules:rules})
	if err != nil {
		log.Printf("Unable to addRule to the F5 (http call failed): %s\n", err.Error())
		return err
	}
	return nil
}

func ruleExists(router structs.Routerspec) (bool, error) {
	var exists bool = false
	partition, virtual, err := getF5pv(router)
	if err != nil {
		log.Printf("Unable to run ruleExists to the F5 (getF5pv failed): %s\n", err.Error())
		return false, err
	}
	rules, err := getRules(partition, virtual)
	if err != nil {
		log.Printf("Unable to run ruleExists to the F5 (getRules failed): %s\n", err.Error())
		return false, err
	}
	for _, element := range rules {
		if element == router.Domain + "-rule" {
			exists = true
		}
	}
	return exists, nil
}

func ruleAttached(router structs.Routerspec, partition string, virtual string) (bool, error) {
	rules, err := getRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to get ruleAttached to the F5 (http call failed): %s\n", err.Error())
		return false, err
	}
	var isRuleAttached bool = false
	for _, element := range rules {
		if element == "/" + partition + "/" + router.Domain + "-rule" {
			isRuleAttached = true
		}
	}
	return isRuleAttached, nil
}

func buildRule(router structs.Routerspec, partition string, virtual string, db *sql.DB) (structs.Rulespec, error) {
	var rule = structs.Rulespec{Name:router.Domain + "-rule", Partition:partition}
	var ruleinfo = structs.RuleInfo{Domain:router.Domain}
	var switches []structs.Switch
	for _, element := range router.Paths {
		var sw structs.Switch
		if element.Space != "default" {
			sw.Pool = element.App + "-" + element.Space + "-pool"
		} else {
			sw.Pool = element.App + "-pool"
		}
		sw.Path = element.Path
		sw.ReplacePath = element.ReplacePath
		appurl, err := getAppUrl(element.App, element.Space, db)
		if err != nil {
			log.Printf("Unable to get app url while building rule: %s\n", err.Error())
			return rule, err
		}
		sw.NewHost = appurl
		switches = append(switches, sw)
	}
	ruleinfo.Switches = switches

	t := template.Must(template.New("snirule").Parse(templates.Snirule))
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

func getRulesAttached(partition string, virtual string) ([]string, error) {
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to getRulesAttached to the F5 router (GetClient failed): %s\n", err.Error())
		return []string{}, err
	}
	res, err := f5.Request("get", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, nil)
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (http call to F5 failed): %s\n", err.Error())
		return []string{}, err
	}
	bodyj, err := simplejson.NewJson(res.Body)
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (response could not marshal): %s\n", err.Error())
		return []string{}, err
	}
	var rulesa []string
	rules, err := bodyj.Get("rules").Array()
	if err != nil {
		log.Printf("Unable to getRulesAttached from the F5 (response did not have rules): %s\n", err.Error())
		return []string{}, err
	}
	for index, _ := range rules {
		value := rules[index]
		rulesa = append(rulesa, value.(string))
	}
	return rulesa, nil
}

func getRules(partition string, virtual string) ([]string, error) {
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to getRules to the F5 router (GetClient failed): %s\n", err.Error())
		return []string{}, err
	}
	res, err := f5.Request("get", "/mgmt/tm/ltm/rule?$filter=partition+eq+" + partition, nil)
	if err != nil {
		log.Printf("Unable to getRules from F5 (http call failed): %s\n", err.Error())
		return []string{}, err
	}
	type Rulesitems struct {
		Items []structs.Rulespec `json:"items"`
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

func detachRule(rule string, partition string, virtual string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	rules, err := getRulesAttached(partition, virtual)
	if err != nil {
		log.Printf("Unable to detachRule from F5 (getRulesAttached failed): %s\n", err.Error())
		return msg, err
	}
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to detachRule to the F5 router (GetClient failed): %s\n", err.Error())
		return msg, err
	}
	var newrules []string
	for _, element := range rules {
		if element != "/"+partition+"/"+rule {
			newrules = append(newrules, element)
		}
	}
	_, err = f5.Request("patch", "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual, structs.Virtualspec{Rules:newrules})
	if err != nil {
		log.Printf("Untable to detachRule, http call to F5 failed: %s\n", err.Error())
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Rule detached"
	return msg, nil
}

func deleteRule(rulename string, partition string, virtual string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	f5, err := GetClient()
	if err != nil {
		log.Printf("Unable to detachRule to the F5 router (GetClient failed): %s\n", err.Error())
		return msg, err
	}

	newrulename := strings.Replace(rulename, "/", "~", -1)
	newpartition := "~" + strings.Replace(partition, "/", "~", -1)

	_, err = f5.Request("delete", "/mgmt/tm/ltm/rule/" + newpartition + "~" + newrulename, nil)
	if err != nil {
		log.Printf("Unable to deleteRule to the F5 router (http request failed): %s\n", err.Error())
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Rule Deleted"
	return msg, nil
}

func Octhc(params martini.Params, r render.Render) {
	_, err := GetClient()
	if err != nil {
		r.Text(200, "ERROR")
	}
	r.Text(200, "OK")
}

func getF5pv(router structs.Routerspec) (p string, v string, e error) {
	var partition string
	var virtual string

	if router.Internal {
		partition = os.Getenv("F5_PARTITION_INTERNAL")
		virtual = os.Getenv("F5_VIRTUAL_INTERNAL")
	} else {
		partition = os.Getenv("F5_PARTITION")
		virtual = os.Getenv("F5_VIRTUAL")
	}
	if partition == "" {
		return "", "", fmt.Errorf("Unable to find partition in environment.")
	}
	if virtual == "" {
		return "", "", fmt.Errorf("Unable to find VIP (virtual) in environment.")
	}
	return partition, virtual, nil
}

func getAppUrl(app string, space string, db *sql.DB) (u string, e error) {
	externaldomain := os.Getenv("EXTERNAL_DOMAIN")
	if externaldomain == "" {
		log.Printf("Error getting app url for app %s and space %s because no EXTERNAL_DOMAIN was defined\n", app, space)
		return "", fmt.Errorf("No EXTERNAL_DOMAIN was defined")
	}
	internaldomain := os.Getenv("INTERNAL_DOMAIN")
	if externaldomain == "" {
		log.Printf("Error getting app url for app %s and space %s because no INTERNAL_DOMAIN was defined\n", app, space)
		return "", fmt.Errorf("No INTERNAL_DOMAIN was defined")
	}
	internal, err := spacepack.IsInternalSpace(db, space)
	if err != nil {
		log.Printf("Error getting app url for app %s and space %s becuase %s\n", app, space, err.Error())
		return "", err
	}
	if internal {
		return app + "-" + space + "." + internaldomain, nil
	} else if space == "default" {
		return app + "." + externaldomain, nil
	} else {
		return app + "-" + space + "." + externaldomain, nil
	}
}
