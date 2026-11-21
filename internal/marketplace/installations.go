package marketplace

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newInstallationsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "installations", Short: "Manage installations", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(
		newInstallationsCreateCommand(f),
		newInstallationsListCommand(f),
		newInstallationsGetCommand(f),
		newInstallationsUpdateCommand(f),
		newInstallationsUninstallCommand(f),
		newInstallationsApproveUpdateCommand(f),
		newInstallationsUpdateConsentCommand(f),
	)
	return cmd
}

func newInstallationsCreateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create an installation", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsCreate(cmd.Context(), f, source)
	}
	return cmd
}

func newInstallationsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var filter string
	cmd := &cobra.Command{Use: "list", Short: "List installations", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum installations to return; 0 returns every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsList(cmd.Context(), f, limit, filter)
	}
	return cmd
}

func newInstallationsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get INSTALLATION_ID", Short: "Get an installation", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newInstallationsUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, mask string
	cmd := &cobra.Command{Use: "update INSTALLATION_ID --update-mask FIELDS --input FILE", Short: "Update an installation", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON installation body file, or - for stdin")
	cmd.Flags().StringVar(&mask, "update-mask", "", "Comma-separated installation field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsUpdate(cmd.Context(), f, args[0], source, mask)
	}
	return cmd
}

func newInstallationsUninstallCommand(f *cli.Factory) *cobra.Command {
	var retention string
	cmd := &cobra.Command{Use: "uninstall INSTALLATION_ID --retention-choice CHOICE", Short: "Uninstall an installation", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&retention, "retention-choice", "", "RETAIN_DATA or DELETE_DATA")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsUninstall(cmd.Context(), f, args[0], retention)
	}
	return cmd
}

func newInstallationsApproveUpdateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "approve-update INSTALLATION_ID --input FILE", Short: "Approve an installation update", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsApproveUpdate(cmd.Context(), f, args[0], source)
	}
	return cmd
}

func newInstallationsUpdateConsentCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "update-consent INSTALLATION_ID", Short: "Get installation update consent", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationsUpdateConsent(cmd.Context(), f, args[0])
	}
	return cmd
}

func newInstallationOperationsCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "installation-operations", Short: "List installation operations", SilenceUsage: true, SilenceErrors: true}
	list := &cobra.Command{Use: "list --installation INSTALLATION_ID", Short: "List operations on an installation", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	list.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	list.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationOperationsList(cmd.Context(), f, installationID)
	}
	cmd.AddCommand(list)
	return cmd
}

func newInstallationConfigurationCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "installation-configuration", Short: "Manage installation configuration", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newInstallationConfigurationGetCommand(f), newInstallationConfigurationUpdateCommand(f))
	return cmd
}

func newInstallationConfigurationGetCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "get --installation INSTALLATION_ID", Short: "Get installation configuration", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationConfigurationGet(cmd.Context(), f, installationID)
	}
	return cmd
}

func newInstallationConfigurationUpdateCommand(f *cli.Factory) *cobra.Command {
	var installationID, source, etag string
	cmd := &cobra.Command{Use: "update --installation INSTALLATION_ID --input FILE", Short: "Update installation configuration", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&etag, "etag", "", "Optimistic concurrency token")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallationConfigurationUpdate(cmd.Context(), f, installationID, source, etag, cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newConnectionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "connections", Short: "Manage installation connections", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(
		newConnectionsListCommand(f),
		newConnectionsGetCommand(f),
		newConnectionsAuthorizeCommand(f),
		newConnectionsUnbindCommand(f),
		newConnectionsSetAPIKeyCommand(f),
	)
	return cmd
}

func newConnectionsListCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "list --installation INSTALLATION_ID", Short: "List installation connections", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsList(cmd.Context(), f, installationID)
	}
	return cmd
}

func newConnectionsGetCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "get CONNECTION_KEY --installation INSTALLATION_ID", Short: "Get an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsGet(cmd.Context(), f, installationID, args[0])
	}
	return cmd
}

func newConnectionsAuthorizeCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "authorize CONNECTION_KEY --installation INSTALLATION_ID", Short: "Start connection authorization", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsAuthorize(cmd.Context(), f, installationID, args[0])
	}
	return cmd
}

func newConnectionsUnbindCommand(f *cli.Factory) *cobra.Command {
	var installationID string
	cmd := &cobra.Command{Use: "unbind CONNECTION_KEY --installation INSTALLATION_ID", Short: "Unbind an installation connection", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&installationID, "installation", "", "Installation ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsUnbind(cmd.Context(), f, installationID, args[0])
	}
	return cmd
}

func runInstallationsCreate(ctx context.Context, f *cli.Factory, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	raw, err := postBody(ctx, f, http.MethodPost, "/v1/installations", source)
	if err != nil {
		return err
	}
	return printInstallation(f, raw)
}

func runInstallationsList(ctx context.Context, f *cli.Factory, limit int, filter string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	rows, raw, err := listPages[installation](ctx, f, "/v1/installations", limit, filter, "", "installations")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, installationColumns())
}

func runInstallationsGet(ctx context.Context, f *cli.Factory, id string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "installation ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, installationPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printInstallation(f, raw)
}

func runInstallationsUpdate(ctx context.Context, f *cli.Factory, id, source, mask string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "installation ID")
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
	raw, err := callRaw(ctx, f, http.MethodPatch, installationPath(id), query, rawBody)
	if err != nil {
		return err
	}
	return printInstallation(f, raw)
}

func runInstallationsUninstall(ctx context.Context, f *cli.Factory, id, retention string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "installation ID")
	if err != nil {
		return err
	}
	if retention != "RETAIN_DATA" && retention != "DELETE_DATA" {
		return &cli.UsageError{Msg: "--retention-choice must be RETAIN_DATA or DELETE_DATA"}
	}
	if err := confirm(f, "Uninstall installation "+id+"?"); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("retentionChoice", retention)
	raw, err := callRaw(ctx, f, http.MethodDelete, installationPath(id), query, nil)
	if err != nil {
		return err
	}
	return printObject(f, raw)
}

func runInstallationsApproveUpdate(ctx context.Context, f *cli.Factory, id, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "installation ID")
	if err != nil {
		return err
	}
	raw, err := postBody(ctx, f, http.MethodPost, installationPath(id)+":approveUpdate", source)
	if err != nil {
		return err
	}
	return printInstallation(f, raw)
}

func runInstallationsUpdateConsent(ctx context.Context, f *cli.Factory, id string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	id, err := requireID(id, "installation ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, installationPath(id)+"/updateConsent", nil, nil)
	if err != nil {
		return err
	}
	return printObject(f, raw)
}

func runInstallationOperationsList(ctx context.Context, f *cli.Factory, installationID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	rows, raw, err := getRows[operationContract](ctx, f, installationPath(installationID)+"/operations", "operations")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, operationColumns())
}

func runInstallationConfigurationGet(ctx context.Context, f *cli.Factory, installationID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, installationPath(installationID)+"/configuration", nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[configuration](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []configuration{row}, configurationColumns())
}

func runInstallationConfigurationUpdate(ctx context.Context, f *cli.Factory, installationID, source, etag string, etagChanged bool) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("etag", "etag", etag, etagChanged); err != nil {
		return err
	}
	rawBody, err := body.Encode()
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPatch, installationPath(installationID)+"/configuration", nil, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[configuration](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []configuration{row}, configurationColumns())
}

func runConnectionsList(ctx context.Context, f *cli.Factory, installationID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return err
	}
	rows, raw, err := getRows[connection](ctx, f, installationPath(installationID)+"/connections", "connections")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, connectionColumns())
}

func runConnectionsGet(ctx context.Context, f *cli.Factory, installationID, connectionKey string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionKey, err := requireInstallationConnection(installationID, connectionKey)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, connectionPath(installationID, connectionKey), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[connection](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []connection{row}, connectionColumns())
}

func runConnectionsAuthorize(ctx context.Context, f *cli.Factory, installationID, connectionKey string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionKey, err := requireInstallationConnection(installationID, connectionKey)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, connectionPath(installationID, connectionKey)+":initiateAuthorization", nil, []byte("{}"))
	if err != nil {
		return err
	}
	row, err := decodeObject[authorizationStart](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []authorizationStart{row}, authorizationColumns())
}

func runConnectionsUnbind(ctx context.Context, f *cli.Factory, installationID, connectionKey string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionKey, err := requireInstallationConnection(installationID, connectionKey)
	if err != nil {
		return err
	}
	if err := confirm(f, "Unbind connection "+connectionKey+"?"); err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, connectionPath(installationID, connectionKey)+":unbind", nil, []byte("{}"))
	if err != nil {
		return err
	}
	row, err := decodeObject[connection](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []connection{row}, connectionColumns())
}

func printInstallation(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[installation](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []installation{row}, installationColumns())
}

func requireInstallationConnection(installationID, connectionKey string) (string, string, error) {
	installationID, err := requireID(installationID, "installation ID")
	if err != nil {
		return "", "", err
	}
	connectionKey, err = requireID(connectionKey, "connection key")
	if err != nil {
		return "", "", err
	}
	return installationID, connectionKey, nil
}
