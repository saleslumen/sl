package workflows

type artifactRoute struct {
	Method string
	Path   string
	RPC    string
	Body   string
	Query  []string
	User   bool
}

// routes is the typed WorkflowsService surface from its generated HTTP bindings.
var routes = []artifactRoute{
	{Method: "GET", Path: "/v1/triggerEventTypes", RPC: "ListTriggerEventTypes", Query: []string{"pageSize", "pageToken"}},
	{Method: "POST", Path: "/v1/workflows", RPC: "CreateWorkflow", Body: "workflow", Query: []string{"workflowId"}},
	{Method: "GET", Path: "/v1/workflows", RPC: "ListWorkflows", Query: []string{"pageSize", "pageToken", "query"}},
	{Method: "GET", Path: "/v1/workflows/{workflowId}", RPC: "GetWorkflow"},
	{Method: "PATCH", Path: "/v1/workflows/{workflowId}", RPC: "UpdateWorkflow", Body: "workflow", Query: []string{"updateMask"}},
	{Method: "DELETE", Path: "/v1/workflows/{workflowId}", RPC: "DeleteWorkflow", Query: []string{"etag", "force"}},
	{Method: "POST", Path: "/v1/workflows/{workflowId}:deploy", RPC: "DeployWorkflow", Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}:undeploy", RPC: "UndeployWorkflow", Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/revisions", RPC: "CreateRevision", Body: "revision", Query: []string{"revisionId"}},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/revisions/{revisionId}", RPC: "GetRevision"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/revisions", RPC: "ListRevisions", Query: []string{"pageSize", "pageToken"}},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions", RPC: "CreateExecution", Body: "execution", Query: []string{"executionId", "requestId"}, User: true},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions/{executionId}", RPC: "GetExecution"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions", RPC: "ListExecutions", Query: []string{"pageSize", "pageToken", "filter"}},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions/{executionId}:cancel", RPC: "CancelExecution", Body: "*"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/executions/{executionId}:signal", RPC: "SignalExecution", Body: "*", User: true},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions/{executionId}/stepRuns/{sequence}", RPC: "GetStepRun"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/executions/{executionId}/stepRuns", RPC: "ListStepRuns", Query: []string{"pageSize", "pageToken"}},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/triggers", RPC: "CreateTrigger", Body: "trigger", Query: []string{"triggerId"}, User: true},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "GetTrigger"},
	{Method: "GET", Path: "/v1/workflows/{workflowId}/triggers", RPC: "ListTriggers", Query: []string{"pageSize", "pageToken", "showDeleted"}},
	{Method: "PATCH", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "UpdateTrigger", Body: "trigger", Query: []string{"updateMask"}},
	{Method: "DELETE", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}", RPC: "DeleteTrigger"},
	{Method: "POST", Path: "/v1/workflows/{workflowId}/triggers/{triggerId}:rotateWebhookToken", RPC: "RotateWebhookToken", Body: "*"},
}
