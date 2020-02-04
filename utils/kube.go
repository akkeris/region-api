package utils

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	runtime "region-api/runtime"
	structs "region-api/structs"
)

// These are raw kubernetes interfaces for
// additional information needed by other apis
// or for debug/maintenance reasons, these should
// not be used for adding or removing resources
// just inspecting what actual was created behind
// the scenes.

func Octhc(db *sql.DB, params martini.Params, r render.Render) {
	rt, err := runtime.GetRuntimeFor(db, "default")
	if err != nil {
		ReportError(err, r)
		return
	}
	service, err := rt.GetService("default", "oct-apitest")
	if err != nil {
		ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, service)
}

func GetService(db *sql.DB, params martini.Params, r render.Render) {
	space := params["space"]
	app := params["app"]

	rt, err := runtime.GetRuntimeFor(db, space)
	if err != nil {
		ReportError(err, r)
		return
	}
	service, err := rt.GetService(space, app)
	if err != nil {
		ReportError(err, r)
		return
	}
	r.JSON(http.StatusOK, service)
}

func GetKubeSystemPods(db *sql.DB, params martini.Params, r render.Render) {
	rts, err := runtime.GetAllRuntimes(db)
	if err != nil {
		ReportError(err, r)
		return
	}

	list := []runtime.PodStatusItems{}
	for _, rt := range rts {
		podStatus, err := rt.GetPodsBySpace("kube-system")
		if err != nil {
			ReportError(err, r)
			return
		}
		list = append(list, podStatus.Items...)
	}

	var msg structs.Messagespec
	for _, element := range list {
		for _, containerelement := range element.Status.ContainerStatuses {
			state := containerelement.State
			var keys []string
			for k := range state {
				keys = append(keys, k)
			}
			containerstate := keys[0]
			if containerstate != "running" {
				msg.Status = 500
				msg.Message = containerelement.Name + " is not running"
				r.JSON(msg.Status, msg)
				return
			}
		}
	}
	msg.Status = 200
	msg.Message = "OK"
	r.JSON(msg.Status, msg)
}

func GetNodes(db *sql.DB, params martini.Params, r render.Render) {
	rts, err := runtime.GetAllRuntimes(db)
	if err != nil {
		ReportError(err, r)
		return
	}
	list := []structs.KubeNodeItems{}
	for _, rt := range rts {
		nodes, err := rt.GetNodes()
		if err != nil {
			ReportError(err, r)
			return
		}
		list = append(list, nodes.Items...)
	}

	r.JSON(200, structs.KubeNodes{Kind: "NodeList", APIVersion: "v1", Items: list})
}
