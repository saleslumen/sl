package marketplace

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newSubmissionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "submissions", Short: "Manage submissions", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(
		newSubmissionsCreateCommand(f),
		newSubmissionsListCommand(f),
		newSubmissionsGetCommand(f),
		newSubmissionsWithdrawCommand(f),
		newSubmissionsApproveCommand(f),
		newSubmissionsRequestChangesCommand(f),
		newSubmissionsRejectCommand(f),
	)
	return cmd
}

func newSubmissionsCreateCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "create --app APP_ID --input FILE", Short: "Create a submission", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsCreate(cmd.Context(), f, appID, source)
	}
	return cmd
}

func newSubmissionsListCommand(f *cli.Factory) *cobra.Command {
	var appID string
	var limit int
	cmd := &cobra.Command{Use: "list --app APP_ID", Short: "List submissions", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum submissions to return; 0 returns every page")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsList(cmd.Context(), f, appID, limit)
	}
	return cmd
}

func newSubmissionsGetCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "get SUBMISSION_ID --app APP_ID", Short: "Get a submission", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsGet(cmd.Context(), f, appID, args[0])
	}
	return cmd
}

func newSubmissionsWithdrawCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "withdraw SUBMISSION_ID --app APP_ID", Short: "Withdraw a submission", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsAction(cmd.Context(), f, appID, args[0], "withdraw", "Withdraw submission ", true, "")
	}
	return cmd
}

func newSubmissionsApproveCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "approve SUBMISSION_ID --app APP_ID", Short: "Approve a submission", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "Optional JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsAction(cmd.Context(), f, appID, args[0], "approve", "", false, source)
	}
	return cmd
}

func newSubmissionsRequestChangesCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "request-changes SUBMISSION_ID --app APP_ID --input FILE", Short: "Request submission changes", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsAction(cmd.Context(), f, appID, args[0], "requestChanges", "", true, source)
	}
	return cmd
}

func newSubmissionsRejectCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "reject SUBMISSION_ID --app APP_ID --input FILE", Short: "Reject a submission", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSubmissionsAction(cmd.Context(), f, appID, args[0], "reject", "Reject submission ", true, source)
	}
	return cmd
}

func runSubmissionsCreate(ctx context.Context, f *cli.Factory, appID, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	raw, err := postBody(ctx, f, http.MethodPost, appPath(appID)+"/submissions", source)
	if err != nil {
		return err
	}
	return printSubmission(f, raw)
}

func runSubmissionsList(ctx context.Context, f *cli.Factory, appID string, limit int) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	rows, raw, err := listPages[submission](ctx, f, appPath(appID)+"/submissions", limit, "", "", "submissions")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, submissionColumns())
}

func runSubmissionsGet(ctx context.Context, f *cli.Factory, appID, submissionID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, submissionID, err := requireAppSubmission(appID, submissionID)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, submissionPath(appID, submissionID), nil, nil)
	if err != nil {
		return err
	}
	return printSubmission(f, raw)
}

func runSubmissionsAction(ctx context.Context, f *cli.Factory, appID, submissionID, action, confirmPrefix string, bodyRequired bool, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, submissionID, err := requireAppSubmission(appID, submissionID)
	if err != nil {
		return err
	}
	if confirmPrefix != "" {
		if err := confirm(f, confirmPrefix+submissionID+"?"); err != nil {
			return err
		}
	}
	var rawBody any
	if bodyRequired && action != "withdraw" {
		body, err := readRequiredObject(f, source)
		if err != nil {
			return err
		}
		encoded, err := body.Encode()
		if err != nil {
			return err
		}
		rawBody = encoded
	} else if source != "" {
		body, err := readOptionalObject(f, source)
		if err != nil {
			return err
		}
		encoded, err := body.Encode()
		if err != nil {
			return err
		}
		rawBody = encoded
	} else {
		rawBody = json.RawMessage(`{}`)
	}
	raw, err := callRaw(ctx, f, http.MethodPost, submissionPath(appID, submissionID)+":"+action, nil, rawBody)
	if err != nil {
		return err
	}
	return printSubmission(f, raw)
}

func printSubmission(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[submission](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []submission{row}, submissionColumns())
}

func requireAppSubmission(appID, submissionID string) (string, string, error) {
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return "", "", err
	}
	submissionID, err = requireID(submissionID, "submission ID")
	if err != nil {
		return "", "", err
	}
	return appID, submissionID, nil
}
