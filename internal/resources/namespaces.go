package resources

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type namespace struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	UpdatedAt      string `json:"updated_at"`
}

func newNamespacesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "namespaces", Short: "Manage namespaces", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newNamespacesListCommand(f), newNamespacesCreateCommand(f), newNamespacesDeleteCommand(f))
	return cmd
}

func newNamespacesListCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List namespaces", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNamespacesList(cmd.Context(), f)
	}
	return cmd
}

func newNamespacesCreateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create a namespace", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNamespacesCreate(cmd.Context(), f, source)
	}
	return cmd
}

func newNamespacesDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a namespace", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNamespacesDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func runNamespacesList(ctx context.Context, f *cli.Factory) error {
	org, err := resolveOrganization(f)
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doResources(ctx, client, http.MethodGet, namespacesPath(org), nil, nil)
	if err != nil {
		return err
	}
	rows, err := decodeArray[namespace](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, rows, namespaceColumns())
}

func runNamespacesCreate(ctx context.Context, f *cli.Factory, source string) error {
	org, err := resolveOrganization(f)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
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
	resp, err := doResources(ctx, client, http.MethodPost, namespacesPath(org), nil, raw)
	if err != nil {
		return err
	}
	row, err := decodeObject[namespace](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []namespace{row}, namespaceColumns())
}

func runNamespacesDelete(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireID(id, "namespace ID")
	if err != nil {
		return err
	}
	org, err := resolveOrganization(f)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete namespace " + id + "?"); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doResources(ctx, client, http.MethodDelete, namespacePath(org, id), nil, nil)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return nil
	}
	return f.Printer().Object(resp.Body)
}

func namespaceColumns() []output.Column[namespace] {
	return []output.Column[namespace]{
		{Header: "ID", Value: func(n namespace) string { return n.ID }},
		{Header: "ORGANIZATION", Value: func(n namespace) string { return n.OrganizationID }},
		{Header: "NAME", Value: func(n namespace) string { return n.Name }},
		{Header: "DESCRIPTION", Value: func(n namespace) string { return n.Description }},
		{Header: "UPDATED_AT", Value: func(n namespace) string { return n.UpdatedAt }},
	}
}
