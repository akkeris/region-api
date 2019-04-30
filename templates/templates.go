package templates

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
)

func GetURLTemplates(db *sql.DB, params martini.Params, r render.Render) {
	urltemplates, err := getURLTemplates()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, urltemplates)
}

func getURLTemplates() (u structs.URLTemplates, e error) {
	var urltemplates structs.URLTemplates
	urltemplates.Internal = os.Getenv("ALAMO_INTERNAL_URL_TEMPLATE")
	urltemplates.External = os.Getenv("ALAMO_URL_TEMPLATE")
	if urltemplates.Internal == "" {
		fmt.Println("No internal url template")
		return urltemplates, errors.New("No Inernal URL Template")
	}
	if urltemplates.External == "" {
		fmt.Println("No external url template")
		return urltemplates, errors.New("No External URL Template")
	}
	return urltemplates, nil
}


