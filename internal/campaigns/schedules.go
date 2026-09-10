package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type campaignSchedule struct {
	Name                  string   `json:"name"`
	DisplayName           string   `json:"display_name"`
	Timezone              string   `json:"timezone"`
	Days                  []string `json:"days"`
	StartTime             string   `json:"start_time"`
	EndTime               string   `json:"end_time"`
	MinimumSpacingSeconds int64    `json:"minimum_spacing_seconds"`
	Organization          string   `json:"organization"`
	Etag                  string   `json:"etag"`
	CreateTime            string   `json:"create_time"`
	UpdateTime            string   `json:"update_time"`
}

func newSchedulesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "schedules", Short: "Manage campaign schedules", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newSchedulesListCommand(f), newSchedulesGetCommand(f), newSchedulesCreateCommand(f), newSchedulesUpdateCommand(f), newSchedulesDeleteCommand(f))
	return cmd
}

func newSchedulesListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List schedules", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum schedules to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSchedulesList(cmd.Context(), f, limit)
	}
	return cmd
}

func newSchedulesGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get SCHEDULE_ID", Short: "Get a schedule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("schedule id")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSchedulesGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newSchedulesCreateCommand(f *cli.Factory) *cobra.Command {
	var source, displayName, timezone, startTime, endTime, requestID string
	var spacing int
	cmd := &cobra.Command{Use: "create", Short: "Create a schedule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Schedule display name")
	cmd.Flags().StringVar(&timezone, "timezone", "", "IANA timezone")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Local start time (HH:MM)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "Local end time (HH:MM)")
	cmd.Flags().IntVar(&spacing, "minimum-spacing-seconds", 0, "Minimum spacing between sends in seconds")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSchedulesCreate(cmd.Context(), f, source, displayName, timezone, startTime, endTime, requestID, spacing, cmd.Flags().Changed("display-name"), cmd.Flags().Changed("timezone"), cmd.Flags().Changed("start-time"), cmd.Flags().Changed("end-time"), cmd.Flags().Changed("minimum-spacing-seconds"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newSchedulesUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, requestID, etag string
	cmd := &cobra.Command{Use: "update SCHEDULE_ID --input FILE", Short: "Update a schedule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("schedule id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Schedule etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSchedulesUpdate(cmd.Context(), f, args[0], source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newSchedulesDeleteCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete SCHEDULE_ID", Short: "Delete a schedule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("schedule id")}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Schedule etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSchedulesDelete(cmd.Context(), f, args[0], requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func runSchedulesList(ctx context.Context, f *cli.Factory, limit int) error {
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	items, raw, err := listSchedules(ctx, f, limit)
	if err != nil {
		return err
	}
	return printSchedules(f, raw, items)
}

func runSchedulesGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireArg([]string{id}, "schedule id")
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, schedulePath(id), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeSchedule(resp.Body)
	if err != nil {
		return err
	}
	return printSchedules(f, resp.Body, []campaignSchedule{row})
}

func runSchedulesCreate(ctx context.Context, f *cli.Factory, source, displayName, timezone, startTime, endTime, requestID string, spacing int, displayNameSet, timezoneSet, startTimeSet, endTimeSet, spacingSet, requestIDSet bool) error {
	obj, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := obj.MergeFlag("display_name", "display-name", displayName, displayNameSet); err != nil {
		return err
	}
	if err := obj.MergeFlag("timezone", "timezone", timezone, timezoneSet); err != nil {
		return err
	}
	if err := obj.MergeFlag("start_time", "start-time", startTime, startTimeSet); err != nil {
		return err
	}
	if err := obj.MergeFlag("end_time", "end-time", endTime, endTimeSet); err != nil {
		return err
	}
	if err := obj.MergeFlag("minimum_spacing_seconds", "minimum-spacing-seconds", spacing, spacingSet); err != nil {
		return err
	}
	if err := ensureOrganization(f, obj); err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/campaignSchedules", nil, obj)
	if err != nil {
		return err
	}
	row, err := decodeSchedule(resp.Body)
	if err != nil {
		return err
	}
	return printSchedules(f, resp.Body, []campaignSchedule{row})
}

func runSchedulesUpdate(ctx context.Context, f *cli.Factory, id, source, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "schedule id")
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
	if err := ensureEtag(ctx, f, obj, etag, etagSet, schedulePath(id)); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPatch, schedulePath(id), nil, obj)
	if err != nil {
		return err
	}
	row, err := decodeSchedule(resp.Body)
	if err != nil {
		return err
	}
	return printSchedules(f, resp.Body, []campaignSchedule{row})
}

func runSchedulesDelete(ctx context.Context, f *cli.Factory, id, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "schedule id")
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete schedule " + id + "?"); err != nil {
		return err
	}
	obj := input.Object{}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, schedulePath(id)); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", obj.String("request_id"))
	query.Set("etag", obj.String("etag"))
	if _, err := do(ctx, f, http.MethodDelete, schedulePath(id), query, nil); err != nil {
		return err
	}
	return nil
}

func listSchedules(ctx context.Context, f *cli.Factory, limit int) ([]campaignSchedule, []byte, error) {
	collected := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := do(ctx, f, http.MethodGet, "/v1/campaignSchedules", listQuery(limit-collected, pageToken), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "campaign_schedules", "next_page_token")
		if err != nil {
			return nil, "", err
		}
		collected += len(page)
		return page, next, nil
	})
	if err != nil {
		return nil, nil, err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	rows, err := decodeScheduleRecords(items)
	if err != nil {
		return nil, nil, err
	}
	raw, err := encodeSchedulePage(items)
	if err != nil {
		return nil, nil, err
	}
	return rows, raw, nil
}

func printSchedules(f *cli.Factory, raw []byte, rows []campaignSchedule) error {
	if err := output.Table(f.Printer(), raw, rows, scheduleColumns()); err != nil {
		return fmt.Errorf("campaigns: print schedules: %w", err)
	}
	return nil
}

func scheduleColumns() []output.Column[campaignSchedule] {
	return []output.Column[campaignSchedule]{
		{Header: "ID", Value: func(row campaignSchedule) string { return resourceID(row.Name) }},
		{Header: "DISPLAY NAME", Value: func(row campaignSchedule) string { return row.DisplayName }},
		{Header: "TIMEZONE", Value: func(row campaignSchedule) string { return row.Timezone }},
		{Header: "DAYS", Value: func(row campaignSchedule) string { return strings.Join(row.Days, ",") }},
		{Header: "WINDOW", Value: func(row campaignSchedule) string { return scheduleWindow(row.StartTime, row.EndTime) }},
		{Header: "SPACING", Value: func(row campaignSchedule) string { return formatScalar(row.MinimumSpacingSeconds) }},
		{Header: "UPDATED", Value: func(row campaignSchedule) string { return row.UpdateTime }},
	}
}

func scheduleWindow(start, end string) string {
	if start == "" && end == "" {
		return ""
	}
	return start + "-" + end
}

func schedulePath(id string) string {
	return "/v1/campaignSchedules/" + url.PathEscape(resourceID(id))
}

func decodeSchedule(raw []byte) (campaignSchedule, error) {
	var row campaignSchedule
	if err := json.Unmarshal(raw, &row); err != nil {
		return campaignSchedule{}, fmt.Errorf("campaigns: decode schedule: %w", err)
	}
	return row, nil
}

func decodeScheduleRecords(items []json.RawMessage) ([]campaignSchedule, error) {
	rows := make([]campaignSchedule, 0, len(items))
	for _, raw := range items {
		row, err := decodeSchedule(raw)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func encodeSchedulePage(items []json.RawMessage) ([]byte, error) {
	raw, err := json.Marshal(struct {
		CampaignSchedules []json.RawMessage `json:"campaign_schedules"`
	}{CampaignSchedules: items})
	if err != nil {
		return nil, fmt.Errorf("campaigns: encode schedules: %w", err)
	}
	return raw, nil
}
