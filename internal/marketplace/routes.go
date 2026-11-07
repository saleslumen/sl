package marketplace

import "net/http"

type route struct {
	Method string
	Path   string
}

var cliRoutes = [...]route{
	{http.MethodGet, "/v1/apps"},
	{http.MethodPost, "/v1/apps"},
	{http.MethodGet, "/v1/apps/{app_id}"},
	{http.MethodPatch, "/v1/apps/{app_id}"},
	{http.MethodGet, "/v1/apps/{app_id}/operations"},
	{http.MethodPost, "/v1/apps/{app_id}:rollback"},
	{http.MethodPost, "/v1/apps/{app_id}:suspend"},
	{http.MethodPost, "/v1/apps/{app_id}:unpublish"},
	{http.MethodGet, "/v1/publisherApps"},
	{http.MethodGet, "/v1/publisherApps/{app_id}"},
	{http.MethodPost, "/v1/apps/{app_id}/listings"},
	{http.MethodGet, "/v1/apps/{app_id}/listings"},
	{http.MethodGet, "/v1/apps/{app_id}/listings/{listing_id}"},
	{http.MethodGet, "/v1/apps/{app_id}/listings/{listing_id}/revisions/{revision_id}"},
	{http.MethodPatch, "/v1/apps/{app_id}/listings/{listing_id}/revisions/{revision_id}"},
	{http.MethodPut, "/v1/apps/{app_id}/connectionOAuthClients/{connection_key}"},
	{http.MethodGet, "/v1/apps/{app_id}/connectionOAuthClients/{connection_key}"},
	{http.MethodDelete, "/v1/apps/{app_id}/connectionOAuthClients/{connection_key}"},
	{http.MethodPost, "/v1/apps/{app_id}/releases"},
	{http.MethodGet, "/v1/apps/{app_id}/releases"},
	{http.MethodGet, "/v1/apps/{app_id}/releases/{release_id}"},
	{http.MethodPost, "/v1/apps/{app_id}/releases/{release_id}:revoke"},
	{http.MethodPost, "/v1/apps/{app_id}/submissions"},
	{http.MethodGet, "/v1/apps/{app_id}/submissions"},
	{http.MethodGet, "/v1/apps/{app_id}/submissions/{submission_id}"},
	{http.MethodPost, "/v1/apps/{app_id}/submissions/{submission_id}:withdraw"},
	{http.MethodPost, "/v1/apps/{app_id}/submissions/{submission_id}:approve"},
	{http.MethodPost, "/v1/apps/{app_id}/submissions/{submission_id}:requestChanges"},
	{http.MethodPost, "/v1/apps/{app_id}/submissions/{submission_id}:reject"},
	{http.MethodGet, "/v1/reviewQueue"},
	{http.MethodGet, "/v1/reviewQueue/submissions/{submission_id}"},
	{http.MethodPost, "/v1/installations"},
	{http.MethodGet, "/v1/installations"},
	{http.MethodGet, "/v1/installations/{installation_id}"},
	{http.MethodPatch, "/v1/installations/{installation_id}"},
	{http.MethodDelete, "/v1/installations/{installation_id}"},
	{http.MethodPost, "/v1/installations/{installation_id}:approveUpdate"},
	{http.MethodGet, "/v1/installations/{installation_id}/updateConsent"},
	{http.MethodGet, "/v1/installations/{installation_id}/operations"},
	{http.MethodGet, "/v1/installations/{installation_id}/configuration"},
	{http.MethodPatch, "/v1/installations/{installation_id}/configuration"},
	{http.MethodGet, "/v1/installations/{installation_id}/connections"},
	{http.MethodGet, "/v1/installations/{installation_id}/connections/{connection_key}"},
	{http.MethodPost, "/v1/installations/{installation_id}/connections/{connection_key}:initiateAuthorization"},
	{http.MethodPost, "/v1/installations/{installation_id}/connections/{connection_key}:unbind"},
	{http.MethodPost, "/v1/installations/{installation_id}/connections/{connection_key}:initiateApiKey"},
	{http.MethodPost, "/v1/installationConnectionApiKeyOperations:complete"},
	{http.MethodGet, "/v1/organizations/{organization_id}/marketplacePolicy"},
	{http.MethodPatch, "/v1/organizations/{organization_id}/marketplacePolicy"},
	{http.MethodGet, "/v1/organizations/{organization_id}/installCapabilities"},
}

var scriptBindRoute = route{Method: http.MethodPost, Path: "/v1/installedConnectionApiKeyCredentials:bind"}
