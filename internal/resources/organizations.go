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

func newOrganizationsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "organizations", Short: "Manage organizations", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newOrganizationsGetCommand(f))
	return cmd
}

func newOrganizationsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get an organization", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runOrganizationsGet(cmd.Context(), f, args[0])
	}
	return cmd
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
