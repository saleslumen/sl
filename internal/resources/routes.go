package resources

import "net/http"

type route struct {
	Method string
	Path   string
}

var cliRoutes = [...]route{
	{http.MethodGet, "/v1/organizations/{organization_id}"},
	{http.MethodGet, "/v1/organizations/{organization_id}/namespaces"},
	{http.MethodPost, "/v1/organizations/{organization_id}/namespaces"},
	{http.MethodDelete, "/v1/organizations/{organization_id}/namespaces/{namespace_id}"},
}
