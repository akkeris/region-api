package server

import (
	"region-api/deployment"
	"region-api/structs"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
)

func initV2Endpoints(m *martini.ClassicMartini) {

	// app* endpoints affect resources on ALL deployments for an Akkeris app
	// space* endpoints affect individual deployments in a namespace

	// Get a list of all apps
	m.Get("/v2beta1/apps", deployment.ListAppsV2) // Probably should go in the apps package?

	// Get a list of all deployments for an app
	m.Get("/v2beta1/app/:appid", deployment.DescribeAppV2) // Probably should go in the apps package?

	// List all deployments in a space
	m.Get("/v2beta1/space/:space/deployments", deployment.DescribeSpaceV2)

	// Get info on a deployment
	m.Get("/v2beta1/space/:space/deployment/:deployment", deployment.DescribeDeploymentV2)

	// Get configvars for a deployment
	m.Get("/v2beta1/space/:space/deployment/:deployment/configvars", deployment.GetAllConfigVarsV2)

	// Create a new deployment in the DB
	// Oneoff exists here as well (parameter in body)
	m.Post("/v2beta1/space/:space/deployment/:deployment", binding.Json(structs.AppDeploymentSpec{}), deployment.AddDeploymentV2)

	// Redeploy (modify) an existing deployment
	m.Put("/v2beta1/space/:space/deployment/:deployment/deploy", binding.Json(structs.DeploySpecV2{}), deployment.DeploymentV2Handler)

	// Remove a deployment
	m.Delete("/v2beta1/space/:space/deployment/:deployment", deployment.DeleteDeploymentV2Handler)

	// Modify deployment healthcheck
	m.Patch("/v2beta1/space/:space/deployment/:deployment/healthcheck", binding.Json(structs.AppDeploymentSpec{}), deployment.UpdateDeploymentHealthCheckV2)
	m.Delete("/v2beta1/space/:space/deployment/:deployment/healthcheck", deployment.DeleteDeploymentHealthCheckV2)

	// Update deployment plan
	m.Patch("/v2beta1/space/:space/deployment/:deployment/plan", binding.Json(structs.AppDeploymentSpec{}), deployment.UpdateDeploymentPlanV2)

	// Scale deployment
	m.Patch("/v2beta1/space/:space/deployment/:deployment/scale", binding.Json(structs.AppDeploymentSpec{}), deployment.ScaleDeploymentV2)

	// Remove all deployments for an app
	m.Delete("/v2beta1/app/:appid", deployment.DeleteAppV2)

	// Delete a space
	m.Delete("/v2beta1/space/:space", deployment.DeleteSpaceV2) // Probably should go in the space package?

	// TODOs

	// TODO: Implement
	// Rename all deployments for an app
	m.Put("/v2beta1/app/:appid/rename", deployment.RenameAppV2)

	// TODO: Investigate combining deployment.AddDeploymentV2 and
	// deployment.DeploymentV2 (if deployment DNE in db, create at deploy time)

	// TODO: Write tests for v2 one off deployments
}
