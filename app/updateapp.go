package app

import (
	"database/sql"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
)

//Updateapp centralized
func Updateapp(db *sql.DB, spec structs.Appspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	if spec.Name == "" {
		utils.ReportInvalidRequest("Name Cannot be blank", r)
		return
	}
	if spec.Port == 0 {
		utils.ReportInvalidRequest("Port Cannot be blank", r)
		return
	}

	var name string
	inserterr := db.QueryRow("UPDATE apps set port = $1 where name = $2 returning name;", spec.Port, spec.Name).Scan(&name)
	if inserterr != nil {
		utils.ReportError(inserterr, r)
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status: http.StatusCreated, Message: "App " + name + "Updated"})
}
