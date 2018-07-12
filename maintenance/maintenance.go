package maintenance

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	vault "github.com/akkeris/vault-client"
	"github.com/bitly/go-simplejson"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"net/http"
	"os"
	spacepack "region-api/space"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

var Client *http.Client
var F5url string
var F5token string
var creds structs.F5creds
var f5auth string
var inserttext string

func enableMaintenancePage(partition string, rulename string) (e error) {
	enabled, err := isMaintenancePageEnabled(partition, rulename)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if !enabled {
		rule, err := getrule(partition, rulename)
		if err != nil {
			fmt.Println(err)
			return err
		}
		rulearray := strings.Split(rule.APIAnonymous, "\n")
		a := rulearray
		var poolindex int
		for index, element := range a {
			if strings.HasPrefix(element, "pool /") {
				poolindex = index
			}
		}
		a = append(a[:poolindex], append([]string{inserttext}, a[poolindex:]...)...)
		newrule := strings.Join(a, "\n")
		fmt.Println(newrule)
		rule.APIAnonymous = newrule
		err = updaterule(partition, rulename, rule)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

func disableMaintenancePage(partition string, rulename string) (e error) {
	enabled, err := isMaintenancePageEnabled(partition, rulename)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if enabled {
		rule, err := getrule(partition, rulename)
		if err != nil {
			fmt.Println(err)
		}

		rulearray := strings.Split(rule.APIAnonymous, "\n")
		a := rulearray
		var maintindex int
		for index, element := range a {
			if strings.HasPrefix(element, "HTTP::respond") {
				maintindex = index
			}
		}
		a = append(a[:maintindex], a[maintindex+1:]...)

		newrule := strings.Join(a, "\n")
		fmt.Println(newrule)
		rule.APIAnonymous = newrule
		err = updaterule(partition, rulename, rule)
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

func isMaintenancePageEnabled(partition string, rulename string) (b bool, e error) {
	var toreturn bool
	toreturn = false
	rule, err := getrule(partition, rulename)
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}
	if strings.Contains(rule.APIAnonymous, "ifile get maintpage") {
		toreturn = true
	} else {
		toreturn = false
	}
	return toreturn, nil
}

func getrule(partition string, rulename string) (r structs.Rule, e error) {
	var rule structs.Rule
	urlStr := F5url + "/mgmt/tm/ltm/rule/~" + partition + "~" + rulename
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
		return rule, err
	}

	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	_ = json.Unmarshal(bb, &rule)
	return rule, nil
}

func updaterule(partition string, rulename string, rule structs.Rule) (e error) {
	str, err := json.Marshal(rule)
	if err != nil {
		fmt.Println(err)
		return err
	}
	jsonStr := []byte(string(str))
	urlStr := F5url + "/mgmt/tm/ltm/rule/~" + partition + "~" + rulename
	req, _ := http.NewRequest("PATCH", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("X-F5-Auth-Token", F5token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}

	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(resp.Status)
	fmt.Println(string(bb))
	return nil
}

func startclient() {
	f5secret := os.Getenv("F5_SECRET")
	F5url = vault.GetField(f5secret, "url")
	f5username := vault.GetField(f5secret, "username")
	f5password := vault.GetField(f5secret, "password")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	Client = &http.Client{Transport: tr}
	data := []byte(f5username + ":" + f5password)
	dstr := base64.StdEncoding.EncodeToString(data)
	f5auth = "Basic " + dstr

	creds.Username = f5username
	creds.Password = f5password
	creds.LoginProviderName = "tmos"
	str, err := json.Marshal(creds)
	if err != nil {
		fmt.Println("Error preparing request")
	}
	jsonStr := []byte(string(str))
	urlStr := F5url + "/mgmt/shared/authn/login"
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonStr))
	req.Header.Add("Authorization", f5auth)
	req.Header.Add("Content-Type", "application/json")
	resp, err := Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bodyj, _ := simplejson.NewFromReader(resp.Body)
	F5token, _ = bodyj.Get("token").Get("token").String()
}

func getPartition(db *sql.DB, space string) (p string, e error) {
	internalpartition := os.Getenv("F5_PARTITION_INTERNAL")
	externalpartition := os.Getenv("F5_PARTITION")
	internal, err := spacepack.IsInternalSpace(db, space)
	var toreturn string
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	if internal {
		toreturn = internalpartition
	}
	if !internal {
		toreturn = externalpartition
	}
	return toreturn, nil
}

func EnableMaintenancePage(db *sql.DB, params martini.Params, r render.Render) {
	var msg structs.Messagespec
	space := params["space"]
	app := params["app"]

	inserttext = `HTTP::respond 200 content [ifile get maintpage] "Content-Type" "text/html" "Connection" "Close" return`
	startclient()
	var rulename string
	var partition string

	partition, err := getPartition(db, space)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	if space != "default" {
		rulename = app + "-" + space + "-rule"
	}
	if space == "default" {
		rulename = app + "-rule"
	}

	err = enableMaintenancePage(partition, rulename)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	msg.Status = 201
	msg.Message = "Maintenance Page Enabled"
	r.JSON(msg.Status, msg)

}

func DisableMaintenancePage(db *sql.DB, params martini.Params, r render.Render) {
	var msg structs.Messagespec
	space := params["space"]
	app := params["app"]

	startclient()
	var rulename string
	var partition string

	partition, err := getPartition(db, space)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	if space != "default" {
		rulename = app + "-" + space + "-rule"
	}
	if space == "default" {
		rulename = app + "-rule"
	}
	err = disableMaintenancePage(partition, rulename)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	msg.Status = 200
	msg.Message = "Maintenance Page Disabled"
	r.JSON(msg.Status, msg)

}

func MaintenancePageStatus(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	app := params["app"]

	startclient()
	var rulename string
	var partition string

	partition, err := getPartition(db, space)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	if space != "default" {
		rulename = app + "-" + space + "-rule"
	}
	if space == "default" {
		rulename = app + "-rule"
	}

	enabled, err := isMaintenancePageEnabled(partition, rulename)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	var enabledstring string
	if enabled {
		enabledstring = "on"
	}
	if !enabled {
		enabledstring = "off"
	}
	var response structs.Maintenancespec
	response.App = app
	response.Space = space
	response.Status = enabledstring

	r.JSON(200, response)
}
