package main

import (
	"database/sql"
	"fmt"
	"github.com/stackimpact/stackimpact-go"
	"os"
	"region-api/server"
	"region-api/utils"
	"time"
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
			AppName:  "Alamo API",
		})
	}
	utils.InitAuth()
	pitdb := os.Getenv("PITDB")
	pool := utils.GetDB(pitdb)
	server.Init(pool)
}
