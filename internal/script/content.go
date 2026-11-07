package script

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type content struct {
	ScriptID string        `json:"scriptId"`
	Files    []contentFile `json:"files"`
	Etag     string        `json:"etag"`
}

type contentFile struct {
	Name             string       `json:"name"`
	Type             string       `json:"type"`
	Source           string       `json:"source"`
	LastModifyUserID string       `json:"lastModifyUserId"`
	CreateTime       string       `json:"createTime"`
	UpdateTime       string       `json:"updateTime"`
	FunctionSet      *functionSet `json:"functionSet"`
}

type functionSet struct {
	Values []scriptFunction `json:"values"`
}

type scriptFunction struct {
	Name       string   `json:"name"`
	Parameters []string `json:"parameters"`
}

func newContentCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "content", Short: "Manage project content", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newContentGetCommand(f), newContentUpdateCommand(f))
	return cmd
}

func newContentGetCommand(f *cli.Factory) *cobra.Command {
	var project string
	var version int
	cmd := &cobra.Command{Use: "get --project ID", Short: "Get project content", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&version, "version", 0, "Version number; omit for draft")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runContentGet(cmd.Context(), f, project, version, cmd.Flags().Changed("version"))
	}
	return cmd
}

func newContentUpdateCommand(f *cli.Factory) *cobra.Command {
	var project, source string
	cmd := &cobra.Command{Use: "update --project ID --input FILE", Short: "Update project content", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runContentUpdate(cmd.Context(), f, project, source)
	}
	return cmd
}

func contentFileColumns() []output.Column[contentFile] {
	return []output.Column[contentFile]{
		{Header: "NAME", Value: func(f contentFile) string { return f.Name }},
		{Header: "TYPE", Value: func(f contentFile) string { return f.Type }},
		{Header: "UPDATE_TIME", Value: func(f contentFile) string { return f.UpdateTime }},
		{Header: "FUNCTIONS", Value: contentFileFunctions},
	}
}

func contentFileFunctions(file contentFile) string {
	if file.FunctionSet == nil {
		return ""
	}
	names := make([]string, 0, len(file.FunctionSet.Values))
	for _, fn := range file.FunctionSet.Values {
		names = append(names, fn.Name)
	}
	return strings.Join(names, ",")
}

func printContent(f *cli.Factory, raw []byte) error {
	doc, err := decodeObject[content](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, doc.Files, contentFileColumns())
}

func runContentGet(ctx context.Context, f *cli.Factory, project string, version int, versionSet bool) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	var query url.Values
	if versionSet {
		query = url.Values{}
		query.Set("versionNumber", strconv.Itoa(version))
	}
	resp, err := doScript(ctx, client, http.MethodGet, projectContentPath(project), query, nil)
	if err != nil {
		return err
	}
	return printContent(f, resp.Body)
}

func runContentUpdate(ctx context.Context, f *cli.Factory, project, source string) error {
	if err := requireProject(project); err != nil {
		return err
	}
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
	resp, err := doScript(ctx, client, http.MethodPut, projectContentPath(project), nil, json.RawMessage(body))
	if err != nil {
		return err
	}
	return printContent(f, resp.Body)
}
