package maintenance

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	router "region-api/router"
	space "region-api/space"
	structs "region-api/structs"
	utils "region-api/utils"
)

func EnableMaintenancePage(db *sql.DB, params martini.Params, r render.Render) {
	internal, err := space.IsInternalSpace(db, params["space"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	ingress, err := router.GetAppIngress(db, internal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if err = ingress.SetMaintenancePage(params["app"], params["space"], true); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status:http.StatusCreated, Message:"Maintenance Page Enabled"})
}

func DisableMaintenancePage(db *sql.DB, params martini.Params, r render.Render) {
	internal, err := space.IsInternalSpace(db, params["space"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	ingress, err := router.GetAppIngress(db, internal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if err = ingress.SetMaintenancePage(params["app"], params["space"], false); err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Messagespec{Status:http.StatusOK, Message:"Maintenance Page Disabled"})
}

func MaintenancePageStatus(db *sql.DB, params martini.Params, r render.Render) {
	internal, err := space.IsInternalSpace(db, params["space"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	ingress, err := router.GetAppIngress(db, internal)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	enabled, err := ingress.GetMaintenancePageStatus(params["app"], params["space"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	status := "off"
	if enabled {
		status = "on"
	}
	r.JSON(http.StatusOK, structs.Maintenancespec{App:params["app"], Space:params["space"], Status:status})
}
