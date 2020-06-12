package server

import (
	"region-api/app"
	"region-api/space"
	"region-api/structs"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
)

func initV2Endpoints(m *martini.ClassicMartini) {

	/****************************
	*
	*	 Modified from V1 Schema
	*
	****************************/

	// Get details about all of the deployments for an app
	m.Get("/v2beta1/app/:appid", app.DescribeAppV2)

	// Get details about a specific deployment for an app
	m.Get("/v2beta1/space/:space/app/:appname", space.DescribeDeploymentV2)

	// Get a list of all app names
	m.Get("/v2beta1/apps", app.ListAppsV2)

	// Get a list of all apps in a space
	m.Get("/v2beta1/space/:space/apps", space.DescribeSpaceV2)

	// Get all config vars for an app
	m.Get("/v2beta1/space/:space/app/:appname/configvars", app.GetAllConfigVarsV2)

	// Add and remove apps
	m.Put("/v2beta1/space/:space/app/:app", binding.Json(structs.AppDeploymentSpec{}), space.AddAppV2)
	m.Delete("/v2beta1/space/:space/app/:app", space.DeleteDeploymentV2)

	// Update app details
	m.Put("/v2beta1/space/:space/app/:app/healthcheck", binding.Json(structs.Spaceappspec{}), space.UpdateAppHealthCheckV2)
	m.Delete("/v2beta1/space/:space/app/:app/healthcheck", space.DeleteAppHealthCheckV2)
	m.Put("/v2beta1/space/:space/app/:app/plan", binding.Json(structs.Spaceappspec{}), space.UpdateAppPlanV2)
	m.Put("/v2beta1/space/:space/app/:app/scale", binding.Json(structs.Spaceappspec{}), space.ScaleAppV2)

	m.Put("/v2beta1/app/:appid/name", binding.Json(structs.AppRenameSpec{}), app.RenameAppV2)

	// Delete a space
	m.Delete("/v2beta1/space/:space", binding.Json(structs.Spacespec{}), space.DeleteSpaceV2)

	// Todo

	// "from apps,spacesapps"
	m.Delete("/v2beta1/app/:appid", app.DeleteAppV2)
	m.Post("/v2beta1/app/deploy", binding.Json(structs.Deployspec{}), app.DeploymentV2)
	m.Post("/v2beta1/app/deploy/oneoff", binding.Json(structs.OneOffSpec{}), app.OneOffDeploymentV2)

}
