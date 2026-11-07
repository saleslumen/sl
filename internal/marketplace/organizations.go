package marketplace

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newOrganizationPolicyCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "organization-policy", Short: "Manage Marketplace organization policy", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newOrganizationPolicyGetCommand(f), newOrganizationPolicyUpdateCommand(f))
	return cmd
}

func newOrganizationPolicyGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get", Short: "Get Marketplace organization policy", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationPolicyGet(cmd.Context(), f)
	}
	return cmd
}

func newOrganizationPolicyUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, mask string
	cmd := &cobra.Command{Use: "update --update-mask FIELDS --input FILE", Short: "Update Marketplace organization policy", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON policy body file, or - for stdin")
	cmd.Flags().StringVar(&mask, "update-mask", "", "Comma-separated policy field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationPolicyUpdate(cmd.Context(), f, source, mask)
	}
	return cmd
}

func newInstallCapabilitiesCommand(f *cli.Factory) *cobra.Command {
	var namespaceID string
	cmd := &cobra.Command{Use: "install-capabilities", Short: "Read install capabilities", SilenceUsage: true, SilenceErrors: true}
	get := &cobra.Command{Use: "get", Short: "Get install capabilities", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	get.Flags().StringVar(&namespaceID, "namespace-id", "", "Namespace to project")
	get.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstallCapabilitiesGet(cmd.Context(), f, namespaceID)
	}
	cmd.AddCommand(get)
	return cmd
}

func runOrganizationPolicyGet(ctx context.Context, f *cli.Factory) error {
	if err := requireUser(f); err != nil {
		return err
	}
	org, err := resolveOrganization(f)
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, organizationPolicyPath(org), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[organizationPolicy](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []organizationPolicy{row}, policyColumns())
}

func runOrganizationPolicyUpdate(ctx context.Context, f *cli.Factory, source, mask string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	org, err := resolveOrganization(f)
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
	raw, err := callRaw(ctx, f, http.MethodPatch, organizationPolicyPath(org), query, rawBody)
	if err != nil {
		return err
	}
	row, err := decodeObject[organizationPolicy](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []organizationPolicy{row}, policyColumns())
}

func runInstallCapabilitiesGet(ctx context.Context, f *cli.Factory, namespaceID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	org, err := resolveOrganization(f)
	if err != nil {
		return err
	}
	query := url.Values{}
	if namespaceID != "" {
		query.Set("namespaceId", namespaceID)
	}
	raw, err := callRaw(ctx, f, http.MethodGet, installCapabilitiesPath(org), query, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[installCapabilities](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []installCapabilities{row}, capabilityColumns())
}
