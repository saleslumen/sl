package script

import "net/http"

type route struct {
	Method string
	Path   string
}

// cliRoutes is the closed CLI HTTP surface. It is never mutated.
var cliRoutes = [...]route{
	{http.MethodPost, "/v1/projects"},
	{http.MethodGet, "/v1/projects/{script_id}"},
	{http.MethodPut, "/v1/projects/{script_id}"},
	{http.MethodDelete, "/v1/projects/{script_id}"},
	{http.MethodGet, "/v1/projects"},
	{http.MethodGet, "/v1/projects/{script_id}/content"},
	{http.MethodPut, "/v1/projects/{script_id}/content"},
	{http.MethodGet, "/v1/projects/{script_id}/metrics"},
	{http.MethodGet, "/v1/projects/{script_id}/versions/{version_number}"},
	{http.MethodGet, "/v1/projects/{script_id}/versions"},
	{http.MethodGet, "/v1/projects/{script_id}/versions/{version_number}/content"},
	{http.MethodGet, "/v1/projects/{script_id}/versions:compare"},
	{http.MethodPost, "/v1/projects/{script_id}/versions/{version_number}:restore"},
	{http.MethodPost, "/v1/projects/{script_id}/deployments"},
	{http.MethodGet, "/v1/projects/{script_id}/deployments/{deployment_id}"},
	{http.MethodPut, "/v1/projects/{script_id}/deployments/{deployment_id}"},
	{http.MethodDelete, "/v1/projects/{script_id}/deployments/{deployment_id}"},
	{http.MethodGet, "/v1/projects/{script_id}/deployments"},
	{http.MethodGet, "/v1/processes"},
	{http.MethodGet, "/v1/processes:listScriptProcesses"},
	{http.MethodGet, "/v1/scripts/{script_id}/contracts"},
	{http.MethodGet, "/v1/libraries/{library_id}:lookup"},
	{http.MethodGet, "/v1/libraries/{library_id}/versions"},
	{http.MethodPost, "/v1/libraries:validateDependencyGraph"},
	{http.MethodGet, "/v1/libraries/{library_id}/versions/{version_number}/reference"},
}
