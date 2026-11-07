package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type personRecord struct {
	Name            string  `json:"name"`
	EmailAddress    string  `json:"email_address"`
	EnrollmentState string  `json:"enrollment_state"`
	ActiveSequence  string  `json:"active_sequence"`
	EligibleTime    *string `json:"eligible_time"`
	UpdateTime      string  `json:"update_time"`
}

type activityRecord struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	ReasonCode string `json:"reason_code"`
	Summary    string `json:"summary"`
	OccurredAt string `json:"occurred_at"`
}

func newPeopleCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "people", Short: "Manage campaign people", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newPeopleListCommand(f), newPeopleGetCommand(f), newPeopleCreateCommand(f), newPeopleUpdateCommand(f), newPeopleDeleteCommand(f), newPeopleImportCommand(f), newPeopleBatchDeleteCommand(f), newPeopleRunScriptCommand(f), newPeoplePauseCommand(f), newPeopleResumeCommand(f), newPeopleUnsubscribeCommand(f), newPeopleActivityCommand(f))
	return cmd
}

func newPeopleListCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	limit := 50
	cmd := &cobra.Command{Use: "list --campaign ID", Short: "List campaign people", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(cmd, &campaign)
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum people to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleList(cmd.Context(), f, campaign, limit)
	}
	return cmd
}

func newPeopleGetCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "get ID --campaign ID", Short: "Get a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleGet(cmd.Context(), f, campaign, args[0])
	}
	return cmd
}

func newPeopleCreateCommand(f *cli.Factory) *cobra.Command {
	var campaign, emailAddress, source, requestID string
	cmd := &cobra.Command{Use: "create --campaign ID --input FILE", Short: "Create a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(cmd, &campaign)
	cmd.Flags().StringVar(&emailAddress, "email-address", "", "Person email address")
	peopleBindInput(cmd, &source)
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleCreate(cmd.Context(), f, campaign, emailAddress, source, requestID, cmd.Flags().Changed("email-address"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeopleUpdateCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, etag, requestID string
	cmd := &cobra.Command{Use: "update ID --campaign ID --input FILE", Short: "Update person variables", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	peopleBindInput(cmd, &source)
	peopleBindEtag(cmd, &etag, "Person etag; fetched from the person when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleUpdate(cmd.Context(), f, campaign, args[0], source, etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeopleDeleteCommand(f *cli.Factory) *cobra.Command {
	var campaign, etag, requestID string
	cmd := &cobra.Command{Use: "delete ID --campaign ID", Short: "Delete a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	peopleBindEtag(cmd, &etag, "Campaign etag; fetched from the campaign when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleDelete(cmd.Context(), f, campaign, args[0], etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeopleImportCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, requestID string
	cmd := &cobra.Command{Use: "import --campaign ID --input FILE", Short: "Import campaign people", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(cmd, &campaign)
	peopleBindInput(cmd, &source)
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleImport(cmd.Context(), f, campaign, source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeopleBatchDeleteCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, etag, requestID string
	cmd := &cobra.Command{Use: "batch-delete --campaign ID --input FILE", Short: "Delete campaign people in batch", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(cmd, &campaign)
	peopleBindInput(cmd, &source)
	peopleBindEtag(cmd, &etag, "Campaign etag; fetched from the campaign when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleBatchDelete(cmd.Context(), f, campaign, source, etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeopleRunScriptCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, requestID string
	cmd := &cobra.Command{Use: "run-script --campaign ID --input FILE", Short: "Run a people script; installed-app needs a user credential", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(cmd, &campaign)
	peopleBindInput(cmd, &source)
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleRunScript(cmd.Context(), f, campaign, source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newPeoplePauseCommand(f *cli.Factory) *cobra.Command {
	var campaign, etag, requestID string
	cmd := &cobra.Command{Use: "pause ID --campaign ID", Short: "Pause a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	peopleBindEtag(cmd, &etag, "Person etag; fetched from the person when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleLifecycle(cmd.Context(), f, campaign, args[0], "pause", etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"), printOperation)
	}
	return cmd
}

func newPeopleResumeCommand(f *cli.Factory) *cobra.Command {
	var campaign, etag, requestID string
	cmd := &cobra.Command{Use: "resume ID --campaign ID", Short: "Resume a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	peopleBindEtag(cmd, &etag, "Person etag; fetched from the person when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleLifecycle(cmd.Context(), f, campaign, args[0], "resume", etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"), peoplePrintPerson)
	}
	return cmd
}

func newPeopleUnsubscribeCommand(f *cli.Factory) *cobra.Command {
	var campaign, etag, requestID string
	cmd := &cobra.Command{Use: "unsubscribe ID --campaign ID", Short: "Unsubscribe a campaign person", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	peopleBindEtag(cmd, &etag, "Person etag; fetched from the person when omitted")
	peopleBindRequestID(cmd, &requestID)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleLifecycle(cmd.Context(), f, campaign, args[0], "unsubscribe", etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"), printOperation)
	}
	return cmd
}

func newPeopleActivityCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	limit := 50
	cmd := &cobra.Command{Use: "activity ID --campaign ID", Short: "List person activity", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("person id")}
	peopleBindCampaign(cmd, &campaign)
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum activity events to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPeopleActivity(cmd.Context(), f, campaign, args[0], limit)
	}
	return cmd
}

func runPeopleList(ctx context.Context, f *cli.Factory, campaign string, limit int) error {
	path, err := peopleCollectionPath(campaign)
	if err != nil {
		return err
	}
	if err := peopleRequireLimit(limit); err != nil {
		return err
	}
	items, raw, err := peopleCollectField(ctx, f, path, "people", limit)
	if err != nil {
		return err
	}
	rows, err := peopleDecodeRecords[personRecord](items)
	if err != nil {
		return err
	}
	return peoplePrintRows(f, raw, rows, personColumns())
}

func runPeopleGet(ctx context.Context, f *cli.Factory, campaign, person string) error {
	path, err := personResourcePath(campaign, person)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return peoplePrintPerson(f, resp.Body)
}

func runPeopleCreate(ctx context.Context, f *cli.Factory, campaign, emailAddress, source, requestID string, emailSet, requestIDSet bool) error {
	path, err := peopleCollectionPath(campaign)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := obj.MergeFlag("email_address", "email-address", emailAddress, emailSet); err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, path, nil, obj)
	if err != nil {
		return err
	}
	return peoplePrintPerson(f, resp.Body)
}

func runPeopleUpdate(ctx context.Context, f *cli.Factory, campaign, person, source, etag, requestID string, etagSet, requestIDSet bool) error {
	path, err := personResourcePath(campaign, person)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, path); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPatch, path, nil, obj)
	if err != nil {
		return err
	}
	return peoplePrintPerson(f, resp.Body)
}

func runPeopleDelete(ctx context.Context, f *cli.Factory, campaign, person, etag, requestID string, etagSet, requestIDSet bool) error {
	path, err := personResourcePath(campaign, person)
	if err != nil {
		return err
	}
	campaignPath, err := peopleCampaignPath(campaign)
	if err != nil {
		return err
	}
	id, err := peopleRequirePerson(person)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete person " + id + "?"); err != nil {
		return err
	}
	obj := input.Object{}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, campaignPath); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", obj.String("request_id"))
	query.Set("etag", obj.String("etag"))
	if _, err := do(ctx, f, http.MethodDelete, path, query, nil); err != nil {
		return err
	}
	return nil
}

func runPeopleImport(ctx context.Context, f *cli.Factory, campaign, source, requestID string, requestIDSet bool) error {
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/campaigns/"+campaignID+"/people:import", nil, obj)
	if err != nil {
		return err
	}
	return printTask(f, resp.Body)
}

func runPeopleBatchDelete(ctx context.Context, f *cli.Factory, campaign, source, etag, requestID string, etagSet, requestIDSet bool) error {
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete people?"); err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, "/v1/campaigns/"+campaignID); err != nil {
		return err
	}
	if _, err := do(ctx, f, http.MethodPost, "/v1/campaigns/"+campaignID+"/people:batchDelete", nil, obj); err != nil {
		return err
	}
	return nil
}

func runPeopleRunScript(ctx context.Context, f *cli.Factory, campaign, source, requestID string, requestIDSet bool) error {
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/campaigns/"+campaignID+"/people:runScript", nil, obj)
	if err != nil {
		return err
	}
	return printTask(f, resp.Body)
}

func runPeopleLifecycle(ctx context.Context, f *cli.Factory, campaign, person, verb, etag, requestID string, etagSet, requestIDSet bool, print func(*cli.Factory, []byte) error) error {
	path, err := personResourcePath(campaign, person)
	if err != nil {
		return err
	}
	obj := input.Object{}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, path); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, path+":"+verb, nil, obj)
	if err != nil {
		return err
	}
	return print(f, resp.Body)
}

func runPeopleActivity(ctx context.Context, f *cli.Factory, campaign, person string, limit int) error {
	path, err := personResourcePath(campaign, person)
	if err != nil {
		return err
	}
	if err := peopleRequireLimit(limit); err != nil {
		return err
	}
	items, raw, err := peopleCollectField(ctx, f, path+":activity", "activities", limit)
	if err != nil {
		return err
	}
	rows, err := peopleDecodeRecords[activityRecord](items)
	if err != nil {
		return err
	}
	return peoplePrintRows(f, raw, rows, activityColumns())
}

func peopleBindCampaign(cmd *cobra.Command, campaign *string) {
	cmd.Flags().StringVar(campaign, "campaign", "", "Parent campaign ID")
}

func peopleBindInput(cmd *cobra.Command, source *string) {
	cmd.Flags().StringVar(source, "input", "", "JSON request body file, or - for stdin")
}

func peopleBindRequestID(cmd *cobra.Command, requestID *string) {
	cmd.Flags().StringVar(requestID, "request-id", "", "Idempotency request ID")
}

func peopleBindEtag(cmd *cobra.Command, etag *string, usage string) {
	cmd.Flags().StringVar(etag, "etag", "", usage)
}

func peopleRequirePerson(person string) (string, error) {
	id, err := requireArg([]string{person}, "person id")
	if err != nil {
		return "", err
	}
	return resourceID(id), nil
}

func peopleRequireLimit(limit int) error {
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	return nil
}

func peopleCampaignPath(campaign string) (string, error) {
	id, err := requireCampaign(campaign)
	if err != nil {
		return "", err
	}
	return "/v1/campaigns/" + id, nil
}

func peopleCollectionPath(campaign string) (string, error) {
	base, err := peopleCampaignPath(campaign)
	if err != nil {
		return "", err
	}
	return base + "/people", nil
}

func personResourcePath(campaign, person string) (string, error) {
	base, err := peopleCollectionPath(campaign)
	if err != nil {
		return "", err
	}
	id, err := peopleRequirePerson(person)
	if err != nil {
		return "", err
	}
	return base + "/" + id, nil
}

func peopleCollectField(ctx context.Context, f *cli.Factory, path, field string, limit int) ([]json.RawMessage, []byte, error) {
	remaining := limit
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := do(ctx, f, http.MethodGet, path, listQuery(remaining, pageToken), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, field, "next_page_token")
		if err != nil {
			return nil, "", err
		}
		remaining -= len(page)
		return page, next, nil
	})
	if err != nil {
		return nil, nil, err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	envelope := input.Object{}
	if err := envelope.Set(field, items); err != nil {
		return nil, nil, err
	}
	raw, err := envelope.Encode()
	if err != nil {
		return nil, nil, err
	}
	return items, raw, nil
}

func peopleDecodeRecords[T any](items []json.RawMessage) ([]T, error) {
	rows := make([]T, 0, len(items))
	for _, item := range items {
		var row T
		if err := json.Unmarshal(item, &row); err != nil {
			return nil, fmt.Errorf("campaigns: decode item: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func peoplePrintRows[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	return output.Table(f.Printer(), raw, rows, cols)
}

func peoplePrintPerson(f *cli.Factory, raw []byte) error {
	var row personRecord
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("campaigns: decode person: %w", err)
	}
	return peoplePrintRows(f, raw, []personRecord{row}, personColumns())
}

func peopleOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func personColumns() []output.Column[personRecord] {
	return []output.Column[personRecord]{
		{Header: "ID", Value: func(row personRecord) string { return resourceID(row.Name) }},
		{Header: "EMAIL", Value: func(row personRecord) string { return row.EmailAddress }},
		{Header: "STATE", Value: func(row personRecord) string { return row.EnrollmentState }},
		{Header: "ACTIVE SEQUENCE ID", Value: func(row personRecord) string { return resourceID(row.ActiveSequence) }},
		{Header: "ELIGIBLE AT", Value: func(row personRecord) string { return peopleOptionalString(row.EligibleTime) }},
		{Header: "UPDATED", Value: func(row personRecord) string { return row.UpdateTime }},
	}
}

func activityColumns() []output.Column[activityRecord] {
	return []output.Column[activityRecord]{
		{Header: "EVENT ID", Value: func(row activityRecord) string { return row.EventID }},
		{Header: "TYPE", Value: func(row activityRecord) string { return row.Type }},
		{Header: "REASON", Value: func(row activityRecord) string { return row.ReasonCode }},
		{Header: "SUMMARY", Value: func(row activityRecord) string { return row.Summary }},
		{Header: "OCCURRED AT", Value: func(row activityRecord) string { return row.OccurredAt }},
	}
}
