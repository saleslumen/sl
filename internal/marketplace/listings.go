package marketplace

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newListingsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "listings", Short: "Manage listings", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newListingsCreateCommand(f), newListingsListCommand(f), newListingsGetCommand(f))
	return cmd
}

func newListingsCreateCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "create --app APP_ID --input FILE", Short: "Create a listing", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runListingsCreate(cmd.Context(), f, appID, source)
	}
	return cmd
}

func newListingsListCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "list --app APP_ID", Short: "List listings", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runListingsList(cmd.Context(), f, appID)
	}
	return cmd
}

func newListingsGetCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "get LISTING_ID --app APP_ID", Short: "Get a listing", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runListingsGet(cmd.Context(), f, appID, args[0])
	}
	return cmd
}

func newListingRevisionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "listing-revisions", Short: "Manage listing revisions", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newListingRevisionsGetCommand(f), newListingRevisionsUpdateCommand(f))
	return cmd
}

func newListingRevisionsGetCommand(f *cli.Factory) *cobra.Command {
	var appID, listingID string
	cmd := &cobra.Command{Use: "get REVISION_ID --app APP_ID --listing LISTING_ID", Short: "Get a listing revision", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&listingID, "listing", "", "Listing ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runListingRevisionsGet(cmd.Context(), f, appID, listingID, args[0])
	}
	return cmd
}

func newListingRevisionsUpdateCommand(f *cli.Factory) *cobra.Command {
	var appID, listingID, source, mask, etag string
	cmd := &cobra.Command{Use: "update REVISION_ID --app APP_ID --listing LISTING_ID --input FILE", Short: "Update a listing revision", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&listingID, "listing", "", "Listing ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&mask, "update-mask", "", "Comma-separated revision field paths")
	cmd.Flags().StringVar(&etag, "etag", "", "Optimistic concurrency token")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runListingRevisionsUpdate(cmd.Context(), f, appID, listingID, args[0], source, mask, cmd.Flags().Changed("update-mask"), etag, cmd.Flags().Changed("etag"))
	}
	return cmd
}

func runListingsCreate(ctx context.Context, f *cli.Factory, appID, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
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
	raw, err := callRaw(ctx, f, http.MethodPost, appPath(appID)+"/listings", nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[listing](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []listing{row}, listingColumns())
}

func runListingsList(ctx context.Context, f *cli.Factory, appID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	rows, raw, err := getRows[listing](ctx, f, appPath(appID)+"/listings", "listings")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, listingColumns())
}

func runListingsGet(ctx context.Context, f *cli.Factory, appID, listingID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return err
	}
	listingID, err = requireID(listingID, "listing ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, listingPath(appID, listingID), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[listing](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []listing{row}, listingColumns())
}

func runListingRevisionsGet(ctx context.Context, f *cli.Factory, appID, listingID, revisionID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, listingID, revisionID, err := requireListingRevision(appID, listingID, revisionID)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, revisionPath(appID, listingID, revisionID), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[listingRevision](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []listingRevision{row}, revisionColumns())
}

func runListingRevisionsUpdate(ctx context.Context, f *cli.Factory, appID, listingID, revisionID, source, mask string, maskChanged bool, etag string, etagChanged bool) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, listingID, revisionID, err := requireListingRevision(appID, listingID, revisionID)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("updateMask", "update-mask", mask, maskChanged); err != nil {
		return err
	}
	if err := body.MergeFlag("etag", "etag", etag, etagChanged); err != nil {
		return err
	}
	if err := requireMerged(body, "updateMask", "update-mask"); err != nil {
		return err
	}
	if err := requireMerged(body, "etag", "etag"); err != nil {
		return err
	}
	rawBody, err := body.Encode()
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPatch, revisionPath(appID, listingID, revisionID), nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[listingRevision](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []listingRevision{row}, revisionColumns())
}

func requireListingRevision(appID, listingID, revisionID string) (string, string, string, error) {
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return "", "", "", err
	}
	listingID, err = requireID(listingID, "listing ID")
	if err != nil {
		return "", "", "", err
	}
	revisionID, err = requireID(revisionID, "revision ID")
	if err != nil {
		return "", "", "", err
	}
	return appID, listingID, revisionID, nil
}
