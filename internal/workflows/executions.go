package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/spf13/cobra"
)

func newExecutionsCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "executions", Short: "Manage workflow executions", SilenceUsage: true, SilenceErrors: true, Args: groupArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(
		newExecutionsCreateCommand(f, &workflowID),
		newExecutionsListCommand(f, &workflowID),
		newExecutionsGetCommand(f, &workflowID),
		newExecutionsWatchCommand(f, &workflowID),
		newExecutionsCancelCommand(f, &workflowID),
		newExecutionsSignalCommand(f, &workflowID),
	)
	return cmd
}

func newExecutionsCreateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source, executionID, requestID string
	cmd := &cobra.Command{Use: "create --workflow ID --input FILE", Short: "Create a workflow execution", Long: "Create a workflow execution. A user or an API key may call this command.", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "Execution JSON body file, or - for stdin")
	cmd.Flags().StringVar(&executionID, "execution-id", "", "Caller-chosen execution ID")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Optional UUIDv4 idempotency key")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsCreate(cmd.Context(), f, *workflowID, source, executionID, requestID, cmd.Flags().Changed("execution-id"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newExecutionsListCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var limit int
	var filter string
	cmd := &cobra.Command{Use: "list --workflow ID", Short: "List workflow executions", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum executions to return; 0 follows every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter for execution state, create time, trigger source, or revision")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsList(cmd.Context(), f, *workflowID, limit, filter)
	}
	return cmd
}

func newExecutionsGetCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get EXECUTION_ID --workflow ID", Short: "Get a workflow execution", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsGet(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newExecutionsWatchCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "watch EXECUTION_ID --workflow ID", Short: "Watch workflow execution snapshots", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsWatch(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newExecutionsCancelCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "cancel EXECUTION_ID --workflow ID", Short: "Cancel a workflow execution", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsCancel(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newExecutionsSignalCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source, requestID, targetStepRun, executionUID, startRevision, startInput string
	var start bool
	cmd := &cobra.Command{Use: "signal EXECUTION_ID --workflow ID --input FILE", Short: "Signal a workflow execution", Long: "Signal a workflow execution. A user or an API key may call this command.", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&source, "input", "", "Signal JSON body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Optional UUIDv4 idempotency key, sent in the body")
	cmd.Flags().StringVar(&targetStepRun, "target-step-run", "", "Observed open step run resource name")
	cmd.Flags().StringVar(&executionUID, "execution-uid", "", "Observed execution UID")
	cmd.Flags().BoolVar(&start, "start", false, "Create the named execution when it is absent, then deliver the signal")
	cmd.Flags().StringVar(&startRevision, "start-revision", "", "Revision resource name used when --start creates the execution")
	cmd.Flags().StringVar(&startInput, "start-input", "", "JSON execution input used when --start creates the execution")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		return runExecutionsSignal(cmd.Context(), f, *workflowID, args[0], source, requestID, targetStepRun, executionUID, startRevision, startInput, start, flags.Changed("request-id"), flags.Changed("target-step-run"), flags.Changed("execution-uid"), flags.Changed("start"), flags.Changed("start-revision"), flags.Changed("start-input"))
	}
	return cmd
}

func runExecutionsCreate(ctx context.Context, f *cli.Factory, workflowID, source, executionID, requestID string, executionIDSet, requestIDSet bool) error {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, source)
	if err != nil {
		return err
	}
	query := url.Values{}
	if executionIDSet {
		query.Set("executionId", executionID)
	}
	if requestIDSet {
		query.Set("requestId", requestID)
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowSubpath(workflowID, "executions"), query, body)
	if err != nil {
		return err
	}
	return printExecution(f, resp.Body)
}

func runExecutionsList(ctx context.Context, f *cli.Factory, workflowID string, limit int, filter string) error {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	query := url.Values{}
	if filter != "" {
		query.Set("filter", filter)
	}
	items, raw, err := listPages[Execution](ctx, f, workflowSubpath(workflowID, "executions"), limit, maxPageSize, "executions", query)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, executionColumns())
}

func runExecutionsGet(ctx context.Context, f *cli.Factory, workflowID, executionID string) error {
	path, err := executionPath(workflowID, executionID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printExecution(f, resp.Body)
}

func runExecutionsWatch(ctx context.Context, f *cli.Factory, workflowID, executionID string) error {
	path, err := executionPath(workflowID, executionID)
	if err != nil {
		return err
	}
	expectedName, err := url.PathUnescape(strings.TrimPrefix(path, "/v1/"))
	if err != nil {
		return fmt.Errorf("workflows: invalid execution path: %w", err)
	}
	path += ":watch"
	client, err := f.Client()
	if err != nil {
		return err
	}
	terminal := false
	var uid string
	var version int64
	err = client.Stream(ctx, apiclient.Request{Product: productWorkflows, Method: http.MethodGet, Path: path}, func(name string, data []byte) error {
		switch name {
		case "execution":
			var snapshot struct {
				State   string `json:"state"`
				UID     string `json:"uid"`
				Name    string `json:"name"`
				Version string `json:"observationVersion"`
			}
			if err := json.Unmarshal(data, &snapshot); err != nil {
				return fmt.Errorf("workflows: invalid execution snapshot: %w", err)
			}
			if snapshot.State == "" || snapshot.UID == "" || snapshot.Name != expectedName {
				return fmt.Errorf("workflows: invalid execution snapshot identity or state")
			}
			current, parseErr := strconv.ParseInt(snapshot.Version, 10, 64)
			if parseErr != nil || current <= 0 {
				return fmt.Errorf("workflows: invalid execution observation version")
			}
			if uid != "" && snapshot.UID != uid {
				return fmt.Errorf("workflows: execution identity changed during watch")
			}
			uid = snapshot.UID
			if current <= version {
				return nil
			}
			version = current
			if err := f.Printer().StreamObject(data); err != nil {
				return fmt.Errorf("workflows: print: %w", err)
			}
			switch snapshot.State {
			case "STATE_SUCCEEDED", "STATE_FAILED", "STATE_CANCELLED", "STATE_REJECTED":
				terminal = true
			}
			return nil
		case "error":
			return apiclient.StreamStatusError(data, productWorkflows, http.MethodGet, path)
		default:
			return fmt.Errorf("workflows: unexpected watch event %q", name)
		}
	})
	if err != nil {
		return err
	}
	if !terminal {
		return fmt.Errorf("workflows: watch interrupted before a terminal snapshot; rerun watch to reconnect")
	}
	return nil
}

func runExecutionsCancel(ctx context.Context, f *cli.Factory, workflowID, executionID string) error {
	path, err := executionPath(workflowID, executionID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":cancel", nil, map[string]any{})
	if err != nil {
		return err
	}
	return printExecution(f, resp.Body)
}

func runExecutionsSignal(ctx context.Context, f *cli.Factory, workflowID, executionID, source, requestID, targetStepRun, executionUID, startRevision, startInput string, start bool, requestIDSet, targetSet, uidSet, startSet, startRevisionSet, startInputSet bool) error {
	path, err := executionPath(workflowID, executionID)
	if err != nil {
		return err
	}
	if source == "" {
		return &cli.UsageError{Msg: "--input is required"}
	}
	bodyObject, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := bodyObject.MergeFlag("requestId", "request-id", requestID, requestIDSet); err != nil {
		return err
	}
	if err := bodyObject.MergeFlag("targetStepRun", "target-step-run", targetStepRun, targetSet); err != nil {
		return err
	}
	if err := bodyObject.MergeFlag("executionUid", "execution-uid", executionUID, uidSet); err != nil {
		return err
	}
	if err := applySignalStart(bodyObject, start, startRevision, startInput, startSet, startRevisionSet, startInputSet, targetSet, uidSet); err != nil {
		return err
	}
	body, err := bodyObject.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":signal", nil, body)
	if err != nil {
		return annotateSignalRejection(err)
	}
	return printExecution(f, resp.Body)
}

func applySignalStart(body input.Object, start bool, startRevision, startInput string, startSet, startRevisionSet, startInputSet, targetSet, uidSet bool) error {
	if (startRevisionSet || startInputSet) && !start {
		return &cli.UsageError{Msg: "--start-revision and --start-input require --start"}
	}
	if start && (targetSet || uidSet) {
		return &cli.UsageError{Msg: "--start cannot be combined with --target-step-run or --execution-uid"}
	}
	if (startSet || startRevisionSet || startInputSet) && body.Has("start") {
		return body.MergeFlag("start", "start", map[string]any{}, true)
	}
	if !start {
		return nil
	}
	startObject := map[string]any{}
	if startRevisionSet {
		startObject["revision"] = startRevision
	}
	if startInputSet {
		value, err := parseJSONValue(startInput)
		if err != nil {
			return err
		}
		startObject["input"] = value
	}
	return body.MergeFlag("start", "start", startObject, true)
}

func parseJSONValue(raw string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, &cli.UsageError{Msg: "--start-input must be JSON"}
	}
	return value, nil
}

func annotateSignalRejection(err error) error {
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	suffix, ok := signalRejectionSuffix(apiErr.Details)
	if !ok {
		return err
	}
	apiErr.Message += replaceControls(suffix)
	return err
}

func signalRejectionSuffix(details json.RawMessage) (string, bool) {
	if len(details) == 0 {
		return "", false
	}
	var items []struct {
		Reason   string            `json:"reason"`
		Metadata map[string]string `json:"metadata"`
	}
	if json.Unmarshal(details, &items) != nil {
		return "", false
	}
	for _, item := range items {
		if item.Reason != "SIGNAL_REJECTED" {
			continue
		}
		parts := []string{"SIGNAL_REJECTED"}
		if code := item.Metadata["error_code"]; code != "" {
			parts = append(parts, "error_code="+code)
		}
		if reason := item.Metadata["error_reason"]; reason != "" {
			parts = append(parts, "error_reason="+reason)
		}
		return " (" + strings.Join(parts, " ") + ")", true
	}
	return "", false
}

func replaceControls(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

func executionPath(workflowID, executionID string) (string, error) {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return "", err
	}
	executionID, err = requirePositional("execution ID", executionID)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflowID, "executions", executionID), nil
}
