package resources

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type organization struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Domain            string `json:"domain"`
	Industry          string `json:"industry"`
	EmployeeCount     string `json:"employee_count"`
	PreferredLanguage string `json:"preferred_language"`
	Country           string `json:"country"`
	UpdatedAt         string `json:"updated_at"`
}

type organizationProvision struct {
	OperationID    string `json:"operationId"`
	OrganizationID string `json:"organizationId"`
	State          string `json:"state"`
}

func newOrganizationsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "organizations", Short: "Manage organizations", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newOrganizationsListCommand(f), newOrganizationsCreateCommand(f), newOrganizationsGetCommand(f))
	return cmd
}

func newOrganizationsListCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List organizations", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationsList(cmd.Context(), f)
	}
	return cmd
}

func newOrganizationsCreateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create an organization", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationsCreate(cmd.Context(), f, source)
	}
	return cmd
}

func newOrganizationsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get an organization", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func runOrganizationsList(ctx context.Context, f *cli.Factory) error {
	if err := requireUser(f); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doResources(ctx, client, http.MethodGet, "/v1/organizations", nil, nil)
	if err != nil {
		return err
	}
	rows, err := decodeArray[organization](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, rows, organizationColumns())
}

func runOrganizationsCreate(ctx context.Context, f *cli.Factory, source string) error {
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
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doResources(ctx, client, http.MethodPost, "/v1/organizations", nil, rawBody)
	if err != nil {
		return err
	}
	if resp.Status == http.StatusAccepted {
		row, err := decodeObject[organizationProvision](resp.Body)
		if err != nil {
			return err
		}
		return printTable(f, resp.Body, []organizationProvision{row}, provisionColumns())
	}
	row, err := decodeObject[organization](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []organization{row}, organizationColumns())
}

func runOrganizationsGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireID(id, "organization ID")
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doResources(ctx, client, http.MethodGet, organizationPath(id), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[organization](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []organization{row}, organizationColumns())
}

func provisionColumns() []output.Column[organizationProvision] {
	return []output.Column[organizationProvision]{
		{Header: "OPERATION_ID", Value: func(row organizationProvision) string { return row.OperationID }},
		{Header: "ORGANIZATION_ID", Value: func(row organizationProvision) string { return row.OrganizationID }},
		{Header: "STATE", Value: func(row organizationProvision) string { return row.State }},
	}
}

func organizationColumns() []output.Column[organization] {
	return []output.Column[organization]{
		{Header: "ID", Value: func(o organization) string { return o.ID }},
		{Header: "NAME", Value: func(o organization) string { return o.Name }},
		{Header: "DOMAIN", Value: func(o organization) string { return o.Domain }},
		{Header: "INDUSTRY", Value: func(o organization) string { return o.Industry }},
		{Header: "EMPLOYEE_COUNT", Value: func(o organization) string { return o.EmployeeCount }},
		{Header: "LANGUAGE", Value: func(o organization) string { return o.PreferredLanguage }},
		{Header: "COUNTRY", Value: func(o organization) string { return o.Country }},
		{Header: "UPDATED_AT", Value: func(o organization) string { return o.UpdatedAt }},
	}
}
