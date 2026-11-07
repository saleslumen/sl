package marketplace

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newOAuthClientsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "connection-oauth-clients", Short: "Manage connection OAuth clients", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newOAuthClientsUpsertCommand(f), newOAuthClientsGetCommand(f), newOAuthClientsDeleteCommand(f))
	return cmd
}

func newOAuthClientsUpsertCommand(f *cli.Factory) *cobra.Command {
	var appID, source string
	cmd := &cobra.Command{Use: "upsert CONNECTION_KEY --app APP_ID --input FILE", Short: "Create or replace an OAuth client", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOAuthClientsUpsert(cmd.Context(), f, appID, args[0], source)
	}
	return cmd
}

func newOAuthClientsGetCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "get CONNECTION_KEY --app APP_ID", Short: "Get an OAuth client", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOAuthClientsGet(cmd.Context(), f, appID, args[0])
	}
	return cmd
}

func newOAuthClientsDeleteCommand(f *cli.Factory) *cobra.Command {
	var appID string
	cmd := &cobra.Command{Use: "delete CONNECTION_KEY --app APP_ID", Short: "Delete an OAuth client", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&appID, "app", "", "App ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOAuthClientsDelete(cmd.Context(), f, appID, args[0])
	}
	return cmd
}

func runOAuthClientsUpsert(ctx context.Context, f *cli.Factory, appID, connectionKey, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, connectionKey, err := requireAppConnection(appID, connectionKey)
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
	raw, err := callRaw(ctx, f, http.MethodPut, oauthClientPath(appID, connectionKey), nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[oauthClient](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []oauthClient{row}, oauthClientColumns())
}

func runOAuthClientsGet(ctx context.Context, f *cli.Factory, appID, connectionKey string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, connectionKey, err := requireAppConnection(appID, connectionKey)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, oauthClientPath(appID, connectionKey), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[oauthClient](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []oauthClient{row}, oauthClientColumns())
}

func runOAuthClientsDelete(ctx context.Context, f *cli.Factory, appID, connectionKey string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	appID, connectionKey, err := requireAppConnection(appID, connectionKey)
	if err != nil {
		return err
	}
	if err := confirm(f, "Delete OAuth client "+connectionKey+"?"); err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodDelete, oauthClientPath(appID, connectionKey), nil, nil)
	if err != nil {
		return err
	}
	return printObject(f, raw)
}

func requireAppConnection(appID, connectionKey string) (string, string, error) {
	appID, err := requireID(appID, "app ID")
	if err != nil {
		return "", "", err
	}
	connectionKey, err = requireID(connectionKey, "connection key")
	if err != nil {
		return "", "", err
	}
	return appID, connectionKey, nil
}
