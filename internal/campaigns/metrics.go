package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type metricValue struct {
	Count       *json.Number `json:"count"`
	Rate        *json.Number `json:"rate"`
	Numerator   string       `json:"numerator"`
	Denominator string       `json:"denominator"`
	Unavailable bool         `json:"unavailable"`
}

type campaignPeopleMetrics struct {
	SnapshotTime string `json:"snapshot_time"`
	Active       int64  `json:"active"`
	Paused       int64  `json:"paused"`
	NotSent      int64  `json:"not_sent"`
	Finished     int64  `json:"finished"`
}

type campaignVariantMetrics struct {
	Variant           string       `json:"variant"`
	ScheduledMessages *metricValue `json:"scheduled_messages"`
	SentMessages      *metricValue `json:"sent_messages"`
	BouncedMessages   *metricValue `json:"bounced_messages"`
	UniqueReplied     *metricValue `json:"unique_replied"`
}

type campaignStepMetrics struct {
	Sequence          string                   `json:"sequence"`
	Step              string                   `json:"step"`
	ScheduledMessages *metricValue             `json:"scheduled_messages"`
	SentMessages      *metricValue             `json:"sent_messages"`
	BouncedMessages   *metricValue             `json:"bounced_messages"`
	UniqueReplied     *metricValue             `json:"unique_replied"`
	InterestedPeople  *metricValue             `json:"interested_people"`
	Variants          []campaignVariantMetrics `json:"variants"`
}

type campaignMetrics struct {
	Campaign          string                `json:"campaign"`
	TimeRangeStart    string                `json:"time_range_start"`
	TimeRangeEnd      string                `json:"time_range_end"`
	ReportingTimezone string                `json:"reporting_timezone"`
	SentMessages      *metricValue          `json:"sent_messages"`
	BouncedMessages   *metricValue          `json:"bounced_messages"`
	UniqueOpened      *metricValue          `json:"unique_opened"`
	UniqueClicked     *metricValue          `json:"unique_clicked"`
	UniqueReplied     *metricValue          `json:"unique_replied"`
	People            campaignPeopleMetrics `json:"people"`
	Steps             []campaignStepMetrics `json:"steps"`
}

type metricRow struct {
	Metric      string
	Count       string
	Rate        string
	Numerator   string
	Denominator string
	Unavailable string
}

func newMetricsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "metrics", Short: "Read campaign metrics", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newMetricsGetCommand(f))
	return cmd
}

func newMetricsGetCommand(f *cli.Factory) *cobra.Command {
	var campaign, start, end, timezone, stepID, variantID, senderAccountID string
	cmd := &cobra.Command{Use: "get --campaign ID --time-range-start TIME --time-range-end TIME --reporting-timezone TZ", Short: "Get campaign metrics", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().StringVar(&start, "time-range-start", "", "Time range start (RFC3339)")
	cmd.Flags().StringVar(&end, "time-range-end", "", "Time range end (RFC3339)")
	cmd.Flags().StringVar(&timezone, "reporting-timezone", "", "IANA reporting timezone")
	cmd.Flags().StringVar(&stepID, "step-id", "", "Filter by step ID")
	cmd.Flags().StringVar(&variantID, "variant-id", "", "Filter by variant ID")
	cmd.Flags().StringVar(&senderAccountID, "sender-account-id", "", "Filter by sender account ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMetricsGet(cmd.Context(), f, campaign, start, end, timezone, stepID, variantID, senderAccountID)
	}
	return cmd
}

func runMetricsGet(ctx context.Context, f *cli.Factory, campaign, start, end, timezone, stepID, variantID, senderAccountID string) error {
	path, err := campaignMetricsPath(campaign)
	if err != nil {
		return err
	}
	start, err = requireFlagValue("time-range-start", start)
	if err != nil {
		return err
	}
	end, err = requireFlagValue("time-range-end", end)
	if err != nil {
		return err
	}
	timezone, err = requireFlagValue("reporting-timezone", timezone)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, path, metricsQuery(start, end, timezone, stepID, variantID, senderAccountID), nil)
	if err != nil {
		return err
	}
	return printMetrics(f, resp.Body)
}

func campaignMetricsPath(campaign string) (string, error) {
	id, err := requireCampaign(campaign)
	if err != nil {
		return "", err
	}
	return "/v1/campaigns/" + id + "/metrics", nil
}

func requireFlagValue(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &cli.UsageError{Msg: "--" + name + " is required"}
	}
	return value, nil
}

func metricsQuery(start, end, timezone, stepID, variantID, senderAccountID string) url.Values {
	query := url.Values{}
	query.Set("time_range_start", start)
	query.Set("time_range_end", end)
	query.Set("reporting_timezone", timezone)
	if stepID != "" {
		query.Set("step_id", stepID)
	}
	if variantID != "" {
		query.Set("variant_id", variantID)
	}
	if senderAccountID != "" {
		query.Set("sender_account_id", senderAccountID)
	}
	return query
}

func printMetrics(f *cli.Factory, raw []byte) error {
	var payload campaignMetrics
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("campaigns: decode metrics: %w", err)
	}
	if err := output.Table(f.Printer(), raw, metricRows(payload), metricColumns()); err != nil {
		return fmt.Errorf("campaigns: print metrics: %w", err)
	}
	return nil
}

func metricRows(payload campaignMetrics) []metricRow {
	specs := []struct {
		name  string
		value *metricValue
	}{
		{"sent_messages", payload.SentMessages},
		{"bounced_messages", payload.BouncedMessages},
		{"unique_opened", payload.UniqueOpened},
		{"unique_clicked", payload.UniqueClicked},
		{"unique_replied", payload.UniqueReplied},
	}
	rows := make([]metricRow, 0, len(specs))
	for _, spec := range specs {
		if spec.value == nil {
			continue
		}
		rows = append(rows, metricRow{
			Metric:      spec.name,
			Count:       formatMetricNumber(spec.value.Count),
			Rate:        formatMetricNumber(spec.value.Rate),
			Numerator:   spec.value.Numerator,
			Denominator: spec.value.Denominator,
			Unavailable: formatScalar(spec.value.Unavailable),
		})
	}
	return rows
}

func formatMetricNumber(n *json.Number) string {
	if n == nil {
		return ""
	}
	return n.String()
}

func metricColumns() []output.Column[metricRow] {
	return []output.Column[metricRow]{
		{Header: "METRIC", Value: func(row metricRow) string { return row.Metric }},
		{Header: "COUNT", Value: func(row metricRow) string { return row.Count }},
		{Header: "RATE", Value: func(row metricRow) string { return row.Rate }},
		{Header: "NUMERATOR", Value: func(row metricRow) string { return row.Numerator }},
		{Header: "DENOMINATOR", Value: func(row metricRow) string { return row.Denominator }},
		{Header: "UNAVAILABLE", Value: func(row metricRow) string { return row.Unavailable }},
	}
}
