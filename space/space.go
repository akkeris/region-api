package space

import (
	"database/sql"
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"net/http"
	"os"
	runtime "region-api/runtime"
	structs "region-api/structs"
	utils "region-api/utils"
	"strings"
)

func UpdateSpaceTags(db *sql.DB, space structs.Spacespec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if strings.Contains(space.ComplianceTags, ",") {
		space.ComplianceTags = strings.Replace(space.ComplianceTags, ",", "-", -1)
	}
	space.ComplianceTags = strings.Replace(space.ComplianceTags, " ", "", -1)

	rt, err := runtime.GetRuntimeFor(db, space.Name)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	err = rt.UpdateSpaceTags(space.Name, space.ComplianceTags)

	if err != nil {
		utils.ReportError(err, r)
		return
	}
	_, err = db.Exec("UPDATE spaces set compliancetags = $1 where name = $2", space.ComplianceTags, space.Name)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "space updated"})
}

func Createspace(db *sql.DB, space structs.Spacespec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	if space.Name == "kube-system" || space.Name == "default" || space.Name == "kube-public" || space.Name == "akkeris" {
		utils.ReportInvalidRequest("The space name is invalid or reserved keywords", r)
		return
	}

	if _, err := getSpace(db, space.Name); err == nil {
		utils.ReportInvalidRequest("The specified space is already taken.", r)
		return
	}

	if space.Stack == "" {
		space.Stack = os.Getenv("DEFAULT_STACK")
		if space.Stack == "" {
			space.Stack = "ds1"
		}
	}

	// this must happen before GetRuntimeFor.
	if _, err := addSpace(db, space); err != nil {
		utils.ReportError(err, r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space.Name)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if err = rt.CreateSpace(space.Name, space.Internal, space.ComplianceTags); err != nil {
		utils.ReportError(err, r)
		return
	}
	if os.Getenv("FF_QUAY") == "true" || os.Getenv("IMAGE_PULL_SECRET") != "" {
		vs, err := getPullSecret(space.Name)
		if err != nil {
			utils.ReportError(err, r)
			return
		}

		// This secret created used to be passed in to AddImagePullSecretToSpace, but that method
		// did nothing with it, so we don't in the refactor.
		if _, err = rt.CreateSecret(space.Name, vs.Data.Name, vs.Data.Base64, "kubernetes.io/dockerconfigjson"); err != nil {
			utils.ReportError(err, r)
			return
		}

		if err = rt.AddImagePullSecretToSpace(space.Name); err != nil {
			utils.ReportError(err, r)
			return
		}
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "space created"})
}

func Deletespace(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]

	if space == "" {
		utils.ReportInvalidRequest("The space was blank or invalid.", r)
		return
	}

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if _, err = getSpace(db, space); err != nil {
		if err.Error() == "sql: no rows in result set" {
			r.JSON(http.StatusNotFound, structs.Messagespec{Status: http.StatusNotFound, Message: "The specified space does not exist"})
			return
		} else {
			utils.ReportError(err, r)
			return
		}
	}

	pods, err := rt.GetPodsBySpace(space)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if len(pods.Items) != 0 {
		r.JSON(http.StatusConflict, structs.Messagespec{Status: http.StatusConflict, Message: "The space cannot be deleted as it still has pods in it."})
		return
	}

	var appsCount int
	err = db.QueryRow("select count(*) from spacesapps where space = $1", space).Scan(&appsCount)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	if appsCount > 0 {
		r.JSON(http.StatusConflict, structs.Messagespec{Status: http.StatusConflict, Message: "The space cannot be deleted as it still has apps in it."})
		return
	}

	if err = rt.DeleteSpace(space); err != nil {
		utils.ReportError(err, r)
		return
	}

	// this must happen after GetRuntimeFor.
	if _, err = db.Exec("delete from spaces where name = $1", space); err != nil {
		utils.ReportError(err, r)
		return
	}

	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "space deleted"})
}

func getPullSecret(datacenter string) (vs structs.Vaultsecretspec, err error) {
	var vsecret structs.Vaultsecretspec
	vaulttoken := os.Getenv("VAULT_TOKEN")
	vaultaddr := os.Getenv("VAULT_ADDR")
	secretPath := os.Getenv("QUAY_PULL_SECRET") // deprecated name
	if secretPath == "" {
		secretPath = os.Getenv("IMAGE_PULL_SECRET")
	}
	vaultaddruri := vaultaddr + "/v1/" + secretPath
	vreq, err := http.NewRequest("GET", vaultaddruri, nil)
	vreq.Header.Add("X-Vault-Token", vaulttoken)
	vclient := &http.Client{}
	vresp, err := vclient.Do(vreq)
	if err != nil {
		return vsecret, err
	}
	defer vresp.Body.Close()

	bb, _ := ioutil.ReadAll(vresp.Body)
	_ = json.Unmarshal(bb, &vsecret)
	return vsecret, nil
}

func addSpace(db *sql.DB, space structs.Spacespec) (msg structs.Messagespec, err error) {
	var name string
	var spaceinsert bool
	if space.Internal == true {
		spaceinsert = true
	} else {
		spaceinsert = false
	}
	var inserterr error
	if len(space.ComplianceTags) > 0 {
		inserterr = db.QueryRow("INSERT INTO spaces(name,internal,compliancetags,stack) VALUES($1,$2,$3,$4) returning name;", space.Name, spaceinsert, space.ComplianceTags, space.Stack).Scan(&name)
	} else {
		inserterr = db.QueryRow("INSERT INTO spaces(name,internal,stack) VALUES($1,$2,$3) returning name;", space.Name, spaceinsert, space.Stack).Scan(&name)
	}

	if inserterr != nil {
		return structs.Messagespec{Status: http.StatusInternalServerError, Message: inserterr.Error()}, inserterr
	}

	return structs.Messagespec{Status: http.StatusOK, Message: "Created space"}, nil
}

func Space(db *sql.DB, params martini.Params, r render.Render) {
	space, err := getSpace(db, params["space"])
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			r.JSON(http.StatusNotFound, structs.Messagespec{Status: http.StatusNotFound, Message: "The specified space does not exist"})
			return
		} else {
			utils.ReportError(err, r)
			return
		}
	}
	r.JSON(200, space)
}

func getSpace(db *sql.DB, space string) (s structs.Spacespec, e error) {
	var spaceobject structs.Spacespec
	var internal bool
	var stack string
	var compliancetags string
	err := db.QueryRow("select internal, COALESCE(compliancetags, '') as compliancetags, stack from spaces where name = $1", space).Scan(&internal, &compliancetags, &stack)
	if err != nil {
		return spaceobject, err
	}
	spaceobject.Name = space
	spaceobject.Internal = internal
	spaceobject.ComplianceTags = compliancetags
	spaceobject.Stack = stack
	return spaceobject, nil
}

func Listspaces(db *sql.DB, params martini.Params, r render.Render) {
	var spaces structs.Spacelist
	var (
		name string
	)
	stmt, err := db.Prepare("select name from spaces")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()
	var spacelist []string
	for rows.Next() {
		err := rows.Scan(&name)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		spacelist = append(spacelist, name)
	}
	spaces.Spaces = spacelist
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, spaces)
}

