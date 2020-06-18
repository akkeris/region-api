package server

import (
	"region-api/app"
	"region-api/space"
	"region-api/structs"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
)

func initV2Endpoints(m *martini.ClassicMartini) {

	// app* endpoints affect resources on ALL deployments for an Akkeris app
	// space* endpoints affect individual deployments in a namespace

	// Get a list of all apps
	// Code done, need to write tests
	m.Get("/v2beta1/apps", app.ListAppsV2)

	// Get a list of all deployments for an app
	// Code done, need to write tests
	m.Get("/v2beta1/app/:appid", app.DescribeAppV2)

	// List all deployments in a space
	// Code done, need to write tests
	m.Get("/v2beta1/space/:space/deployments", space.DescribeSpaceV2)

	// Get info on a deployment
	// Code done, need to write tests
	m.Get("/v2beta1/space/:space/deployment/:deployment", space.DescribeDeploymentV2)

	// Get configvars for a deployment
	// Code done, need to write tests
	m.Get("/v2beta1/space/:space/deployment/:deployment/configvars", space.GetAllConfigVarsV2)

	// TODO: Combine space.AddDeploymentV2 and space.DeploymentV2 (if deployment DNE in db, create at deploy time)
	// Create a new deployment in the DB
	// Oneoff exists here as well (parameter in body)
	// Code done, need to write tests
	m.Post("/v2beta1/space/:space/deployment/:deployment", binding.Json(structs.AppDeploymentSpec{}), space.AddDeploymentV2)

	// Redeploy (modify) an existing deployment
	// Code done, need to write tests
	m.Put("/v2beta1/space/:space/deployment/:deployment/deploy", binding.Json(structs.DeploySpecV2{}), space.DeploymentV2Handler)

	// Remove a deployment
	// Code done, need to write tests
	m.Delete("/v2beta1/space/:space/deployment/:deployment", space.DeleteDeploymentV2Handler)

	// Modify deployment healthcheck
	// Code done, need to write tests
	m.Put("/v2beta1/space/:space/deployment/:deployment/healthcheck", binding.Json(structs.AppDeploymentSpec{}), space.UpdateDeploymentHealthCheckV2)
	m.Delete("/v2beta1/space/:space/deployment/:deployment/healthcheck", space.DeleteDeploymentHealthCheckV2)

	// Update deployment plan
	// Code done, need to write tests
	m.Put("/v2beta1/space/:space/deployment/:deployment/plan", binding.Json(structs.AppDeploymentSpec{}), space.UpdateDeploymentPlanV2)

	// Scale deployment
	// Code done, need to write tests
	m.Put("/v2beta1/space/:space/deployment/:deployment/scale", binding.Json(structs.AppDeploymentSpec{}), space.ScaleDeploymentV2)

	// Remove all deployments for an app
	// Code done, need to write tests
	m.Delete("/v2beta1/app/:appid", app.DeleteAppV2)

	// TODOs below

	// TODO: Implement
	// Delete a space
	m.Delete("/v2beta1/space/:space", space.DeleteSpaceV2)

	// TODO: Implement
	// Rename all deployments for an app
	m.Put("/v2beta1/app/:appid/rename", app.RenameAppV2)

	// TODO: Take Deployment functions out of the space package. Create "deployment" package maybe?
}
