package utils

import (
	"os"
)

var AuthUser string
var AuthPassword string

func InitAuth() {

	AuthUser = os.Getenv("ALAMO_API_AUTH_USERNAME")
	AuthPassword =  os.Getenv("ALAMO_API_AUTH_PASSWORD")

}
