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
)

type f5creds struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}

var creds f5creds
var Client *http.Client
var F5url string
var F5token string
var f5auth string

func getF5Credentials() (f5creds, error) {
	var c f5creds
	f5secret := os.Getenv("F5_SECRET")
	F5url = vault.GetField(f5secret, "url")
	vaulttoken := os.Getenv("VAULT_TOKEN")
	vaultaddr := os.Getenv("VAULT_ADDR")
	vaultaddruri := vaultaddr + "/v1/" + f5secret
	vreq, err := http.NewRequest("GET", vaultaddruri, nil)
	vreq.Header.Add("X-Vault-Token", vaulttoken)
	vclient := &http.Client{}
	vresp, err := vclient.Do(vreq)
	if err != nil {
		fmt.Println("Unable to get vault secret:")
		fmt.Println(err)
		return c, err
	}
	defer vresp.Body.Close()
	bodyj, err := simplejson.NewFromReader(vresp.Body)
	if err != nil {
		fmt.Println("Unable to parse vault secret:")
		fmt.Println(err)
		return c, err
	}
	f5username, _ := bodyj.Get("data").Get("username").String()
	f5password, _ := bodyj.Get("data").Get("password").String()
	c.Username = f5username
	c.Password = f5password
	c.LoginProviderName = "tmos"
	return c, nil
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

	startClient()
	device := f5client.New(strings.Replace(F5url, "https://", "", 1), creds.Username, creds.Password, f5client.BASIC_AUTH)
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

func DeleteF5(router structs.Routerspec, db *sql.DB) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	startClient()
	partition, virtual, err := getF5pv(router)
	fmt.Println("Partition: " + partition)
	fmt.Println("Virtual:" + virtual)

	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	rule := buildRule(router, partition, virtual, db)
	msg, err = detachRule(rule.Name, partition, virtual)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	msg, err = deleteRule(rule.Name, partition, virtual)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	msg.Status = 200
	msg.Message = "Removed from F5"
	return msg, nil
}

func UpdateF5(router structs.Routerspec, db *sql.DB) (m structs.Messagespec, e error) {
	fmt.Println(router.Domain)
	fmt.Println(router.Paths)
	var msg structs.Messagespec
	startClient()
	partition, virtual, err := getF5pv(router)
	fmt.Println("THIS IS THE PARITION: " + partition)
	fmt.Println("THIS IS THE VIRTUAL: " + virtual)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	rule := buildRule(router, partition, virtual, db)
	ruleexists := ruleExists(router)
	if ruleexists {
		updateRule(rule)
	}
	if !ruleexists {
		addRule(rule)
	}
	ruleattached := ruleAttached(router, partition, virtual)
	if !ruleattached {
		attachRule(rule, partition, virtual)
	}
	return msg, nil
}

func startClient() {
	var err error
	creds, err = getF5Credentials()
	if err != nil {
		// error already reported.
		return
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	Client = &http.Client{Transport: tr}
	data := []byte(creds.Username + ":" + creds.Password)
	dstr := base64.StdEncoding.EncodeToString(data)
	f5auth = "Basic " + dstr
	str, err := json.Marshal(creds)
	if err != nil {
		fmt.Println("Error preparing request for f5:")
		fmt.Println(err)
		return
	}
	jsonStr := []byte(string(str))
	urlStr := F5url + "/mgmt/shared/authn/login"
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("Authorization", f5auth)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println("Unable to make f5 login request:")
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	F5token, _ = bodyj.Get("token").Get("token").String()
}

func addRule(rule structs.Rulespec) {
	fmt.Println(rule.Name)
	fmt.Println(rule.Partition)
	fmt.Println(rule.ApiAnonymous)

	str, err := json.Marshal(rule)
	if err != nil {
		fmt.Println("Error preparing request")
	}
	str = bytes.Replace(str, []byte("\\u003e"), []byte(">"), -1)
	jsonStr := []byte(string(str))
	fmt.Println(string(str))
	urlStr := F5url + "/mgmt/tm/ltm/rule"
	req, _ := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	fmt.Println(bodyj)
}

func updateRule(rule structs.Rulespec) {

	fmt.Println(rule.Name)
	fmt.Println(rule.Partition)
	fmt.Println(rule.ApiAnonymous)

	str, err := json.Marshal(rule)
	if err != nil {
		fmt.Println("Error preparing request")
	}
	str = bytes.Replace(str, []byte("\\u003e"), []byte(">"), -1)
	jsonStr := []byte(string(str))
	fmt.Println(string(str))
	urlStr := F5url + "/mgmt/tm/ltm/rule/~" + rule.Partition + "~" + rule.Name
	req, _ := http.NewRequest("PATCH", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	fmt.Println(bodyj)

}

func attachRule(rule structs.Rulespec, partition string, virtual string) {
	rules := getRulesAttached(partition, virtual)
	rules = append(rules, "/"+partition+"/"+rule.Name)
	fmt.Println(rules)
	var virtualo structs.Virtualspec
	virtualo.Rules = rules
	fmt.Println(virtualo)

	str, err := json.Marshal(virtualo)
	if err != nil {
		fmt.Println("Error preparing request")
	}
	jsonStr := []byte(string(str))
	fmt.Println(string(str))
	urlStr := F5url + "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual
	req, _ := http.NewRequest("PATCH", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	fmt.Println(bodyj)

}

func ruleExists(router structs.Routerspec) bool {
	var toreturn bool
	toreturn = false
	partition, virtual, err := getF5pv(router)
	if err != nil {
		toreturn = false
	}
	rules := getRules(partition, virtual)
	for _, element := range rules {
		fmt.Println(element)
		if element == router.Domain+"-rule" {
			toreturn = true
		}

	}

	return toreturn
}

func ruleAttached(router structs.Routerspec, partition string, virtual string) bool {

	rules := getRulesAttached(partition, virtual)
	fmt.Println(rules)
	var toreturn bool
	toreturn = false
	for _, element := range rules {
		if element == "/"+partition+"/"+router.Domain+"-rule" {
			toreturn = true
		}
	}
	return toreturn
}

func buildRule(router structs.Routerspec, partition string, virtual string, db *sql.DB) structs.Rulespec {
	var rule structs.Rulespec
	rule.Name = router.Domain + "-rule"
	rule.Partition = partition
	var ruleinfo structs.RuleInfo
	ruleinfo.Domain = router.Domain
	var switches []structs.Switch
	for _, element := range router.Paths {
		var sw structs.Switch
		if element.Space != "default" {
			sw.Pool = element.App + "-" + element.Space + "-pool"
		}
		if element.Space == "default" {
			sw.Pool = element.App + "-pool"
		}
		sw.Path = element.Path
		sw.ReplacePath = element.ReplacePath
		appurl, err := getAppUrl(element.App, element.Space, db)
		if err != nil {
			fmt.Println(err)
			return rule
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
		fmt.Println(err)
	}
	wr.Flush()
	fmt.Println(string(b.Bytes()))

	/*
		var pathswitches string
		for _, element := range router.Paths {
			if element.Space != "default" {
				pathswitches = pathswitches + "\"" + element.Path + "*\"\n{\nHTTP::uri [string map -nocase {\"" + element.Path + "\" \"" + element.ReplacePath + "\"} [HTTP::uri]]\npool " + element.App + "-" + element.Space + "-pool}\n"

				//       pathswitches = pathswitches+"\""+element.Path+"*\"\n{\nHTTP::uri [string map -nocase {\""+element.Path+"/\" \"/\" \""+element.Path+"\" \"/\"} [HTTP::uri]]\npool "+element.App+"-"+element.Space+"-pool}\n"
			}
			if element.Space == "default" {
				pathswitches = pathswitches + "\"" + element.Path + "*\"\n{\nHTTP::uri [string map -nocase {\"" + element.Path + "\" \"" + element.ReplacePath + "\"} [HTTP::uri]]\npool " + element.App + "-pool}\n"
				//        pathswitches = pathswitches+"\""+element.Path+"*\"\n{\nHTTP::uri [string map -nocase {\""+element.Path+"/\" \"/\" \""+element.Path+"\" \"/\"} [HTTP::uri]]\npool "+element.App+"-pool}\n"
			}
		}

		rule.ApiAnonymous = "when HTTP_REQUEST {\nswitch [string tolower [HTTP::host]] {\n\"" + router.Domain + "\" {\nswitch -glob [string tolower [HTTP::uri]]\n{\n" + pathswitches + "}\n}\n}\n}"
	*/
	rule.ApiAnonymous = string(b.Bytes())
	fmt.Println(rule.ApiAnonymous)
	fmt.Println(rule.Name)
	return rule
}

func getRulesAttached(partition string, virtual string) []string {
	fmt.Println("getRulesAttached virutal " + virtual)
	urlStr := F5url + "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	fmt.Println(bodyj)
	var rulesa []string
	rules, _ := bodyj.Get("rules").Array()
	fmt.Println(rules)
	for index, _ := range rules {
		value := rules[index]
		rulesa = append(rulesa, value.(string))
	}
	fmt.Println("finishing getRulesAttached")
	return rulesa

}

func getRules(partition string, virtual string) []string {
	urlStr := F5url + "/mgmt/tm/ltm/rule?$filter=partition+eq+" + partition
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	type Rulesitems struct {
		Items []structs.Rulespec `json:"items"`
	}
	var ruleso Rulesitems
	bodybytes, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bodybytes, &ruleso)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(bodybytes))
	fmt.Println(ruleso)
	var toreturn []string
	for _, element := range ruleso.Items {
		toreturn = append(toreturn, element.Name)
	}
	return toreturn

}

func detachRule(rule string, partition string, virtual string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	rules := getRulesAttached(partition, virtual)
	//    fmt.Println(rules)
	var newrules []string
	for _, element := range rules {
		if element != "/"+partition+"/"+rule {
			newrules = append(newrules, element)
		}
	}
	//    fmt.Println(newrules)
	var virtualo structs.Virtualspec
	virtualo.Rules = newrules
	//    fmt.Println(virtualo)

	str, err := json.Marshal(virtualo)
	if err != nil {
		fmt.Println("Error preparing request")
		return msg, err
	}
	jsonStr := []byte(string(str))
	//    fmt.Println(string(str))
	urlStr := F5url + "/mgmt/tm/ltm/virtual/~" + partition + "~" + virtual

	req, _ := http.NewRequest("PATCH", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	if resp.StatusCode == 200 {
		fmt.Println("rule detached")
	}
	if resp.StatusCode != 200 {
		output, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			return msg, err
		}
		fmt.Printf("%s", output)
	}

	resp.Body.Close()
	msg.Status = 200
	msg.Message = "Rule detached"
	return msg, nil
}

func deleteRule(rulename string, partition string, virtual string) (m structs.Messagespec, e error) {
	var msg structs.Messagespec
	newrulename := strings.Replace(rulename, "/", "~", -1)
	newpartition := "~" + strings.Replace(partition, "/", "~", -1)
	urlStr := F5url + "/mgmt/tm/ltm/rule/" + newpartition + "~" + newrulename
	fmt.Println(urlStr)
	req, _ := http.NewRequest("DELETE", urlStr, nil)
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
		return msg, err
	}
	defer resp.Body.Close()
	fmt.Println(resp)
	if resp.StatusCode == 200 {
		fmt.Println("Rule deleted")
		msg.Status = 200
		msg.Message = "Rule Deleted"
	}
	return msg, nil

}

func Octhc(params martini.Params, r render.Render) {
	startClient()
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
	return partition, virtual, nil
}

func getAppUrl(app string, space string, db *sql.DB) (u string, e error) {
	var toreturn string
	externaldomain := os.Getenv("EXTERNAL_DOMAIN")
	internaldomain := os.Getenv("INTERNAL_DOMAIN")
	internal, err := spacepack.IsInternalSpace(db, space)
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}

	if internal {
		toreturn = app + "-" + space + "." + internaldomain
	}
	if !internal {

		toreturn = app + "-" + space + "." + externaldomain
		if space == "default" {
			toreturn = app + "." + externaldomain
		}
	}
	return toreturn, nil
}
