package certs

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	"region-api/router"
	"region-api/structs"
	"region-api/utils"
	"os"
)

func HttpCreateCertificateOrder(db *sql.DB, request CertificateOrder, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	issuer, err := GetIssuer(db, "cert-manager") // we only support cert-manager as an issuer system at the moment.
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if request.Issuer == "" && os.Getenv("DEFAULT_ISSUER") != "" {
		request.Issuer = os.Getenv("DEFAULT_ISSUER")
	} else if request.Issuer == "" && os.Getenv("DEFAULT_ISSUER") == "" {
		request.Issuer = "letsencrypt"
	}
	id, err := issuer.CreateOrder(request.CommonName, request.SubjectAlternativeNames, request.Comment, request.Requestor, request.Issuer)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	request.Id = id
	r.JSON(http.StatusCreated, request)
}

func HttpGetCertificateOrderStatus(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db, "cert-manager") // we only support cert-manager as an issuer system at the moment.
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	order, err := issuer.GetOrderStatus(params["id"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, order)
}

func HttpGetCertificateOrders(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db, "cert-manager") // we only support cert-manager as an issuer system at the moment.
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	orders, err := issuer.GetOrders()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, orders)
}

func HttpInstallCertificate(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db, "cert-manager") // we only support cert-manager as an issuer system at the moment.
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	ready, err := issuer.IsOrderReady(params["id"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if !ready {
		r.JSON(http.StatusUnprocessableEntity, structs.Messagespec{Status: http.StatusUnprocessableEntity, Message: "The certificate is not yet ready to be installed, it may still need to be approved or issued."})
		return
	}
	order, err := issuer.GetOrderStatus(params["id"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	private, err := router.GetSiteIngress(db, false)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	public, err := router.GetSiteIngress(db, true)
	if err != nil {
		utils.ReportError(err, r)
		return
	}

	// Get Certificate
	pem_cert, pem_key, err := issuer.GetCertificate(params["id"], order.CommonName)
	auto, err := issuer.IsOrderAutoInstalled(private)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if !auto {
		err = private.InstallCertificate(order.CommonName, pem_cert, pem_key)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	}
	auto, err = issuer.IsOrderAutoInstalled(public)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if !auto {
		err = public.InstallCertificate(order.CommonName, pem_cert, []byte(pem_key))
		if err != nil {
			utils.ReportError(err, r)
			return
		}
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status: http.StatusOK, Message: "Certificate Installed"})
}

func AddToMartini(m *martini.ClassicMartini) {
	m.Post("/v1/certs", binding.Json(CertificateOrder{}), HttpCreateCertificateOrder)
	m.Get("/v1/certs", HttpGetCertificateOrders)
	m.Get("/v1/certs/:id", HttpGetCertificateOrderStatus)
	m.Post("/v1/certs/:id/install", HttpInstallCertificate)
}

