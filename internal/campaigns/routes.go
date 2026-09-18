package campaigns

import "net/http"

type route struct {
	Method string
	Path   string
}

// cliRoutes is the closed CLI HTTP surface. It is never mutated.
var cliRoutes = [...]route{
	{http.MethodGet, "/v1/campaigns"},
	{http.MethodPost, "/v1/campaigns"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}"},
	{http.MethodPatch, "/v1/campaigns/{campaign_id}"},
	{http.MethodDelete, "/v1/campaigns/{campaign_id}"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:activate"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:pause"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:resume"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:complete"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:archive"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:unarchive"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:setVariables"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}:setSenderAccounts"},
	{http.MethodGet, "/v1/campaignSchedules"},
	{http.MethodPost, "/v1/campaignSchedules"},
	{http.MethodGet, "/v1/campaignSchedules/{schedule_id}"},
	{http.MethodPatch, "/v1/campaignSchedules/{schedule_id}"},
	{http.MethodDelete, "/v1/campaignSchedules/{schedule_id}"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/people"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/people/{person_id}"},
	{http.MethodPatch, "/v1/campaigns/{campaign_id}/people/{person_id}"},
	{http.MethodDelete, "/v1/campaigns/{campaign_id}/people/{person_id}"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people:import"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people:batchDelete"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people:runScript"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people/{person_id}:pause"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people/{person_id}:resume"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/people/{person_id}:unsubscribe"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/people/{person_id}:activity"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/sequences"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/sequences"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/sequences/{sequence_id}"},
	{http.MethodPatch, "/v1/campaigns/{campaign_id}/sequences/{sequence_id}"},
	{http.MethodDelete, "/v1/campaigns/{campaign_id}/sequences/{sequence_id}"},
	{http.MethodPost, "/v1/campaigns/{campaign_id}/sequences/{sequence_id}:preview"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/deliveries"},
	{http.MethodGet, "/v1/deliveries/{delivery_id}"},
	{http.MethodPost, "/v1/deliveries/{delivery_id}:resolveUnknown"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/metrics"},
	{http.MethodGet, "/v1/suppressions"},
	{http.MethodPost, "/v1/suppressions"},
	{http.MethodDelete, "/v1/suppressions/{suppression_id}"},
	{http.MethodGet, "/v1/operations/{operation_id}"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/tasks/{task_id}"},
	{http.MethodGet, "/v1/campaigns/{campaign_id}/tasks/{task_id}:stream"},
}
