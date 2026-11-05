package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newVersionsCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "versions", Short: "Manage workflow versions", SilenceUsage: true, SilenceErrors: true}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(newVersionGetCommand(f, &workflowID))
	cmd.AddCommand(newVersionRevertCommand(f, &workflowID))
	return cmd
}

func newVersionGetCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get VERSION --workflow ID", Short: "Get a workflow version (isLive pin, isDraft unpublished)", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionGet(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newVersionRevertCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "revert VERSION --workflow ID", Short: "Copy a published version into a new draft", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionRevert(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func runVersionGet(ctx context.Context, f *cli.Factory, workflowID, version string) error {
	path, err := versionResourcePath(workflowID, version)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeVersionEnvelope(resp.Body)
	if err != nil {
		return err
	}
	return printVersions(f, resp.Body, []WorkflowVersion{row})
}

func runVersionRevert(ctx context.Context, f *cli.Factory, workflowID, version string) error {
	path, err := versionResourcePath(workflowID, version)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":revert", nil, emptyObject())
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func versionResourcePath(workflowID, version string) (string, error) {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return "", err
	}
	version, err = parseVersionArg(version)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflowID, "versions", version), nil
}

func parseVersionArg(value string) (string, error) {
	value, err := requirePositional("version", value)
	if err != nil {
		return "", err
	}
	if _, err := strconv.ParseUint(value, 10, 32); err != nil {
		return "", &cli.UsageError{Msg: "version must be a non-negative integer"}
	}
	return value, nil
}

func decodeVersionEnvelope(raw []byte) (WorkflowVersion, error) {
	var payload struct {
		Version WorkflowVersion `json:"version"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WorkflowVersion{}, fmt.Errorf("workflows: decode version: %w", err)
	}
	return payload.Version, nil
}
