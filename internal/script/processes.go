package script

import (
	"context"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type process struct {
	ProjectName     string `json:"projectName"`
	FunctionName    string `json:"functionName"`
	ProcessType     string `json:"processType"`
	ProcessStatus   string `json:"processStatus"`
	UserAccessLevel string `json:"userAccessLevel"`
	StartTime       string `json:"startTime"`
	Duration        string `json:"duration"`
	RuntimeVersion  string `json:"runtimeVersion"`
}

func newProcessesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "processes", Short: "List script processes", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newProcessesListCommand(f), newProcessesListScriptProcessesCommand(f))
	return cmd
}

func newProcessesListCommand(f *cli.Factory) *cobra.Command {
	var project string
	var limit int
	cmd := &cobra.Command{Use: "list --project ID", Short: "List user processes for a project", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProcessesList(cmd.Context(), f, project, limit, "/v1/processes", "userProcessFilterScriptId")
	}
	return cmd
}

func newProcessesListScriptProcessesCommand(f *cli.Factory) *cobra.Command {
	var project string
	var limit int
	cmd := &cobra.Command{Use: "list-script-processes --project ID", Short: "List script processes for a project", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProcessesList(cmd.Context(), f, project, limit, "/v1/processes:listScriptProcesses", "scriptId")
	}
	return cmd
}

func runProcessesList(ctx context.Context, f *cli.Factory, project string, limit int, path, projectQuery string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	items, raw, err := collectScriptList[process](ctx, client, path, "processes", url.Values{projectQuery: {project}}, limit)
	if err != nil {
		return err
	}
	return printTable(f, raw, items, processColumns())
}

func processColumns() []output.Column[process] {
	return []output.Column[process]{
		{Header: "PROJECT_NAME", Value: func(p process) string { return p.ProjectName }},
		{Header: "FUNCTION_NAME", Value: func(p process) string { return p.FunctionName }},
		{Header: "PROCESS_TYPE", Value: func(p process) string { return p.ProcessType }},
		{Header: "PROCESS_STATUS", Value: func(p process) string { return p.ProcessStatus }},
		{Header: "ACCESS", Value: func(p process) string { return p.UserAccessLevel }},
		{Header: "START_TIME", Value: func(p process) string { return p.StartTime }},
		{Header: "DURATION", Value: func(p process) string { return p.Duration }},
		{Header: "RUNTIME", Value: func(p process) string { return p.RuntimeVersion }},
	}
}
