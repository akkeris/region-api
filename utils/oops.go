package utils

import (
	"github.com/martini-contrib/render"
	"log"
	structs "region-api/structs"
	debug "runtime/debug"
)

func ReportInvalidRequest(err string, r render.Render) {
	log.Println(err)
	var message structs.Messagespec
	message.Status = 400
	message.Message = err
	r.JSON(400, message)
}

func LogError(prefix string, err error) {
	if prefix != "" {
		log.Println("Error: " + prefix)
	}
	log.Println(err)
	debug.PrintStack()
}

func ReportError(err error, r render.Render) {
	LogError("", err)
	var message structs.Messagespec
	message.Status = 500
	message.Message = err.Error()
	r.JSON(500, message)
}
