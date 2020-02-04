package app

import (
	"gopkg.in/guregu/null.v3/zero"
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	utils "region-api/utils"
)


type resourceSpec struct {
	Requests struct {
		Memory string `json:"memory,omitempty"`
		CPU    string `json:"cpu,omitempty"`
	} `json:"requests"`
	Limits struct {
		Memory string `json:"memory,omitempty"`
		CPU    string `json:"cpu,omitempty"`
	} `json:"limits"`
}

type qos struct {
	Name        string       `json:"name"`
	Resources   resourceSpec `json:"resources"`
	Price       int          `json:"price"`
	Description zero.String  `json:"description"`
	Deprecated  bool         `json:"deprecated"`
	Type        zero.String  `json:"type"`
}

func GetPlans(db *sql.DB, params martini.Params, r render.Render) {
	var plan qos

	rows, err := db.Query("select name, memrequest, memlimit, price, deprecated, description, type from plans")
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	defer rows.Close()
	var planlist []interface{}
	for rows.Next() {
		err := rows.Scan(&plan.Name, &plan.Resources.Requests.Memory, &plan.Resources.Limits.Memory, &plan.Price, &plan.Deprecated, &plan.Description, &plan.Type)
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
