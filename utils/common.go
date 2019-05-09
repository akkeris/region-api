package utils

import (
	"region-api/structs"
	"database/sql"
)

func getSpace(db *sql.DB, space string) (s structs.Spacespec, e error) {
	var spaceobject structs.Spacespec
	var internal bool
	var stack string
	var compliancetags string
	err := db.QueryRow("select internal, COALESCE(compliancetags, '') as compliancetags, stack from spaces where name = $1", space).Scan(&internal, &compliancetags, &stack)
	if err != nil {
		return spaceobject, err
	}
	spaceobject.Name = space
	spaceobject.Internal = internal
	spaceobject.ComplianceTags = compliancetags
	spaceobject.Stack = stack
	return spaceobject, nil
}

func IsInternalSpace(db *sql.DB, space string) (i bool, e error) {
	spaceobj, err := getSpace(db, space)
	if err != nil {
		LogError("", err)
		return true, err
	}
	return spaceobj.Internal, nil
}