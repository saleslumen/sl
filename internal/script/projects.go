package script

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type project struct {
	ScriptID         string `json:"scriptId"`
	Title            string `json:"title"`
	ParentID         string `json:"parentId"`
	CreateTime       string `json:"createTime"`
	UpdateTime       string `json:"updateTime"`
	CreatorUserID    string `json:"creatorUserId"`
	LastModifyUserID string `json:"lastModifyUserId"`
	LifecycleState   string `json:"lifecycleState"`
	ArchiveTime      string `json:"archiveTime"`
}

func newProjectsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "projects", Short: "Manage script projects", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newProjectsListCommand(f), newProjectsGetCommand(f), newProjectsCreateCommand(f), newProjectsUpdateCommand(f), newProjectsDeleteCommand(f))
	return cmd
}

func newProjectsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List script projects", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProjectsList(cmd.Context(), f, limit)
	}
	return cmd
}

func newProjectsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get PROJECT_ID", Short: "Get a script project", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProjectsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newProjectsCreateCommand(f *cli.Factory) *cobra.Command {
	var title, input string
	cmd := &cobra.Command{Use: "create", Short: "Create a script project", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&title, "title", "", "Project title")
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProjectsCreate(cmd.Context(), f, title, input, cmd.Flags().Changed("title"))
	}
	return cmd
}

func newProjectsUpdateCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "update PROJECT_ID --input FILE", Short: "Update a script project", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProjectsUpdate(cmd.Context(), f, args[0], source)
	}
	return cmd
}

func newProjectsDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete PROJECT_ID", Short: "Delete a script project", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProjectsDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func projectColumns() []output.Column[project] {
	return []output.Column[project]{
		{Header: "SCRIPT_ID", Value: func(p project) string { return p.ScriptID }},
		{Header: "TITLE", Value: func(p project) string { return p.Title }},
		{Header: "LIFECYCLE_STATE", Value: func(p project) string { return p.LifecycleState }},
		{Header: "UPDATE_TIME", Value: func(p project) string { return p.UpdateTime }},
		{Header: "ARCHIVE_TIME", Value: func(p project) string { return p.ArchiveTime }},
	}
}

func printProject(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[project](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []project{row}, projectColumns())
}

func runProjectsList(ctx context.Context, f *cli.Factory, limit int) error {
	client, err := f.Client()
	if err != nil {
		return err
	}
	items, raw, err := collectScriptList[project](ctx, client, projectsPath(), "projects", nil, limit)
	if err != nil {
		return err
	}
	return printTable(f, raw, items, projectColumns())
}

func runProjectsGet(ctx context.Context, f *cli.Factory, id string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, projectPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printProject(f, resp.Body)
}

func runProjectsCreate(ctx context.Context, f *cli.Factory, title, source string, titleSet bool) error {
	client, err := f.Client()
	if err != nil {
		return err
	}
	body, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("title", "title", title, titleSet); err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPost, projectsPath(), nil, raw)
	if err != nil {
		return err
	}
	return printProject(f, resp.Body)
}

func runProjectsUpdate(ctx context.Context, f *cli.Factory, id, source string) error {
	if source == "" {
		return &cli.UsageError{Msg: "--input is required"}
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	body, err := input.Read(f.IO.In, source)
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPut, projectPath(id), nil, json.RawMessage(body))
	if err != nil {
		return err
	}
	return printProject(f, resp.Body)
}

func runProjectsDelete(ctx context.Context, f *cli.Factory, id string) error {
	if err := f.Confirm("Delete project " + id + "?"); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodDelete, projectPath(id), nil, nil)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return nil
	}
	return f.Printer().Object(resp.Body)
}
