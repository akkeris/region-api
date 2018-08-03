package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
)

//Listapps centralized
func Listapps(db *sql.DB, params martini.Params, r render.Render) {
	var name string
	rows, err := db.Query("select name from apps")
	defer rows.Close()
	var applist []string
	for rows.Next() {
		err := rows.Scan(&name)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		applist = append(applist, name)
	}
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, structs.Applist{Apps: applist})
}
