package workflows

type artifactRoute struct {
	Method  string
	Path    string
	RPC     string
	Covered bool
	Body    string
	Reason  string
}

// routes is the closed Workflows HTTP surface from the generated OpenAPI artifact.
// Covered routes are CLI commands; uncovered routes carry the approved exclusion reason.
var routes = []artifactRoute{
	{Method: "POST", Path: "/v1/hooks/{token}", RPC: "InvokeWebhook", Body: "body", Reason: "anonymous webhook ingress"},
	{Method: "GET", Path: "/v1/triggerEventTypes", RPC: "ListSupportedTriggerEventTypes", Covered: true},
	{Method: "GET", Path: "/v1/workflows", RPC: "ListWorkflows", Covered: true},
	{Method: "POST", Path: "/v1/workflows", RPC: "CreateWorkflow", Covered: true, Body: "*"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}", RPC: "GetWorkflow", Covered: true},
	{Method: "DELETE", Path: "/v1/workflows/{workflowId}", RPC: "DeleteWorkflow", Covered: true},
	{Method: "PUT", Path: "/v1/workflows/{workflowId}", RPC: "UpdateWorkflow", Covered: true, Body: "*"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions", RPC: "ListExecutions", Covered: true},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions/{executionId}", RPC: "GetExecution", Covered: true},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions/{executionId}:cancel", RPC: "CancelExecution", Covered: true, Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions/{executionId}:resume", RPC: "ResumeExecution", Covered: true, Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions:start", RPC: "StartExecution", Covered: true, Body: "*"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/history", RPC: "GetWorkflowHistory", Covered: true},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/triggers", RPC: "ListWorkflowTriggers", Covered: true},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/triggers", RPC: "CreateWorkflowTrigger", Covered: true, Body: "trigger"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "GetWorkflowTrigger", Covered: true},
	{Method: "DELETE", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "DeleteWorkflowTrigger", Covered: true},
	{Method: "PATCH", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "UpdateWorkflowTrigger", Covered: true, Body: "trigger"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}:rotateWebhookToken", RPC: "RotateWorkflowTriggerWebhookToken", Covered: true, Body: "*"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/versions/{version}", RPC: "GetWorkflowVersion", Covered: true},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/versions/{version}:revert", RPC: "RevertWorkflowVersion", Covered: true, Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}:activate", RPC: "ActivateWorkflow", Covered: true, Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}:deactivate", RPC: "DeactivateWorkflow", Covered: true, Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}:publish", RPC: "PublishWorkflowVersion", Covered: true, Body: "*"},
}
