package marketplace

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newAppCommands(f *cli.Factory) []*cobra.Command {
	return []*cobra.Command{
		newAppsListCommand(f),
		newAppsGetCommand(f),
		newAppsCreateCommand(f),
		newAppsUpdateCommand(f),
		newAppsActionCommand(f, "rollback", "Roll back an app", "Roll back app "),
		newAppsActionCommand(f, "suspend", "Suspend an app", "Suspend app "),
		newAppsActionCommand(f, "unpublish", "Unpublish an app", "Unpublish app "),
	}
}

func newAppsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var filter, orderBy string
	cmd := &cobra.Command{Use: "list", Short: "List catalog apps", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum apps to return; 0 returns every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort order")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsList(cmd.Context(), f, limit, filter, orderBy)
	}
	return cmd
}

func newAppsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get APP_ID", Short: "Get a catalog app", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newAppsCreateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create an app", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsCreate(cmd.Context(), f, source)
	}
	return cmd
}

func newAppsUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, mask string
	cmd := &cobra.Command{Use: "update APP_ID --update-mask FIELDS --input FILE", Short: "Update an app", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON app body file, or - for stdin")
	cmd.Flags().StringVar(&mask, "update-mask", "", "Comma-separated app field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsUpdate(cmd.Context(), f, args[0], source, mask)
	}
	return cmd
}

func newAppsActionCommand(f *cli.Factory, use, short, confirmPrefix string) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: use + " APP_ID --input FILE", Short: short, Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppsAction(cmd.Context(), f, use, args[0], source, confirmPrefix)
	}
	return cmd
}

func newAppOperationsCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "operations", Short: "List app operations", SilenceUsage: true, SilenceErrors: true}
	list := &cobra.Command{Use: "list --app APP_ID", Short: "List operations on an app", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	list.Flags().StringVar(&appID, "app", "", "App ID")
	list.RunE = func(cmd *cobra.Command, args []string) error {
		return runAppOperationsList(cmd.Context(), f, appID)
	}
	cmd.AddCommand(list)
	return cmd
}

func newPublisherAppsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "publisher-apps", Short: "Manage publisher apps", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newPublisherAppsListCommand(f), newPublisherAppsGetCommand(f))
	return cmd
}

func newPublisherAppsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var filter string
	cmd := &cobra.Command{Use: "list", Short: "List publisher apps", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum apps to return; 0 returns every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublisherAppsList(cmd.Context(), f, limit, filter)
	}
	return cmd
}

func newPublisherAppsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get APP_ID", Short: "Get a publisher app", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runPublisherAppsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func runAppsList(ctx context.Context, f *cli.Factory, limit int, filter, orderBy string) error {
	rows, raw, err := listPages[catalogApp](ctx, f, "/v1/apps", limit, filter, orderBy, "apps")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, catalogColumns())
}

func runAppsGet(ctx context.Context, f *cli.Factory, id string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "app ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, appPath(id), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[catalogApp](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []catalogApp{row}, catalogColumns())
}

func runAppsCreate(ctx context.Context, f *cli.Factory, source string) error {
	return postApp(ctx, f, http.MethodPost, "/v1/apps", source)
}

func runAppsUpdate(ctx context.Context, f *cli.Factory, id, source, mask string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "app ID")
	if err != nil {
		return err
	}
	mask, err = requireID(mask, "--update-mask")
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if body.Has("updateMask") || body.Has("update_mask") {
		return &cli.UsageError{Msg: "--update-mask conflicts with updateMask in --input"}
	}
	rawBody, err := body.Encode()
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("updateMask", mask)
	raw, err := callRaw(ctx, f, http.MethodPatch, appPath(id), query, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[app](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []app{row}, appColumns())
}

func runAppsAction(ctx context.Context, f *cli.Factory, action, id, source, confirmPrefix string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "app ID")
	if err != nil {
		return err
	}
	if err := confirm(f, confirmPrefix+id+"?"); err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	rawBody, err := body.Encode()
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, appPath(id)+":"+action, nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[app](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []app{row}, appColumns())
}

func postApp(ctx context.Context, f *cli.Factory, method, path, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	rawBody, err := body.Encode()
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, method, path, nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[app](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []app{row}, appColumns())
}

func runAppOperationsList(ctx context.Context, f *cli.Factory, appID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	rows, raw, err := getRows[operationContract](ctx, f, appPath(appID)+"/operations", "operations")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, operationColumns())
}

func runPublisherAppsList(ctx context.Context, f *cli.Factory, limit int, filter string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	rows, raw, err := listPages[app](ctx, f, "/v1/publisherApps", limit, filter, "", "apps")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, appColumns())
}

func runPublisherAppsGet(ctx context.Context, f *cli.Factory, id string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "app ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, publisherAppPath(id), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[app](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []app{row}, appColumns())
}
