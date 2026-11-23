package script

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type metricsValue struct {
	Value     string `json:"value"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type metrics struct {
	ActiveUsers      []metricsValue `json:"activeUsers"`
	TotalExecutions  []metricsValue `json:"totalExecutions"`
	FailedExecutions []metricsValue `json:"failedExecutions"`
}

type metricsRow struct {
	Metric    string
	Value     string
	StartTime string
	EndTime   string
}

func newMetricsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "metrics", Short: "Read script metrics", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newMetricsGetCommand(f))
	return cmd
}

func newMetricsGetCommand(f *cli.Factory) *cobra.Command {
	var project, granularity string
	cmd := &cobra.Command{Use: "get --project ID", Short: "Get project metrics", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().StringVar(&granularity, "granularity", "", "Metrics granularity (DAILY or HOURLY)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMetricsGet(cmd.Context(), f, project, granularity)
	}
	return cmd
}

func runMetricsGet(ctx context.Context, f *cli.Factory, project, granularity string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	switch granularity {
	case "", "DAILY", "HOURLY":
	default:
		return &cli.UsageError{Msg: "--granularity must be DAILY or HOURLY"}
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	query := url.Values{}
	if granularity != "" {
		query.Set("metricsGranularity", granularity)
	}
	resp, err := doScript(ctx, client, http.MethodGet, metricsPath(project), query, nil)
	if err != nil {
		return err
	}
	payload, err := decodeObject[metrics](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, flattenMetrics(payload), metricsColumns())
}

func flattenMetrics(payload metrics) []metricsRow {
	var rows []metricsRow
	appendMetric := func(name string, values []metricsValue) {
		for _, value := range values {
			rows = append(rows, metricsRow{Metric: name, Value: value.Value, StartTime: value.StartTime, EndTime: value.EndTime})
		}
	}
	appendMetric("activeUsers", payload.ActiveUsers)
	appendMetric("totalExecutions", payload.TotalExecutions)
	appendMetric("failedExecutions", payload.FailedExecutions)
	return rows
}

func metricsColumns() []output.Column[metricsRow] {
	return []output.Column[metricsRow]{
		{Header: "METRIC", Value: func(r metricsRow) string { return r.Metric }},
		{Header: "VALUE", Value: func(r metricsRow) string { return r.Value }},
		{Header: "START_TIME", Value: func(r metricsRow) string { return r.StartTime }},
		{Header: "END_TIME", Value: func(r metricsRow) string { return r.EndTime }},
	}
}

func metricsPath(project string) string {
	return projectPath(project) + "/metrics"
}
