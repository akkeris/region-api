package utils

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq" //driver
)

//GetDB centralized access point
func GetDB(uri string) *sql.DB {
	db, dberr := sql.Open("postgres", uri)
	if dberr != nil {
		fmt.Println(dberr)
		return nil
	}
  // not available in 1.5 golang, youll want to turn it on for v1.6 or higher once upgraded.
  //pool.SetConnMaxLifetime(time.ParseDuration("1h"));
  db.SetMaxIdleConns(4);
  db.SetMaxOpenConns(20);
	return db
}
