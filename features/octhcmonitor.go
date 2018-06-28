package features

import (
	"database/sql"
	"fmt"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func GetOcthcOption(db *sql.DB, params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	optionvalue, err := getOcthcOption(db, app, space)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}

	//r.JSON(200, strconv.FormatBool(optionvalue ))
	r.JSON(200, map[string]interface{}{"option": optionvalue})
}

func getOcthcOption(db *sql.DB, app string, space string) (o bool, e error) {
	fmt.Println("space " + space)
	fmt.Println("app " + app)
	stmt, err := db.Prepare("select optionvalue as toreturn from appfeature where space=$1 and app=$2 and optionkey=$3")
	if err != nil {
		var msg structs.Messagespec
		fmt.Println(err)
		msg.Status = 500
		msg.Message = err.Error()
		return false, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(space, app, "octhc")
	defer rows.Close()
	var toreturn bool
	feature_default_octhc, _ := strconv.ParseBool(os.Getenv("FEATURE_DEFAULT_OCTHC"))
	toreturn = feature_default_octhc
	for rows.Next() {
		fmt.Println("LOOOOOOOOOP")
		err := rows.Scan(&toreturn)
		fmt.Println(toreturn)
		if err != nil {
			fmt.Println(err)
			return false, err
		}
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	return toreturn, nil

}

func UpdateOcthcOption(db *sql.DB, params martini.Params, r render.Render) {
	var msg structs.Messagespec
	app := params["app"]
	space := params["space"]
	optionvalue := params["optionvalue"]
	optionvaluebool, err := strconv.ParseBool(optionvalue)
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = err.Error()
		r.JSON(msg.Status, msg)
		return
	}

	msg, _ = updateOcthcOption(db, app, space, optionvaluebool)
	r.JSON(msg.Status, msg)
}

func updateOcthcOption(db *sql.DB, app string, space string, optionvalue bool) (m structs.Messagespec, e error) {
	var msg structs.Messagespec

	fmt.Println(app)
	fmt.Println(space)
	fmt.Println(optionvalue)

	err := db.QueryRow("INSERT into appfeature (space,app,optionkey,optionvalue) values ($1,$2,$3,$4)  ON CONFLICT ON CONSTRAINT appfeature_pkey DO UPDATE set optionvalue=$4 returning optionvalue;", space, app, "octhc", optionvalue).Scan(&optionvalue)
	if err != nil {
		fmt.Println(err)
		msg.Status = 500
		msg.Message = "Error while updating"
		return msg, err
	}
	msg.Status = 201
	msg.Message = "Option updated"
	return msg, nil
}
