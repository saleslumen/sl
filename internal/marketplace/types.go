package marketplace

import (
	"strconv"

	"github.com/saleslumen/sl/internal/output"
)

type catalogApp struct {
	AppID          string `json:"appId"`
	LifecycleState string `json:"lifecycleState"`
	NameDisplay    string `json:"nameDisplay"`
}

type app struct {
	AppID          string `json:"appId"`
	LifecycleState string `json:"lifecycleState"`
	Slug           string `json:"slug"`
}

type operationContract struct {
	OperationKey string `json:"operationKey"`
	Description  string `json:"description"`
}

type listing struct {
	ListingID        string `json:"listingId"`
	Locale           string `json:"locale"`
	DraftRevisionID  string `json:"draftRevisionId"`
	ActiveRevisionID string `json:"activeRevisionId"`
}

type listingRevision struct {
	RevisionID  string `json:"revisionId"`
	State       string `json:"state"`
	NameDisplay string `json:"nameDisplay"`
}

type oauthClient struct {
	ConnectionKey string `json:"connectionKey"`
	ClientID      string `json:"clientId"`
	Frozen        bool   `json:"frozen"`
}

type release struct {
	ReleaseID     string `json:"releaseId"`
	State         string `json:"state"`
	ReleaseNumber int64  `json:"releaseNumber"`
}

type submission struct {
	SubmissionID string `json:"submissionId"`
	State        string `json:"state"`
	ReviewMode   string `json:"reviewMode"`
}

type reviewDetail struct {
	Submission submission `json:"submission"`
}

type installation struct {
	InstallationID string `json:"installationId"`
	LifecycleState string `json:"lifecycleState"`
	AppID          string `json:"appId"`
}

type configuration struct {
	InstallationID string `json:"installationId"`
	SchemaDigest   string `json:"schemaDigest"`
	Etag           string `json:"etag"`
}

type connection struct {
	ConnectionKey string `json:"connectionKey"`
	Status        string `json:"status"`
	DisplayLabel  string `json:"displayLabel"`
}

type authorizationStart struct {
	Connection            connection `json:"connection"`
	AuthorizationStartURI string     `json:"authorizationStartUri"`
}

type apiKeyOutcome struct {
	Outcome string `json:"outcome"`
}

type organizationPolicy struct {
	OrganizationID       string `json:"organizationId"`
	AllowUserSelfInstall bool   `json:"allowUserSelfInstall"`
}

type installCapabilities struct {
	OrganizationID         string `json:"organizationId"`
	NamespaceID            string `json:"namespaceId"`
	CanInstallUser         bool   `json:"canInstallUser"`
	CanInstallNamespace    bool   `json:"canInstallNamespace"`
	CanInstallOrganization bool   `json:"canInstallOrganization"`
}

func catalogColumns() []output.Column[catalogApp] {
	return []output.Column[catalogApp]{
		{Header: "APP_ID", Value: func(row catalogApp) string { return row.AppID }},
		{Header: "LIFECYCLE_STATE", Value: func(row catalogApp) string { return row.LifecycleState }},
		{Header: "NAME_DISPLAY", Value: func(row catalogApp) string { return row.NameDisplay }},
	}
}

func appColumns() []output.Column[app] {
	return []output.Column[app]{
		{Header: "APP_ID", Value: func(row app) string { return row.AppID }},
		{Header: "LIFECYCLE_STATE", Value: func(row app) string { return row.LifecycleState }},
		{Header: "SLUG", Value: func(row app) string { return row.Slug }},
	}
}

func operationColumns() []output.Column[operationContract] {
	return []output.Column[operationContract]{
		{Header: "OPERATION_KEY", Value: func(row operationContract) string { return row.OperationKey }},
		{Header: "DESCRIPTION", Value: func(row operationContract) string { return row.Description }},
	}
}

func listingColumns() []output.Column[listing] {
	return []output.Column[listing]{
		{Header: "LISTING_ID", Value: func(row listing) string { return row.ListingID }},
		{Header: "LOCALE", Value: func(row listing) string { return row.Locale }},
		{Header: "DRAFT_REVISION_ID", Value: func(row listing) string { return row.DraftRevisionID }},
		{Header: "ACTIVE_REVISION_ID", Value: func(row listing) string { return row.ActiveRevisionID }},
	}
}

func revisionColumns() []output.Column[listingRevision] {
	return []output.Column[listingRevision]{
		{Header: "REVISION_ID", Value: func(row listingRevision) string { return row.RevisionID }},
		{Header: "STATE", Value: func(row listingRevision) string { return row.State }},
		{Header: "NAME_DISPLAY", Value: func(row listingRevision) string { return row.NameDisplay }},
	}
}

func oauthClientColumns() []output.Column[oauthClient] {
	return []output.Column[oauthClient]{
		{Header: "CONNECTION_KEY", Value: func(row oauthClient) string { return row.ConnectionKey }},
		{Header: "CLIENT_ID", Value: func(row oauthClient) string { return row.ClientID }},
		{Header: "FROZEN", Value: func(row oauthClient) string { return strconv.FormatBool(row.Frozen) }},
	}
}

func releaseColumns() []output.Column[release] {
	return []output.Column[release]{
		{Header: "RELEASE_ID", Value: func(row release) string { return row.ReleaseID }},
		{Header: "STATE", Value: func(row release) string { return row.State }},
		{Header: "RELEASE_NUMBER", Value: func(row release) string { return strconv.FormatInt(row.ReleaseNumber, 10) }},
	}
}

func submissionColumns() []output.Column[submission] {
	return []output.Column[submission]{
		{Header: "SUBMISSION_ID", Value: func(row submission) string { return row.SubmissionID }},
		{Header: "STATE", Value: func(row submission) string { return row.State }},
		{Header: "REVIEW_MODE", Value: func(row submission) string { return row.ReviewMode }},
	}
}

func installationColumns() []output.Column[installation] {
	return []output.Column[installation]{
		{Header: "INSTALLATION_ID", Value: func(row installation) string { return row.InstallationID }},
		{Header: "LIFECYCLE_STATE", Value: func(row installation) string { return row.LifecycleState }},
		{Header: "APP_ID", Value: func(row installation) string { return row.AppID }},
	}
}

func configurationColumns() []output.Column[configuration] {
	return []output.Column[configuration]{
		{Header: "INSTALLATION_ID", Value: func(row configuration) string { return row.InstallationID }},
		{Header: "SCHEMA_DIGEST", Value: func(row configuration) string { return row.SchemaDigest }},
		{Header: "ETAG", Value: func(row configuration) string { return row.Etag }},
	}
}

func connectionColumns() []output.Column[connection] {
	return []output.Column[connection]{
		{Header: "CONNECTION_KEY", Value: func(row connection) string { return row.ConnectionKey }},
		{Header: "STATUS", Value: func(row connection) string { return row.Status }},
		{Header: "DISPLAY_LABEL", Value: func(row connection) string { return row.DisplayLabel }},
	}
}

func authorizationColumns() []output.Column[authorizationStart] {
	return []output.Column[authorizationStart]{
		{Header: "CONNECTION_KEY", Value: func(row authorizationStart) string { return row.Connection.ConnectionKey }},
		{Header: "STATUS", Value: func(row authorizationStart) string { return row.Connection.Status }},
		{Header: "START_URI", Value: func(row authorizationStart) string { return row.AuthorizationStartURI }},
	}
}

func outcomeColumns() []output.Column[apiKeyOutcome] {
	return []output.Column[apiKeyOutcome]{
		{Header: "OUTCOME", Value: func(row apiKeyOutcome) string { return row.Outcome }},
	}
}

func policyColumns() []output.Column[organizationPolicy] {
	return []output.Column[organizationPolicy]{
		{Header: "ORGANIZATION_ID", Value: func(row organizationPolicy) string { return row.OrganizationID }},
		{Header: "ALLOW_USER_SELF_INSTALL", Value: func(row organizationPolicy) string { return strconv.FormatBool(row.AllowUserSelfInstall) }},
	}
}

func capabilityColumns() []output.Column[installCapabilities] {
	return []output.Column[installCapabilities]{
		{Header: "ORGANIZATION_ID", Value: func(row installCapabilities) string { return row.OrganizationID }},
		{Header: "NAMESPACE_ID", Value: func(row installCapabilities) string { return row.NamespaceID }},
		{Header: "CAN_INSTALL_USER", Value: func(row installCapabilities) string { return strconv.FormatBool(row.CanInstallUser) }},
		{Header: "CAN_INSTALL_NAMESPACE", Value: func(row installCapabilities) string { return strconv.FormatBool(row.CanInstallNamespace) }},
		{Header: "CAN_INSTALL_ORGANIZATION", Value: func(row installCapabilities) string { return strconv.FormatBool(row.CanInstallOrganization) }},
	}
}
