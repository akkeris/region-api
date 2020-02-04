package vault

import (
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	"strings"
	"sync"
)

var list []string
var mutex = &sync.Mutex{}

func GetList() []string {
	return list
}

func getVaultList(path string) ([]string) {
	vault_addr := os.Getenv("VAULT_ADDR")
	vault_token := os.Getenv("VAULT_TOKEN")
	var vaultlist structs.VaultList
	url := vault_addr + "/v1/" + path + "?list=true"
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("X-Vault-Token", vault_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	_ = json.Unmarshal(bb, &vaultlist)
	var list []string
	if len(vaultlist.Data.Keys) == 0 {
		list = append(list, "vault:"+path)
	}
	for _, element := range vaultlist.Data.Keys {
		if strings.HasSuffix(element, "/") {
			newelement := strings.Replace(element, "/", "", -1)
			rlist := getVaultList(path + "/" + newelement)
			list = append(list, rlist...)
		}
		if !strings.HasSuffix(element, "/") {
			list = append(list, "vault:"+path+"/"+element)
		}
	}
	return list
}

func GetVaultPaths() ([]string) {
	return strings.Split(os.Getenv("SECRETS"), ",")
}

func GetVaultListPeriodic() {
	var newlist []string
	secrets_array := GetVaultPaths();
	for _, element := range secrets_array {
		newlist = append(newlist, getVaultList(element)...)
	}
	mutex.Lock()
	list = newlist
	mutex.Unlock()
}

func GetVaultVariables(secret string) ([]structs.Creds) {
	type Creds struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	vault_addr := os.Getenv("VAULT_ADDR")
	vault_token := os.Getenv("VAULT_TOKEN")
	url := vault_addr + "/v1/" + secret
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("X-Vault-Token", vault_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	data, _ := bodyj.Get("data").Map()
	step1 := strings.Replace(secret, "secret", os.Getenv("VAULT_PREFIX"), -1)
	step2 := strings.Replace(step1, "/dev/", "/", -1)
	step3 := strings.Replace(step2, "/qa/", "/", -1)
	step4 := strings.Replace(step3, "/prod/", "/", -1)
	step5 := strings.Replace(step4, "/stage/", "/", -1)
	step6 := strings.Replace(step5, "/stg/", "/", -1)
	step7 := strings.Replace(step6, "/intp-prod/", "/", -1)
	step8 := strings.Replace(step7, "/xo/", "/", -1)
	step9 := strings.Replace(step8, "/", "_", -1)
	prefix := strings.ToUpper(step9)
	var creds []structs.Creds
	for k, v := range data {
		upperfield := strings.ToUpper(k)
		var cred structs.Creds
		cred.Key = prefix + "_" + upperfield
		cred.Value = v.(string)
		creds = append(creds, cred)
	}
	return creds
}

func GetVaultVariablesMasked(secret string) ([]structs.Creds) {
	var masked []structs.Creds
	creds := GetVaultVariables(secret)
	for _, element := range creds {
		ukey := strings.ToUpper(element.Key)
		if strings.Contains(ukey, "PASSWORD") || strings.Contains(ukey, "SECRET") || strings.Contains(ukey, "KEY") || strings.Contains(ukey, "TOKEN") {
			if strings.HasPrefix(secret, "secret/prod") || strings.HasPrefix(secret, "secret/stage") || strings.HasPrefix(secret, "secret/stg") || strings.HasPrefix(secret, "secret/xo") {
				element.Value = "(redacted)"
			}
			masked = append(masked, element)
		}
		if !strings.Contains(ukey, "PASSWORD") && !strings.Contains(ukey, "SECRET") && !strings.Contains(ukey, "KEY") && !strings.Contains(ukey, "TOKEN") {
			masked = append(masked, element)
		}
	}
	return masked
}

