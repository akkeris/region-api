package utils

import (
	"github.com/bitly/go-simplejson"
	"net/http"
	"os"
)

var AuthUser string
var AuthPassword string

func InitAuth() {

	vaulttoken := os.Getenv("VAULT_TOKEN")
	vaultaddr := os.Getenv("VAULT_ADDR")

	kubernetescertsecret := os.Getenv("ALAMO_API_AUTH_SECRET")
	vaultaddruri := vaultaddr + "/v1/" + kubernetescertsecret
	vreq, err := http.NewRequest("GET", vaultaddruri, nil)
	vreq.Header.Add("X-Vault-Token", vaulttoken)
	vclient := &http.Client{}
	vresp, err := vclient.Do(vreq)
	if err != nil {
	}
	defer vresp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(vresp.Body)
	AuthUser, _ = bodyj.Get("data").Get("username").String()
	AuthPassword, _ = bodyj.Get("data").Get("password").String()

}
