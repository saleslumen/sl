package script

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/spf13/cobra"
)

func newRunCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "run SCRIPT_ID --input FILE", Short: "Run a script", Long: "Run a script. Requires user OAuth (`sl auth login`).", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runScript(cmd.Context(), f, args[0], source)
	}
	return cmd
}

func runScript(ctx context.Context, f *cli.Factory, scriptID, source string) error {
	if err := f.RequireUserOAuth(); err != nil {
		return err
	}
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
	return f.Printer().Object(resp.Body)
}

func scriptRunPath(id string) string {
	return "/v1/scripts/" + url.PathEscape(id) + ":run"
}
