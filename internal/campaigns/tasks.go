package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type task struct {
	Name       string `json:"name"`
	Campaign   string `json:"campaign"`
	Type       string `json:"type"`
	State      string `json:"state"`
	Message    string `json:"message"`
	UpdateTime string `json:"update_time"`
}

type taskStreamLine struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type taskStreamStopError struct{}

func (taskStreamStopError) Error() string {
	return "campaigns: task stream stopped"
}

func newTasksCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "tasks", Short: "Manage campaign tasks", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newTasksGetCommand(f), newTasksStreamCommand(f))
	return cmd
}

func newTasksGetCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "get TASK_ID --campaign ID", Short: "Get a campaign task", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("task id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTasksGet(cmd.Context(), f, campaign, args)
	}
	return cmd
}

func newTasksStreamCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "stream TASK_ID --campaign ID", Short: "Stream a campaign task", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("task id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTasksStream(cmd.Context(), f, campaign, args)
	}
	return cmd
}

func taskPath(campaignID, taskID string) string {
	return "/v1/campaigns/" + campaignID + "/tasks/" + taskID
}

func resolveTaskIDs(campaign string, args []string) (string, string, error) {
	id, err := requireArg(args, "task id")
	if err != nil {
		return "", "", err
	}
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return "", "", err
	}
	return campaignID, resourceID(id), nil
}

func taskColumns() []output.Column[task] {
	return []output.Column[task]{
		{Header: "ID", Value: func(row task) string { return resourceID(row.Name) }},
		{Header: "CAMPAIGN ID", Value: func(row task) string { return resourceID(row.Campaign) }},
		{Header: "TYPE", Value: func(row task) string { return row.Type }},
		{Header: "STATE", Value: func(row task) string { return row.State }},
		{Header: "MESSAGE", Value: func(row task) string { return row.Message }},
		{Header: "UPDATED", Value: func(row task) string { return row.UpdateTime }},
	}
}

func decodeTask(raw []byte) (task, error) {
	var row task
	if err := json.Unmarshal(raw, &row); err != nil {
		return task{}, fmt.Errorf("campaigns: decode task: %w", err)
	}
	return row, nil
}

func printTask(f *cli.Factory, raw []byte) error {
	row, err := decodeTask(raw)
	if err != nil {
		return err
	}
	return output.Table(f.Printer(), raw, []task{row}, taskColumns())
}

func runTasksGet(ctx context.Context, f *cli.Factory, campaign string, args []string) error {
	campaignID, taskID, err := resolveTaskIDs(campaign, args)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, taskPath(campaignID, taskID), nil, nil)
	if err != nil {
		return err
	}
	return printTask(f, resp.Body)
}

func runTasksStream(ctx context.Context, f *cli.Factory, campaign string, args []string) error {
	campaignID, taskID, err := resolveTaskIDs(campaign, args)
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	err = client.Stream(ctx, apiclient.Request{Product: "campaigns", Method: http.MethodGet, Path: taskPath(campaignID, taskID) + ":stream"}, func(name string, data []byte) error {
		return writeTaskStreamEvent(f.IO.Out, name, data)
	})
	if err == nil {
		return fmt.Errorf("campaigns: task stream ended before a terminal event")
	}
	var stop taskStreamStopError
	if errors.As(err, &stop) {
		return nil
	}
	return fmt.Errorf("campaigns: stream task: %w", err)
}

func writeTaskStreamEvent(out io.Writer, name string, data []byte) error {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		decoded = string(data)
	}
	line, err := json.Marshal(taskStreamLine{Event: name, Data: decoded})
	if err != nil {
		return fmt.Errorf("campaigns: encode task event: %w", err)
	}
	if _, err := fmt.Fprintf(out, "%s\n", line); err != nil {
		return fmt.Errorf("campaigns: write task event: %w", err)
	}
	if name == "finished" || name == "error" {
		return taskStreamStopError{}
	}
	return nil
}
