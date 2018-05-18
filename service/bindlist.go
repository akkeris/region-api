package service

import (
	structs "../structs"
	utils "../utils"
	"database/sql"
	"fmt"

	"github.com/go-martini/martini"
	_ "github.com/lib/pq"
	"github.com/martini-contrib/render"
)

type BindList struct {
	List []structs.Bindspec `json:"list"`
}

func GetBindingList(db *sql.DB, params martini.Params, r render.Render) {

	list, err := getbindinglist(db, params["service"])
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, list)
}
func getbindinglist(db *sql.DB, bt string) (bl BindList, e error) {
	var bindlist BindList
	var (
		appname  string
		space    string
		bindtype string
		bindname string
	)
	stmt, dberr := db.Prepare("select appname, space, bindtype, bindname from appbindings where bindtype = $1")
	if dberr != nil {
		fmt.Println(dberr)
		return bindlist, dberr
	}
	defer stmt.Close()
	rows, err := stmt.Query(bt)

	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&appname, &space, &bindtype, &bindname)
		if err != nil {
			fmt.Println(err)
			return bindlist, err
		}
		var b structs.Bindspec
		b.App = appname
		b.Space = space
		b.Bindtype = bindtype
		b.Bindname = bindname
		bindlist.List = append(bindlist.List, b)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return bindlist, err
	}

	return bindlist, nil
}
