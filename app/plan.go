package app

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	structs "region-api/structs"
	utils "region-api/utils"
)

func GetPlans(db *sql.DB, params martini.Params, r render.Render) {
	var plan structs.QoS

	rows, err := db.Query("select name, memrequest, memlimit, price from plans")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	var planlist []interface{}
	for rows.Next() {
		err := rows.Scan(&plan.Name, &plan.Resources.Requests.Memory, &plan.Resources.Limits.Memory, &plan.Price)
		if err != nil {
			utils.ReportError(err, r)
			return
		}
		planlist = append(planlist, plan)
	}
	err = rows.Err()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, planlist)
}
