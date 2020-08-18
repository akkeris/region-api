package utils

import (
	"log"
	structs "region-api/structs"
	debug "runtime/debug"

	"github.com/martini-contrib/render"
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

func ReportNotFoundError(r render.Render) {
	var message structs.Messagespec
	message.Status = 404
	message.Message = "Not Found"
	r.JSON(404, message)
}
