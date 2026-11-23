package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newConnectionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "connections", Short: "Manage installation connections", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(
		newConnectionsCreateCommand(f),
		newConnectionsGetCommand(f),
		newConnectionsListCommand(f),
		newConnectionsUpdateCommand(f),
		newConnectionsDeleteCommand(f),
		newConnectionsAuthorizeCommand(f),
		newConnectionsUnbindCommand(f),
		newConnectionsSetAPIKeyCommand(f),
	)
	return cmd
}

func newConnectionsCreateCommand(f *cli.Factory) *cobra.Command {
	var installationID, connectionKey, displayName string
	cmd := &cobra.Command{Use: "create --installation ID --connection-key KEY --display-name NAME", Short: "Create an installation connection", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.Flags().StringVar(&connectionKey, "connection-key", "", "Declared connection key")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Connection display name")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsCreate(cmd.Context(), f, installationID, connectionKey, displayName)
	}
	return cmd
}

func newConnectionsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get CONNECTION", Short: "Get an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newConnectionsListCommand(f *cli.Factory) *cobra.Command {
	var installationID, connectionKey string
	cmd := &cobra.Command{Use: "list --installation ID [--connection-key KEY]", Short: "List installation connections", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.Flags().StringVar(&connectionKey, "connection-key", "", "Declared connection key")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsList(cmd.Context(), f, installationID, connectionKey, cmd.Flags().Changed("connection-key"))
	}
	return cmd
}

func newConnectionsUpdateCommand(f *cli.Factory) *cobra.Command {
	var displayName string
	cmd := &cobra.Command{Use: "update CONNECTION --display-name NAME", Short: "Rename an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&displayName, "display-name", "", "Connection display name")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsUpdate(cmd.Context(), f, args[0], displayName)
	}
	return cmd
}

func newConnectionsDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete CONNECTION", Short: "Delete an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func newConnectionsAuthorizeCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "authorize CONNECTION", Short: "Start connection authorization", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsAuthorize(cmd.Context(), f, args[0])
	}
	return cmd
}

func newConnectionsUnbindCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "unbind CONNECTION", Short: "Unbind an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsUnbind(cmd.Context(), f, args[0])
	}
	return cmd
}

func runConnectionsCreate(ctx context.Context, f *cli.Factory, installationID, connectionKey, displayName string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	connectionKey, err = requireID(connectionKey, "connection key")
	if err != nil {
		return err
	}
	displayName, err = requireID(displayName, "--display-name")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"connectionKey": connectionKey, "displayName": displayName})
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, installationPath(installationID)+"/connections", nil, body)
	if err != nil {
		return err
	}
	return printConnection(f, raw)
}

func runConnectionsGet(ctx context.Context, f *cli.Factory, name string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, connectionResourcePath(installationID, connectionID), nil, nil)
	if err != nil {
		return err
	}
	return printConnection(f, raw)
}

func runConnectionsList(ctx context.Context, f *cli.Factory, installationID, connectionKey string, keySet bool) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	filter := ""
	if keySet {
		connectionKey, err = requireID(connectionKey, "connection key")
		if err != nil {
			return err
		}
		filter = `connection_key = "` + connectionKey + `"`
	}
	rows, raw, err := listPages[connection](ctx, f, installationPath(installationID)+"/connections", 0, filter, "", "connections")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, connectionColumns())
}

func runConnectionsUpdate(ctx context.Context, f *cli.Factory, name, displayName string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	displayName, err = requireID(displayName, "--display-name")
	if err != nil {
		return err
	}
	path := connectionResourcePath(installationID, connectionID)
	current, err := callRaw(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[connection](current)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"displayName": displayName, "etag": row.Etag})
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("updateMask", "display_name")
	raw, err := callRaw(ctx, f, http.MethodPatch, path, query, body)
	if err != nil {
		return err
	}
	return printConnection(f, raw)
}

func runConnectionsDelete(ctx context.Context, f *cli.Factory, name string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	if err := confirm(f, "Delete connection "+connectionResourceName(installationID, connectionID)+"?"); err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodDelete, connectionResourcePath(installationID, connectionID), nil, nil)
	if err != nil {
		return err
	}
	return printObject(f, raw)
}

func runConnectionsAuthorize(ctx context.Context, f *cli.Factory, name string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, connectionResourcePath(installationID, connectionID)+":initiateAuthorization", nil, []byte("{}"))
	if err != nil {
		return err
	}
	row, err := decodeObject[authorizationStart](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []authorizationStart{row}, authorizationColumns())
}

func runConnectionsUnbind(ctx context.Context, f *cli.Factory, name string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	resource := connectionResourceName(installationID, connectionID)
	if err := confirm(f, "Unbind connection "+resource+"?"); err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, connectionResourcePath(installationID, connectionID)+":unbind", nil, []byte("{}"))
	if err != nil {
		return err
	}
	return printConnection(f, raw)
}

func printConnection(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[connection](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []connection{row}, connectionColumns())
}
