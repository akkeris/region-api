package utils

import (
	"database/sql"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
	corev1 "k8s.io/api/core/v1" // todo, get rid of this.
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

	list := []corev1.Pod{}
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
		for _, containerElement := range element.Status.ContainerStatuses {
			if containerElement.State.Running == nil {
				msg.Status = http.StatusInternalServerError
				msg.Message = containerElement.Name + " is not running"
				r.JSON(msg.Status, msg)
				return
			}
		}
	}
	msg.Status = http.StatusOK
	msg.Message = "OK"
	r.JSON(msg.Status, msg)
}

func GetNodes(db *sql.DB, params martini.Params, r render.Render) {
	rts, err := runtime.GetAllRuntimes(db)
	if err != nil {
		ReportError(err, r)
		return
	}
	list := []corev1.Node{}
	for _, rt := range rts {
		nodes, err := rt.GetNodes()
		if err != nil {
			ReportError(err, r)
			return
		}
		list = append(list, nodes.Items...)
	}
	r.JSON(200, corev1.NodeList{Items: list})
}
