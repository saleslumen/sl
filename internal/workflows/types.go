package workflows

import (
	"encoding/json"

	"github.com/saleslumen/sl/internal/output"
)

const (
	productWorkflows      = "workflows"
	defaultPageSize       = 50
	maxPageSize           = 1000
	maxSearchPageSize     = 20
	workflowsPath         = "/v1/workflows"
	triggerEventTypesPath = "/v1/triggerEventTypes"
)

type Workflow struct {
	Name            string `json:"name"`
	UID             string `json:"uid"`
	DisplayName     string `json:"displayName"`
	Etag            string `json:"etag"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
	ServingRevision string `json:"servingRevision"`
}

type Revision struct {
	Name             string            `json:"name"`
	Changelog        string            `json:"changelog"`
	ValidationErrors []json.RawMessage `json:"validationErrors"`
	CreateTime       string            `json:"createTime"`
}

type Execution struct {
	Name       string            `json:"name"`
	UID        string            `json:"uid"`
	Revision   string            `json:"revision"`
	State      string            `json:"state"`
	Source     json.RawMessage   `json:"source"`
	Vars       json.RawMessage   `json:"vars"`
	Waits      []json.RawMessage `json:"waits"`
	Result     json.RawMessage   `json:"result"`
	Error      json.RawMessage   `json:"error"`
	Etag       string            `json:"etag"`
	CreateTime string            `json:"createTime"`
	UpdateTime string            `json:"updateTime"`
	EndTime    string            `json:"endTime"`
}

type StepRun struct {
	Name      string          `json:"name"`
	StepPath  string          `json:"stepPath"`
	Kind      string          `json:"kind"`
	State     string          `json:"state"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	Error     json.RawMessage `json:"error"`
	Attempt   int             `json:"attempt"`
	StartTime string          `json:"startTime"`
	EndTime   string          `json:"endTime"`
}

type Trigger struct {
	Name           string `json:"name"`
	UID            string `json:"uid"`
	Etag           string `json:"etag"`
	Kind           string `json:"kind"`
	Enabled        bool   `json:"enabled"`
	RunAsUser      string `json:"runAsUser"`
	DisabledReason string `json:"disabledReason"`
	DeleteTime     string `json:"deleteTime"`
	WebhookURL     string `json:"webhook_url"`
	Cron           string `json:"cron"`
	TimeZone       string `json:"timeZone"`
	EventType      string `json:"eventType"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}

type TriggerEventType struct {
	EventType string `json:"eventType"`
}

func workflowColumns() []output.Column[Workflow] {
	return []output.Column[Workflow]{
		{Header: "NAME", Value: func(row Workflow) string { return row.Name }},
		{Header: "DISPLAY_NAME", Value: func(row Workflow) string { return row.DisplayName }},
		{Header: "UID", Value: func(row Workflow) string { return row.UID }},
		{Header: "ETAG", Value: func(row Workflow) string { return row.Etag }},
		{Header: "CREATED", Value: func(row Workflow) string { return row.CreateTime }},
		{Header: "UPDATED", Value: func(row Workflow) string { return row.UpdateTime }},
		{Header: "SERVING_REVISION", Value: func(row Workflow) string { return row.ServingRevision }},
	}
}

func revisionColumns() []output.Column[Revision] {
	return []output.Column[Revision]{
		{Header: "NAME", Value: func(row Revision) string { return row.Name }},
		{Header: "CHANGELOG", Value: func(row Revision) string { return row.Changelog }},
		{Header: "CREATED", Value: func(row Revision) string { return row.CreateTime }},
	}
}

func executionColumns() []output.Column[Execution] {
	return []output.Column[Execution]{
		{Header: "NAME", Value: func(row Execution) string { return row.Name }},
		{Header: "UID", Value: func(row Execution) string { return row.UID }},
		{Header: "REVISION", Value: func(row Execution) string { return row.Revision }},
		{Header: "STATE", Value: func(row Execution) string { return row.State }},
		{Header: "ETAG", Value: func(row Execution) string { return row.Etag }},
		{Header: "CREATED", Value: func(row Execution) string { return row.CreateTime }},
		{Header: "UPDATED", Value: func(row Execution) string { return row.UpdateTime }},
		{Header: "ENDED", Value: func(row Execution) string { return row.EndTime }},
	}
}

func stepRunColumns() []output.Column[StepRun] {
	return []output.Column[StepRun]{
		{Header: "NAME", Value: func(row StepRun) string { return row.Name }},
		{Header: "STEP_PATH", Value: func(row StepRun) string { return row.StepPath }},
		{Header: "KIND", Value: func(row StepRun) string { return row.Kind }},
		{Header: "STATE", Value: func(row StepRun) string { return row.State }},
		{Header: "ATTEMPT", Value: func(row StepRun) string { return formatInt(row.Attempt) }},
		{Header: "STARTED", Value: func(row StepRun) string { return row.StartTime }},
		{Header: "ENDED", Value: func(row StepRun) string { return row.EndTime }},
	}
}

func triggerColumns() []output.Column[Trigger] {
	return []output.Column[Trigger]{
		{Header: "NAME", Value: func(row Trigger) string { return row.Name }},
		{Header: "UID", Value: func(row Trigger) string { return row.UID }},
		{Header: "ETAG", Value: func(row Trigger) string { return row.Etag }},
		{Header: "KIND", Value: func(row Trigger) string { return row.Kind }},
		{Header: "ENABLED", Value: func(row Trigger) string { return formatBool(row.Enabled) }},
		{Header: "DISABLED_REASON", Value: func(row Trigger) string { return row.DisabledReason }},
		{Header: "CREATED", Value: func(row Trigger) string { return row.CreateTime }},
		{Header: "UPDATED", Value: func(row Trigger) string { return row.UpdateTime }},
		{Header: "DELETED", Value: func(row Trigger) string { return row.DeleteTime }},
	}
}

func eventTypeColumns() []output.Column[TriggerEventType] {
	return []output.Column[TriggerEventType]{
		{Header: "EVENT_TYPE", Value: func(row TriggerEventType) string { return row.EventType }},
	}
}
