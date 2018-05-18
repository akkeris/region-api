package app

import (
	structs "../structs"
	utils "../utils"
	"database/sql"
	"net/http"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nu7hatch/gouuid"
)

//Createapp centralized
func Createapp(db *sql.DB, spec structs.Appspec, berr binding.Errors, r render.Render) {
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

	newappiduuid, _ := uuid.NewV4()
	newappid := newappiduuid.String()

	var appid string
	inserterr := db.QueryRow("INSERT INTO apps(appid,name,port) VALUES($1,$2,$3) returning appid;", newappid, spec.Name, spec.Port).Scan(&appid)
	if inserterr != nil {
		// dont report this error, its fairly typical and sort of a
		// throw back to some design decisions we made early on, in reality this 
		// end point should probably just be removed.
		r.JSON(http.StatusInternalServerError, structs.Messagespec{Status:http.StatusInternalServerError, Message:inserterr.Error()})
		return
	}
	r.JSON(http.StatusCreated, structs.Messagespec{Status:http.StatusCreated, Message:"App Created with ID " + appid})
}
