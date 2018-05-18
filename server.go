package main

import (
	"./server"
	"./utils"
	"database/sql"
	"fmt"
	"os"
	"time"
	"github.com/stackimpact/stackimpact-go"
)

func PrintDBStats(db *sql.DB) func() {
	return func() {
		fmt.Println("Open connections: %i", db.Stats().OpenConnections)
		time.AfterFunc(30*time.Second, PrintDBStats(db))
	}
}



func main() {
	if os.Getenv("STACKIMPACT") != "" {
		stackimpact.Start(stackimpact.Options{
			AgentKey: os.Getenv("STACKIMPACT"),
	  		AppName: "Alamo API",
		})
	}
	utils.InitAuth()
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	server.Init(pool)
}
