package script

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type deploymentConfig struct {
	ScriptID         string `json:"scriptId,omitempty"`
	VersionNumber    int    `json:"versionNumber,omitempty"`
	ManifestFileName string `json:"manifestFileName,omitempty"`
	Description      string `json:"description,omitempty"`
	DeploymentType   string `json:"deploymentType,omitempty"`
}

type deployment struct {
	DeploymentID     string           `json:"deploymentId"`
	DeploymentConfig deploymentConfig `json:"deploymentConfig"`
	UpdateTime       string           `json:"updateTime"`
	DeploymentType   string           `json:"deploymentType"`
	LifecycleState   string           `json:"lifecycleState"`
	ArchiveTime      string           `json:"archiveTime"`
}

func newDeploymentsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "deployments", Short: "Manage script deployments", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newDeploymentsListCommand(f), newDeploymentsGetCommand(f), newDeploymentsCreateCommand(f), newDeploymentsUpdateCommand(f), newDeploymentsDeleteCommand(f))
	return cmd
}

func newDeploymentsListCommand(f *cli.Factory) *cobra.Command {
	var project string
	var limit int
	cmd := &cobra.Command{Use: "list --project ID", Short: "List deployments", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploymentsList(cmd.Context(), f, project, limit)
	}
	return cmd
}

func newDeploymentsGetCommand(f *cli.Factory) *cobra.Command {
	var project string
	cmd := &cobra.Command{Use: "get DEPLOYMENT_ID --project ID", Short: "Get a deployment", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploymentsGet(cmd.Context(), f, project, args[0])
	}
	return cmd
}

func newDeploymentsCreateCommand(f *cli.Factory) *cobra.Command {
	var project, description, deploymentType, source string
	var versionNumber int
	cmd := &cobra.Command{Use: "create --project ID", Short: "Create a deployment; LIBRARY type requires a user credential", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&versionNumber, "version-number", 0, "Pinned version number")
	cmd.Flags().StringVar(&description, "description", "", "Deployment description")
	cmd.Flags().StringVar(&deploymentType, "deployment-type", "", "Deployment type")
	cmd.Flags().StringVar(&source, "input", "", "JSON deployment_config file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploymentsCreate(cmd.Context(), f, project, source, versionNumber, cmd.Flags().Changed("version-number"), description, cmd.Flags().Changed("description"), deploymentType, cmd.Flags().Changed("deployment-type"))
	}
	return cmd
}

func newDeploymentsUpdateCommand(f *cli.Factory) *cobra.Command {
	var project, source string
	cmd := &cobra.Command{Use: "update DEPLOYMENT_ID --project ID --input FILE", Short: "Update a deployment; LIBRARY type requires a user credential", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON deployment_config file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploymentsUpdate(cmd.Context(), f, project, args[0], source)
	}
	return cmd
}

func newDeploymentsDeleteCommand(f *cli.Factory) *cobra.Command {
	var project string
	cmd := &cobra.Command{Use: "delete DEPLOYMENT_ID --project ID", Short: "Archive a deployment; LIBRARY type requires a user credential", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploymentsDelete(cmd.Context(), f, project, args[0])
	}
	return cmd
}

func runDeploymentsList(ctx context.Context, f *cli.Factory, project string, limit int) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	items, raw, err := collectScriptList[deployment](ctx, client, deploymentsPath(project), "deployments", nil, limit)
	if err != nil {
		return err
	}
	return printTable(f, raw, items, deploymentColumns())
}

func runDeploymentsGet(ctx context.Context, f *cli.Factory, project, id string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, deploymentPath(project, id), nil, nil)
	if err != nil {
		return err
	}
	return printDeployment(f, resp.Body)
}

func runDeploymentsCreate(ctx context.Context, f *cli.Factory, project, source string, versionNumber int, versionSet bool, description string, descriptionSet bool, deploymentType string, typeSet bool) error {
	if err := requireProject(project); err != nil {
		return err
	}
	body, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("versionNumber", "version-number", versionNumber, versionSet); err != nil {
		return err
	}
	if err := body.MergeFlag("description", "description", description, descriptionSet); err != nil {
		return err
	}
	if err := body.MergeFlag("deploymentType", "deployment-type", deploymentType, typeSet); err != nil {
		return err
	}
	if !body.Has("versionNumber") {
		return &cli.UsageError{Msg: "versionNumber is required"}
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPost, deploymentsPath(project), nil, raw)
	if err != nil {
		return err
	}
	return printDeployment(f, resp.Body)
}

func runDeploymentsUpdate(ctx context.Context, f *cli.Factory, project, id, source string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	if source == "" {
		return &cli.UsageError{Msg: "--input is required"}
	}
	body, err := input.Read(f.IO.In, source)
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPut, deploymentPath(project, id), nil, json.RawMessage(body))
	if err != nil {
		return err
	}
	return printDeployment(f, resp.Body)
}

func runDeploymentsDelete(ctx context.Context, f *cli.Factory, project, id string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	if err := f.Confirm("Delete deployment " + id + "?"); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodDelete, deploymentPath(project, id), nil, nil)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return nil
	}
	return f.Printer().Object(resp.Body)
}

func printDeployment(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[deployment](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []deployment{row}, deploymentColumns())
}

func deploymentColumns() []output.Column[deployment] {
	return []output.Column[deployment]{
		{Header: "DEPLOYMENT_ID", Value: func(d deployment) string { return d.DeploymentID }},
		{Header: "VERSION_NUMBER", Value: func(d deployment) string { return fmt.Sprint(d.DeploymentConfig.VersionNumber) }},
		{Header: "DEPLOYMENT_TYPE", Value: func(d deployment) string { return d.DeploymentType }},
		{Header: "LIFECYCLE_STATE", Value: func(d deployment) string { return d.LifecycleState }},
		{Header: "UPDATE_TIME", Value: func(d deployment) string { return d.UpdateTime }},
		{Header: "ARCHIVE_TIME", Value: func(d deployment) string { return d.ArchiveTime }},
		{Header: "DESCRIPTION", Value: func(d deployment) string { return d.DeploymentConfig.Description }},
	}
}

func deploymentsPath(project string) string {
	return projectPath(project) + "/deployments"
}

func deploymentPath(project, id string) string {
	return deploymentsPath(project) + "/" + url.PathEscape(id)
}
