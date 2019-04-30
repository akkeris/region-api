package certs

import (
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"region-api/utils"
	"region-api/structs"
	"region-api/router"
	"database/sql"
	"net/http"
)

func CreateCertificateOrder(db *sql.DB, request structs.CertificateOrder, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	issuer, err := GetIssuer(db)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	id, err := issuer.CreateOrder(request.CommonName, request.SubjectAlternativeNames, request.Comment, request.Requestor)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	request.Id = id
	r.JSON(http.StatusCreated, request)
}

func GetCertificateOrderStatus(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db)
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

func GetCertificateOrders(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db)
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

func InstallCertificate(db *sql.DB, params martini.Params, r render.Render) {
	issuer, err := GetIssuer(db)
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
		r.JSON(http.StatusUnprocessableEntity, structs.Messagespec{Status:http.StatusUnprocessableEntity, Message:"The certificate is not yet ready to be installed, it may still need to be approved or issued."})
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
	auto, err = issuer.IsOrderAutoInstalled(private)
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
	r.JSON(http.StatusOK, structs.Messagespec{Status:http.StatusOK, Message:"Certificate Installed"})
}


