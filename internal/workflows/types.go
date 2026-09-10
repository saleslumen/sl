package workflows

import (
	"strconv"

	"github.com/saleslumen/sl/internal/output"
)

const (
	productWorkflows      = "workflows"
	defaultPageSize       = 50
	maxTriggerPageSize    = 100
	workflowsPath         = "/v1/workflows"
	triggerEventTypesPath = "/v1/triggerEventTypes"
)

var executionStatusNames = map[int]string{
	0: "UNSPECIFIED",
	1: "ACTIVE",
	2: "PAUSED",
	3: "COMPLETED",
	4: "ERROR",
	5: "CANCELLED",
}

var executionSourceTypeNames = map[int]string{
	0: "UNSPECIFIED",
	1: "MANUAL",
	2: "WEBHOOK",
	3: "SCHEDULE",
	4: "EVENT",
}

var triggerKindNames = map[int]string{
	0: "UNSPECIFIED",
	1: "WEBHOOK",
	2: "SCHEDULE",
	3: "EVENT",
}

var triggerDisabledReasonNames = map[int]string{
	0: "UNSPECIFIED",
	1: "RUN_AS_UNAUTHORIZED",
}

type Workflow struct {
	ID             string           `json:"id,omitempty"`
	Name           string           `json:"name,omitempty"`
	Version        uint32           `json:"version,omitempty"`
	CreatedAt      string           `json:"createdAt,omitempty"`
	UpdatedAt      string           `json:"updatedAt,omitempty"`
	IsActive       bool             `json:"isActive"`
	CurrentVersion *WorkflowVersion `json:"currentVersion,omitempty"`
}

type WorkflowVersion struct {
	ID        string `json:"id,omitempty"`
	Version   uint32 `json:"version,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	IsLive    bool   `json:"isLive"`
}

type WorkflowTrigger struct {
	ID             string `json:"id,omitempty"`
	WorkflowID     string `json:"workflowId,omitempty"`
	Kind           int    `json:"kind"`
	Enabled        bool   `json:"enabled"`
	DisabledReason int    `json:"disabledReason"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type Execution struct {
	ID          string `json:"id,omitempty"`
	WorkflowID  string `json:"workflowId,omitempty"`
	Status      int    `json:"status"`
	SourceType  int    `json:"sourceType"`
	Version     uint32 `json:"version,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type TriggerEventType struct {
	EventType   string `json:"eventType,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

type workflowResponse struct {
	Workflow Workflow `json:"workflow"`
}

func enumName(names map[int]string, value int) string {
	if name, ok := names[value]; ok {
		return name
	}
	return strconv.Itoa(value)
}

func formatUint32(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}

func workflowColumns() []output.Column[Workflow] {
	return []output.Column[Workflow]{
		{Header: "ID", Value: func(row Workflow) string { return row.ID }},
		{Header: "NAME", Value: func(row Workflow) string { return row.Name }},
		{Header: "VERSION", Value: func(row Workflow) string { return formatUint32(row.Version) }},
		{Header: "ACTIVE", Value: func(row Workflow) string { return strconv.FormatBool(row.IsActive) }},
		{Header: "UPDATED", Value: func(row Workflow) string { return row.UpdatedAt }},
	}
}

func versionColumns() []output.Column[WorkflowVersion] {
	return []output.Column[WorkflowVersion]{
		{Header: "VERSION", Value: func(row WorkflowVersion) string { return formatUint32(row.Version) }},
		{Header: "ID", Value: func(row WorkflowVersion) string { return row.ID }},
		{Header: "LIVE", Value: func(row WorkflowVersion) string { return strconv.FormatBool(row.IsLive) }},
		{Header: "CREATED", Value: func(row WorkflowVersion) string { return row.CreatedAt }},
	}
}

func triggerColumns() []output.Column[WorkflowTrigger] {
	return []output.Column[WorkflowTrigger]{
		{Header: "ID", Value: func(row WorkflowTrigger) string { return row.ID }},
		{Header: "KIND", Value: func(row WorkflowTrigger) string { return enumName(triggerKindNames, row.Kind) }},
		{Header: "ENABLED", Value: func(row WorkflowTrigger) string { return strconv.FormatBool(row.Enabled) }},
		{Header: "DISABLED_REASON", Value: func(row WorkflowTrigger) string {
			return enumName(triggerDisabledReasonNames, row.DisabledReason)
		}},
		{Header: "UPDATED", Value: func(row WorkflowTrigger) string { return row.UpdatedAt }},
	}
}

func executionColumns() []output.Column[Execution] {
	return []output.Column[Execution]{
		{Header: "ID", Value: func(row Execution) string { return row.ID }},
		{Header: "STATUS", Value: func(row Execution) string { return enumName(executionStatusNames, row.Status) }},
		{Header: "SOURCE", Value: func(row Execution) string { return enumName(executionSourceTypeNames, row.SourceType) }},
		{Header: "VERSION", Value: func(row Execution) string { return formatUint32(row.Version) }},
		{Header: "STARTED", Value: func(row Execution) string { return row.StartedAt }},
		{Header: "COMPLETED", Value: func(row Execution) string { return row.CompletedAt }},
	}
}

func eventTypeColumns() []output.Column[TriggerEventType] {
	return []output.Column[TriggerEventType]{
		{Header: "EVENT_TYPE", Value: func(row TriggerEventType) string { return row.EventType }},
		{Header: "DISPLAY_NAME", Value: func(row TriggerEventType) string { return row.DisplayName }},
		{Header: "DESCRIPTION", Value: func(row TriggerEventType) string { return row.Description }},
	}
}
