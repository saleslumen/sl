package script

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

// scriptOperation is the RunScript Operation. A nil VersionNumber is unresolved;
// a pointer to 0 is the draft.
type scriptOperation struct {
	Done          bool            `json:"done"`
	VersionNumber *int32          `json:"versionNumber"`
	ReleaseID     string          `json:"releaseId"`
	Error         json.RawMessage `json:"error"`
}

func newRunCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "run SCRIPT_ID --input FILE", Short: "Run a script", Long: "Run a script. Accepts a user or an API key.", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runScript(cmd.Context(), f, args[0], source)
	}
	return cmd
}

func runScript(ctx context.Context, f *cli.Factory, scriptID, source string) error {
	scriptID = strings.TrimSpace(scriptID)
	if scriptID == "" {
		return &cli.UsageError{Msg: "script ID is required"}
	}
	if source == "" {
		return &cli.UsageError{Msg: "--input is required"}
	}
	body, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPost, scriptRunPath(scriptID), nil, raw)
	if err != nil {
		return err
	}
	return printOperation(f, resp.Body)
}

func printOperation(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[scriptOperation](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []scriptOperation{row}, operationColumns())
}

func operationColumns() []output.Column[scriptOperation] {
	return []output.Column[scriptOperation]{
		{Header: "DONE", Value: func(op scriptOperation) string { return strconv.FormatBool(op.Done) }},
		{Header: "VERSION_NUMBER", Value: func(op scriptOperation) string { return formatVersionNumber(op.VersionNumber) }},
		{Header: "RELEASE_ID", Value: func(op scriptOperation) string { return op.ReleaseID }},
		{Header: "ERROR", Value: func(op scriptOperation) string { return operationErrorMessage(op.Error) }},
	}
}

func formatVersionNumber(n *int32) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(int64(*n), 10)
}

func operationErrorMessage(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return ""
	}
	message, ok := fields["message"]
	if !ok {
		return ""
	}
	var text string
	if json.Unmarshal(message, &text) != nil {
		return ""
	}
	return text
}

func scriptRunPath(id string) string {
	return "/v1/scripts/" + url.PathEscape(id) + ":run"
}
