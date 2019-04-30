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
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"net/http"
	"os"
	structs "region-api/structs"
	"region-api/router"
	"strconv"
	"strings"
	"time"
)

type digiCertCertificateRequestSpec struct {
	ID            string   `json:"id"`
	Comment       string   `json:"comment,omitempty"`
	CN            string   `json:"cn"`
	SAN           []string `json:"san"`
	Key           string   `json:"key,omitempty"`
	CSR           string   `json:"csr,omitempty"`
	Request       string   `json:"request,omitempty"`
	Requestedby   string   `json:"requestedby,omitempty"`
	Requesteddate string   `json:"requesteddate,omitempty"`
	RequestStatus string   `json:"requeststatus,omitempty"`
	Order         string   `json:"order,omitempty"`
	OrderStatus   string   `json:"orderstatus,omitempty"`
	Installed     bool     `json:"installed,omitempty"`
	Installeddate string   `json:"installeddate,omitempty"`
	ValidFrom     string   `json:"validfrom,omitempty"`
	ValidTo       string   `json:"validto,omitempty"`
	VIP           string   `json:"vip,omitempty"`
	SignatureHash string   `json:"signature,omitempty"`
}

type digiCertRequest struct {
	Certificate struct {
		CommonName        string   `json:"common_name"`
		DNSNames          []string `json:"dns_names,omitempty"`
		Csr               string   `json:"csr"`
		OrganizationUnits []string `json:"organization_units"`
		ServerPlatform    struct {
			ID int `json:"id"`
		} `json:"server_platform"`
		SignatureHash string `json:"signature_hash"`
	} `json:"certificate"`
	Organization struct {
		ID int `json:"id"`
	} `json:"organization"`
	ValidityYears int    `json:"validity_years"`
	Comments      string `json:"comments,omitempty"`
}

type digiCertCertificateRequest struct {
	Response digiCertCertificateResponse `json:request`
	ID       string                         `json:"id"`
}

type digiCertCertificateResponse struct {
	ID       int `json:"id"`
	Requests []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"requests"`
}

type digiCertCertificateRequestObject struct {
	ID            int       `json:"id"`
	Date          time.Time `json:"date"`
	Type          string    `json:"type"`
	Status        string    `json:"status"`
	DateProcessed time.Time `json:"date_processed"`
	Requester     struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"requester"`
	Processor struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"processor"`
	Order struct {
		ID          int `json:"id"`
		Certificate struct {
			ID           int       `json:"id"`
			CommonName   string    `json:"common_name"`
			DNSNames     []string  `json:"dns_names"`
			DateCreated  time.Time `json:"date_created"`
			Csr          string    `json:"csr"`
			Organization struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				City    string `json:"city"`
				State   string `json:"state"`
				Country string `json:"country"`
			} `json:"organization"`
			ServerPlatform struct {
				ID         int    `json:"id"`
				Name       string `json:"name"`
				InstallURL string `json:"install_url"`
				CsrURL     string `json:"csr_url"`
			} `json:"server_platform"`
			SignatureHash string `json:"signature_hash"`
			KeySize       int    `json:"key_size"`
			CaCert        struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"ca_cert"`
		} `json:"certificate"`
		Status       string    `json:"status"`
		IsRenewal    bool      `json:"is_renewal"`
		DateCreated  time.Time `json:"date_created"`
		Organization struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			City    string `json:"city"`
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"organization"`
		ValidityYears               int  `json:"validity_years"`
		DisableRenewalNotifications bool `json:"disable_renewal_notifications"`
		AutoRenew                   int  `json:"auto_renew"`
		Container                   struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"container"`
		Product struct {
			NameID                string `json:"name_id"`
			Name                  string `json:"name"`
			Type                  string `json:"type"`
			ValidationType        string `json:"validation_type"`
			ValidationName        string `json:"validation_name"`
			ValidationDescription string `json:"validation_description"`
		} `json:"product"`
		OrganizationContact struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			JobTitle  string `json:"job_title"`
			Telephone string `json:"telephone"`
		} `json:"organization_contact"`
		TechnicalContact struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
			JobTitle  string `json:"job_title"`
			Telephone string `json:"telephone"`
		} `json:"technical_contact"`
		User struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
		} `json:"user"`
		Requests []struct {
			ID       int       `json:"id"`
			Date     time.Time `json:"date"`
			Type     string    `json:"type"`
			Status   string    `json:"status"`
			Comments string    `json:"comments"`
		} `json:"requests"`
		CsProvisioningMethod string `json:"cs_provisioning_method"`
		ShipInfo             struct {
			Name    string `json:"name"`
			Addr1   string `json:"addr1"`
			Addr2   string `json:"addr2"`
			City    string `json:"city"`
			State   string `json:"state"`
			Zip     int    `json:"zip"`
			Country string `json:"country"`
			Method  string `json:"method"`
		} `json:"ship_info"`
	} `json:"order"`
	Comments         string `json:"comments"`
	ProcessorComment string `json:"processor_comment"`
}

type digiCertOrderList struct {
	Orders []struct {
		ID          int `json:"id"`
		Certificate struct {
			ID            int      `json:"id"`
			CommonName    string   `json:"common_name"`
			DNSNames      []string `json:"dns_names"`
			ValidTill     string   `json:"valid_till"`
			SignatureHash string   `json:"signature_hash"`
		} `json:"certificate"`
		Status       string    `json:"status"`
		IsRenewed    bool      `json:"is_renewed"`
		DateCreated  time.Time `json:"date_created"`
		Organization struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"organization"`
		ValidityYears int `json:"validity_years"`
		Container     struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"container"`
		Product struct {
			NameID string `json:"name_id"`
			Name   string `json:"name"`
			Type   string `json:"type"`
		} `json:"product"`
		HasDuplicates bool   `json:"has_duplicates"`
		Price         int    `json:"price"`
		ProductNameID string `json:"product_name_id"`
	} `json:"orders"`
	Page struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"page"`
}

type digiCertOrderSpec struct {
	ID          int `json:"id"`
	Certificate struct {
		ID           int       `json:"id"`
		Thumbprint   string    `json:"thumbprint"`
		SerialNumber string    `json:"serial_number"`
		CommonName   string    `json:"common_name"`
		DNSNames     []string  `json:"dns_names"`
		DateCreated  time.Time `json:"date_created"`
		ValidFrom    string    `json:"valid_from"`
		ValidTill    string    `json:"valid_till"`
		Csr          string    `json:"csr"`
		Organization struct {
			ID int `json:"id"`
		} `json:"organization"`
		OrganizationUnits []string `json:"organization_units"`
		ServerPlatform    struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			InstallURL string `json:"install_url"`
			CsrURL     string `json:"csr_url"`
		} `json:"server_platform"`
		SignatureHash string `json:"signature_hash"`
		KeySize       int    `json:"key_size"`
		CaCert        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"ca_cert"`
	} `json:"certificate"`
	Status string `json:"status"`
}

var oidEmailAddress = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}

func getOrder(order string) (o digiCertOrderSpec, e error) {
	var orderspec digiCertOrderSpec
	XDCDEVKEY := vault.GetField(os.Getenv("DIGICERT_SECRET"), "xdcdevkey")
	if XDCDEVKEY == "" {
		return orderspec, errors.New("Need XDCDEVKEY")
	}
	var client *http.Client
	client = &http.Client{}
	req, _ := http.NewRequest("GET", os.Getenv("DIGICERT_URL")+"/order/certificate/"+order, nil)
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("X-DC-DEVKEY", XDCDEVKEY)
	resp, err := client.Do(req)
	if err != nil {
		return orderspec, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bb, &orderspec)
	if err != nil {
		return orderspec, err
	}
	return orderspec, nil
}

func getCert(db *sql.DB, id string) (c digiCertCertificateRequestSpec, e error) {
	var certspec digiCertCertificateRequestSpec
	var (
		cn      string
		san     string
		request string
		order   string
	)
	if err := db.QueryRow("select id, cn, san, request, ordernumber from certs where id = $1", id).Scan(&id, &cn, &san, &request, &order); err != nil {
		return certspec, err
	}	
	certspec.ID = id
	certspec.CN = cn
	certspec.SAN = strings.Split(san, ",")
	certspec.Request = request
	certspec.Order = order
	return certspec, nil
}

func getCerts(db *sql.DB) (c []digiCertCertificateRequestSpec, e error) {
	var certspecs []digiCertCertificateRequestSpec
	var (
		id  string
		cn  string
		san string
		request string
		ordernumber string
	)
	stmt, err := db.Prepare("select id, cn, san, request, ordernumber from certs")
	if err != nil {
		return certspecs, err
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		return certspecs, err
	}
	defer rows.Close()
	for rows.Next() {
		var certspec digiCertCertificateRequestSpec
		err := rows.Scan(&id, &cn, &san, &request, &ordernumber)
		if err != nil {
			return nil, err
		}
		certspec.ID = id
		certspec.CN = cn
		certspec.SAN = strings.Split(san, ",")
		certspec.Request = request
		certspec.Order = ordernumber
		certspecs = append(certspecs, certspec)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return certspecs, nil
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

func getKeyFromVault(CN string) (key string) {
	path := os.Getenv("VAULT_CERT_STORAGE") + CN
	key_pem := strings.Replace(strings.Replace(vault.GetField(path, "key"), "\\\\n", "\n", -1), "\\n", "\n", -1)
	return key_pem
}

func getList() (o digiCertOrderList, e error) {
	var orderlist digiCertOrderList
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

func isInArray(wanted string, array []string) bool {
	for _, v := range array {
		if v == wanted {
			return true
		}
	}
	return false
}

func certExists(request digiCertCertificateRequestSpec) (b bool, e error) {
	orderlist, err := getList()
	if err != nil {
		return false, err
	}
	for _, element := range orderlist.Orders {
		if element.Status != "revoked" && element.Status != "expired" && element.Product.Name != "Digital Signature Plus" && element.Product.Name != "Code Signing" {
			if element.Certificate.CommonName == request.CN {
				return true, nil
			}
			for _, dnsname := range element.Certificate.DNSNames {
				if isInArray(dnsname, request.SAN) {
					return true, nil
				}
			}

		}
	}
	return false, nil
}

func addRequestToVault(request digiCertCertificateRequestSpec) (e error) {
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

func generateRequest(request digiCertCertificateRequestSpec) (r digiCertRequest, t string, e error) {
	var csr digiCertRequest
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

func sendRequestToDigicert(csr digiCertRequest, certtype string) (r digiCertCertificateResponse, e error) {
	var response digiCertCertificateResponse
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

func getRequestObject(requestnumber string) (c digiCertCertificateRequestObject, e error) {
	var requestobject digiCertCertificateRequestObject
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

func certificateRequest(request digiCertCertificateRequestSpec, db *sql.DB) (error, *digiCertCertificateRequest) {
	exists, err := certExists(request)
	if err != nil {
		return err, nil
	}
	if exists == true {
		return errors.New("Certificate already exists"), nil
	}

	csr, certtype, err := generateRequest(request)
	if err != nil {
		return err, nil

	}
	response, err := sendRequestToDigicert(csr, certtype)
	if err != nil {
		return err, nil
	}
	request.Request = strconv.Itoa(response.Requests[0].ID)
	requestobject, err := getRequestObject(request.Request)
	if err != nil {
		return err, nil
	}
	request.Order = strconv.Itoa(requestobject.Order.ID)
	newuuid, _ := uuid.NewV4()
	newid := newuuid.String()
	_, err = db.Exec("INSERT INTO certs(id, request, cn, san, ordernumber) VALUES($1,$2,$3,$4,$5)", newid, request.Request, request.CN, strings.Join(request.SAN, ","), request.Order)
	if err != nil {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	return nil, &digiCertCertificateRequest{
		Response: response,
		ID:       newid,
	}
}

type DigiCertIssuer struct {
	db *sql.DB
}

func GetDigiCertIssuer(db *sql.DB) (*DigiCertIssuer, error) {
	return &DigiCertIssuer{
		db:db,
	}, nil
}

func (issuer *DigiCertIssuer) CreateOrder(domain string, sans []string, comment string, requestor string) (id string, err error) {
	err, response := certificateRequest(digiCertCertificateRequestSpec {
		CN:domain,
		SAN:sans,
		Comment:comment,
		Requestedby:comment,
	}, issuer.db)
	if err != nil {
		return "", err
	}
	return response.ID, nil
}

func (issuer *DigiCertIssuer) GetOrderStatus(id string) (*structs.CertificateOrder, error) {
	certspec, err := getCert(issuer.db, id)
	if err != nil {
		return nil, err
	}
	order, err := getOrder(certspec.Order)
	if err != nil {
		return nil, err
	}
	return &structs.CertificateOrder{
		Id: certspec.ID,
		CommonName: certspec.CN,
		SubjectAlternativeNames:certspec.SAN,
		Status: order.Status,
		Issued: order.Certificate.ValidFrom,
		Expires: order.Certificate.ValidTill,
	}, nil
}

func (issuer *DigiCertIssuer) GetOrders() (orders []structs.CertificateOrder, err error) {
	orders = make([]structs.CertificateOrder, 0)
	certspecs, err := getCerts(issuer.db)
	if err != nil {
		return nil, err
	}
	for _, certspec := range certspecs {
		status := "unknown"
		/*
		TODO: See if its necessary to re-request order status on a list...

		order, err := getOrder(certspec.Order)
		if err != nil {
			return nil, err
		}
		if order.Status == "approved" {
			status := "ready"
		} else if order.Status == "rejected" {
			status := "rejected"
		}
		*/
		orders = append(orders, structs.CertificateOrder{
			Id: certspec.ID,
			CommonName: certspec.CN,
			SubjectAlternativeNames:certspec.SAN,
			Status: status,
		});
	}
	return orders, nil
}

func (issuer *DigiCertIssuer) IsOrderAutoInstalled(ingress router.Ingress) (bool, error) {
	return false, nil
}

func (issuer *DigiCertIssuer) IsOrderReady(id string) (bool, error) {
	order, err := issuer.GetOrderStatus(id)
	if err != nil {
		return false, err
	}
	return order.Status == "issued", nil
}

func (issuer *DigiCertIssuer) GetCertificate(id string, domain string) (pem_certs []byte, pem_key []byte, err error) {
	certspec, err := getCert(issuer.db, id)
	if err != nil {
		return nil, nil, err
	}
	order, err := getOrder(certspec.Order)
	if err != nil {
		return nil, nil, err
	}
	key_name := strings.Replace(order.Certificate.CommonName, "*.", "star.", -1)
	pem_key = []byte(getKeyFromVault(key_name))
	pem_certs, err = getCertPEM(strconv.Itoa(order.Certificate.ID))
	if err != nil {
		return nil, nil, err
	}
	return pem_certs, pem_key, nil
}
