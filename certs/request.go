package certs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	vault "github.com/akkeris/vault-client"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"net/http"
	"os"
	router "region-api/router"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"
)

var oidEmailAddress = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}

func GetCertStatus(db *sql.DB, params martini.Params, r render.Render) {
	var id string
	id = params["id"]
	var certspec structs.CertificateRequestSpec
	certspec, err := getCertStatus(db, id)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, certspec)

}

func GetCerts(db *sql.DB, params martini.Params, r render.Render) {
	var certlist []structs.CertificateRequestSpec
	certlist, err := getCerts(db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, certlist)
}

func getPemCertFromBundle(server_name string, pem_certs []byte) ([]byte, error) {
	block, _ := pem.Decode(pem_certs)

	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		return nil, err
	}
	var found_cert *x509.Certificate = nil
	for _, cert := range certs {
		if cert.Subject.CommonName == server_name {
			found_cert = cert
		}
	}
	if found_cert == nil {
		return nil, errors.New("Unable to find certificate in bundle!")
	}
	pem_cert := pem.EncodeToMemory(&pem.Block{
		Type:    "CERTIFICATE",
		Headers: nil,
		Bytes:   found_cert.Raw,
	})
	return pem_cert, nil
}

func InstallCert(db *sql.DB, params martini.Params, r render.Render) {
	var id string
	id = params["id"]

	certspec, err := getCert(db, id)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	order, err := getOrder(certspec.Order)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	pem_certs, err := getCertPEM(strconv.Itoa(order.Certificate.ID))
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	pem_cert, err := getPemCertFromBundle(order.Certificate.CommonName, pem_certs)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	key_name := strings.Replace(order.Certificate.CommonName, "*.", "star.", -1)
	pem_key := getKeyFromVault(key_name)

	err = router.InstallNewCert(os.Getenv("F5_PARTITION"), os.Getenv("F5_VIRTUAL"), pem_cert, []byte(pem_key))
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	err = router.InstallNewCert(os.Getenv("F5_PARTITION_INTERNAL"), os.Getenv("F5_VIRTUAL_INTERNAL"), pem_cert, []byte(pem_key))
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, certspec)
}

func getCertStatus(db *sql.DB, id string) (c structs.CertificateRequestSpec, e error) {
	certspec, err := getCert(db, id)
	if err != nil {
		fmt.Println(err)
		return certspec, err
	}

	order, err := getOrder(certspec.Order)
	if err != nil {
		fmt.Println(err)
		return certspec, err
	}
	certspec.OrderStatus = order.Status
	requestobject, err := getRequestObject(certspec.Request)
	if err != nil {
		fmt.Println(err)
		return certspec, err
	}
	certspec.Requestedby = requestobject.Requester.Email
	certspec.Requesteddate = requestobject.Order.Requests[0].Date.String()
	certspec.Comment = requestobject.Order.Requests[0].Comments
	certspec.RequestStatus = requestobject.Status
	certspec.ValidFrom = order.Certificate.ValidFrom
	certspec.ValidTo = order.Certificate.ValidTill
	certspec.SignatureHash = order.Certificate.SignatureHash
	return certspec, nil

}

func getOrder(order string) (o structs.OrderSpec, e error) {
	var orderspec structs.OrderSpec
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		fmt.Println("Need XDCDEVKEY")
		return orderspec, errors.New("Need XDCDEVKEY")
	}
	var client *http.Client
	client = &http.Client{}
	req, _ := http.NewRequest("GET", os.Getenv("DIGICERT_URL")+"/order/certificate/"+order, nil)
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return orderspec, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bb, &orderspec)
	if err != nil {
		fmt.Println(err)
		return orderspec, err
	}
	return orderspec, nil

}

func getCertPEM(cert_id string) (o []byte, e error) {
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		fmt.Println("Need XDCDEVKEY")
		return nil, errors.New("Need XDCDEVKEY")
	}
	var client *http.Client
	client = &http.Client{}
	req, _ := http.NewRequest("GET", os.Getenv("DIGICERT_URL")+"/certificate/"+cert_id+"/download/format/pem_all", nil)
	req.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Println(string(bb[:]))
		return nil, errors.New(resp.Status + " " + string(bb[:]))
	}
	return bb, nil
}

func getCert(db *sql.DB, idrequested string) (c structs.CertificateRequestSpec, e error) {
	var certspec structs.CertificateRequestSpec
	var (
		id      string
		cn      string
		san     string
		request string
		order   string
	)
	stmt, err := db.Prepare("select id, cn, san, request, ordernumber from certs where id = $1")
	if err != nil {
		fmt.Println(err)
		return certspec, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(idrequested)
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&id, &cn, &san, &request, &order)
		if err != nil {
			fmt.Println(err)
			return certspec, err
		}
		certspec.ID = id
		certspec.CN = cn
		certspec.SAN = strings.Split(san, ",")
		certspec.Request = request
		certspec.Order = order
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return certspec, err
	}

	return certspec, nil
}

func getCerts(db *sql.DB) (c []structs.CertificateRequestSpec, e error) {
	var certspecs []structs.CertificateRequestSpec
	var (
		id  string
		cn  string
		san string
	)
	stmt, err := db.Prepare("select id, cn, san from certs")
	if err != nil {
		fmt.Println(err)
		return certspecs, err
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()
	for rows.Next() {
		var certspec structs.CertificateRequestSpec
		err := rows.Scan(&id, &cn, &san)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		certspec.ID = id
		certspec.CN = cn
		certspec.SAN = strings.Split(san, ",")
		certspecs = append(certspecs, certspec)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return certspecs, nil
}

func CertificateRequest(db *sql.DB, requestin structs.CertificateRequestSpec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	err, request_response := certificateRequest(requestin, db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(201, request_response)
}

func certificateRequest(request structs.CertificateRequestSpec, db *sql.DB) (error, *structs.CertificateRequest) {
	exists, err := certExists(request)
	if err != nil {
		fmt.Println("certExists", err)
		return err, nil
	}
	if exists == true {
		return errors.New("Cert already exists"), nil
	}

	csr, certtype, err := generateRequest(request)
	if err != nil {
		fmt.Println("generateRequest", err)
		return err, nil

	}
	response, err := sendRequestToDigicert(csr, certtype)

	if err != nil {
		fmt.Println("sendRequestToDigicert",err)
		return err, nil
	}
	request.Request = strconv.Itoa(response.Requests[0].ID)
	requestobject, err := getRequestObject(request.Request)
	if err != nil {
		fmt.Println("getRequestObject", err)
		return err, nil
	}
	request.Order = strconv.Itoa(requestobject.Order.ID)
	err, uuid := addRequestToDB(request, db)
	if err != nil {
		fmt.Println("addRequestToDB", err)
		return err, nil
	}
	cert_req := structs.CertificateRequest{
		Response: response,
		ID:       uuid,
	}
	return nil, &cert_req
}

func getList() (o structs.OrderList, e error) {
	var orderlist structs.OrderList
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		fmt.Println("Need XDCDEVKEY")
		return orderlist, errors.New("Need XDCDEVKEY")
	}
	var client *http.Client
	client = &http.Client{}
	req, _ := http.NewRequest("GET", os.Getenv("DIGICERT_URL")+"/order/certificate", nil)
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return orderlist, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bb, &orderlist)
	if err != nil {
		fmt.Println(err)
		return orderlist, err
	}
	return orderlist, nil
}

func certExists(request structs.CertificateRequestSpec) (b bool, e error) {
	var toreturn bool
	toreturn = false
	orderlist, err := getList()
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}
	for _, element := range orderlist.Orders {
		if element.Status != "revoked" && element.Status != "expired" && element.Product.Name != "Digital Signature Plus" && element.Product.Name != "Code Signing" {
			if element.Certificate.CommonName == request.CN {
				fmt.Println("Cert already exists")
				toreturn = true
				return toreturn, nil
			}
			for _, dnsname := range element.Certificate.DNSNames {
				if isInArray(dnsname, request.SAN) {
					fmt.Println("Requested cert is a SAN of " + element.Certificate.CommonName)
					toreturn = true
					return toreturn, nil
				}
			}

		}
	}

	return toreturn, nil
}

func generateRequest(request structs.CertificateRequestSpec) (r structs.DigicertRequest, t string, e error) {
	var csr structs.DigicertRequest
	var nameslist []string
	nameslist = append(nameslist, request.CN)
	nameslist = append(nameslist, request.SAN...)
	primaryname := nameslist[0]

	keyBytes, _ := rsa.GenerateKey(rand.Reader, 2048)
	emailAddress := os.Getenv("CERT_EMAIL")
	subj := pkix.Name{
		CommonName:         nameslist[0],
		Country:            []string{os.Getenv("CERT_COUNTRY")},
		Province:           []string{os.Getenv("CERT_PROVINCE")},
		Locality:           []string{os.Getenv("CERT_LOCALITY")},
		Organization:       []string{os.Getenv("CERT_ORG")},
		OrganizationalUnit: []string{os.Getenv("CERT_ORG_UNIT")},
	}
	rawSubj := subj.ToRDNSequence()
	rawSubj = append(rawSubj, []pkix.AttributeTypeAndValue{
		{Type: oidEmailAddress, Value: emailAddress},
	})

	asn1Subj, _ := asn1.Marshal(rawSubj)
	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		EmailAddresses:     []string{emailAddress},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	if len(nameslist) > 1 {
		template.DNSNames = nameslist
	}

	csrBytes, _ := x509.CreateCertificateRequest(rand.Reader, &template, keyBytes)
	csrwrite := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	request.CSR = string(csrwrite)

	keywrite := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(keyBytes)})
	request.Key = fmt.Sprintf("%v", string(keywrite))

	addRequestToVault(request)
	csrobj, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		fmt.Println(err)
		return csr, "", err
	}
	csr.Certificate.CommonName = csrobj.Subject.CommonName
	if len(nameslist) > 1 {
		csr.Certificate.DNSNames = csrobj.DNSNames
	}
	csr.Certificate.Csr = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}))
	csr.Certificate.OrganizationUnits = []string{csrobj.Subject.OrganizationalUnit[0]}
	csr.Certificate.ServerPlatform.ID = -1
	csr.Certificate.SignatureHash = "sha256"
	csr.Organization.ID = 95612
	validityyears := 2
	if os.Getenv("CERT_VALIDITY_YEARS") != "" {
		validityyears, err = strconv.Atoi(os.Getenv("CERT_VALIDITY_YEARS"))
		if err != nil {
			fmt.Println("The CERT_VALIDITY_YEARS was invalid, it must be a positive number.")
			fmt.Println(err)
			validityyears = 2
		}
	}

	csr.ValidityYears = validityyears
	csr.Comments = "Requested by: " + request.Requestedby + ". " + request.Comment

	var certtype string
	if len(nameslist) > 1 {
		certtype = "ssl_multi_domain"
	}
	if len(nameslist) == 1 {
		certtype = "ssl_plus"
	}
	if strings.HasPrefix(primaryname, "*.") {
		certtype = "ssl_wildcard"
	}
	return csr, certtype, nil
}

func sendRequestToDigicert(csr structs.DigicertRequest, certtype string) (r structs.CertificateRequestResponseSpec, e error) {
	var response structs.CertificateRequestResponseSpec
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		fmt.Println("Need XDCDEVKEY")
		return response, errors.New("Need XDCDEVKEY")
	}
	jsonStr, err := json.Marshal(csr)
	if err != nil {
		fmt.Println(err)
		return response, err
	}
	var client *http.Client
	client = &http.Client{}

	req, _ := http.NewRequest("POST", os.Getenv("DIGICERT_REQUEST_URL")+"/order/certificate/"+certtype, bytes.NewBuffer(jsonStr))
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return response, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bb, &response)
	if err != nil {
		fmt.Println(err)
		return response, err
	}
	return response, nil
}

func getRequestObject(requestnumber string) (c structs.CertificateRequestObject, e error) {
	var requestobject structs.CertificateRequestObject
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		fmt.Println("Need XDCDEVKEY")
		return requestobject, errors.New("Need XDCDEVKEY")
	}
	var client *http.Client
	client = &http.Client{}
	req2, _ := http.NewRequest("GET", os.Getenv("DIGICERT_URL")+"/request/"+requestnumber, nil)
	req2.Header.Add("Content-type", "application/json")
	req2.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Println(err)
		return requestobject, err
	}
	defer resp2.Body.Close()
	bb2, _ := ioutil.ReadAll(resp2.Body)
	err = json.Unmarshal(bb2, &requestobject)
	if err != nil {
		fmt.Println(err)
		return requestobject, err
	}
	return requestobject, err
}

func isInArray(wanted string, array []string) bool {
	for _, v := range array {
		if v == wanted {
			return true
		}
	}
	return false
}

func getKeyFromVault(CN string) (key string) {
	path := os.Getenv("VAULT_CERT_STORAGE") + CN
	key_pem := strings.Replace(vault.GetField(path, "key"), "\\\\n", "\n", -1)
	return key_pem
}

func addRequestToVault(request structs.CertificateRequestSpec) (e error) {
	path := os.Getenv("VAULT_CERT_STORAGE") + request.CN
	path = strings.Replace(path, "*.", "star.", -1)
	err := vault.WriteField(path, "key", request.Key)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = vault.WriteField(path, "csr", request.CSR)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func addRequestToDB(request structs.CertificateRequestSpec, db *sql.DB) (error, string) {
	newuuid, _ := uuid.NewV4()
	newid := newuuid.String()
	var id string

	inserterr := db.QueryRow("INSERT INTO certs(id, request, cn, san, ordernumber ) VALUES($1,$2,$3,$4,$5) returning id;", newid, request.Request, request.CN, strings.Join(request.SAN, ","), request.Order).Scan(&id)
	if inserterr != nil {
		fmt.Println(inserterr)
		return inserterr, ""
	}

	return nil, id

}
