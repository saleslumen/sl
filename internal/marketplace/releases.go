package marketplace

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newReleasesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "releases", Short: "Manage releases", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newReleasesCreateCommand(f), newReleasesListCommand(f), newReleasesGetCommand(f), newReleasesRevokeCommand(f))
	return cmd
}

func newReleasesCreateCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "create --app APP_ID --input FILE", Short: "Create a release", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReleasesCreate(cmd.Context(), f, appID, source)
	}
	return cmd
}

func newReleasesListCommand(f *cli.Factory) *cobra.Command {
	var appID string
	var limit int
	cmd := &cobra.Command{Use: "list --app APP_ID", Short: "List releases", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum releases to return; 0 returns every page")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReleasesList(cmd.Context(), f, appID, limit)
	}
	return cmd
}

func newReleasesGetCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "get RELEASE_ID --app APP_ID", Short: "Get a release", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReleasesGet(cmd.Context(), f, appID, args[0])
	}
	return cmd
}

func newReleasesRevokeCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "revoke RELEASE_ID --app APP_ID --input FILE", Short: "Revoke a release", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReleasesRevoke(cmd.Context(), f, appID, args[0], source)
	}
	return cmd
}

func runReleasesCreate(ctx context.Context, f *cli.Factory, appID, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	raw, err := postBody(ctx, f, http.MethodPost, appPath(appID)+"/releases", source)
	if err != nil {
		return err
	}
	row, err := decodeObject[release](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []release{row}, releaseColumns())
}

func runReleasesList(ctx context.Context, f *cli.Factory, appID string, limit int) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	rows, raw, err := listPages[release](ctx, f, appPath(appID)+"/releases", limit, "", "", "releases")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, releaseColumns())
}

func runReleasesGet(ctx context.Context, f *cli.Factory, appID, releaseID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, releaseID, err := requireAppRelease(appID, releaseID)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, releasePath(appID, releaseID), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[release](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []release{row}, releaseColumns())
}

func runReleasesRevoke(ctx context.Context, f *cli.Factory, appID, releaseID, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, releaseID, err := requireAppRelease(appID, releaseID)
	if err != nil {
		return err
	}
	if err := confirm(f, "Revoke release "+releaseID+"?"); err != nil {
		return err
	}
	raw, err := postBody(ctx, f, http.MethodPost, releasePath(appID, releaseID)+":revoke", source)
	if err != nil {
		return err
	}
	row, err := decodeObject[release](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []release{row}, releaseColumns())
}

func requireAppRelease(appID, releaseID string) (string, string, error) {
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return "", "", err
	}
	releaseID, err = requireID(releaseID, "release ID")
	if err != nil {
		return "", "", err
	}
	return appID, releaseID, nil
}

func postBody(ctx context.Context, f *cli.Factory, method, path, source string) ([]byte, error) {
	body, err := readRequiredObject(f, source)
	if err != nil {
		return nil, err
	}
	rawBody, err := body.Encode()
	if err != nil {
		return nil, err
	}
	return callRaw(ctx, f, method, path, nil, rawBody)
}
