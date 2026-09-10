package script

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type version struct {
	ScriptID       string `json:"scriptId"`
	VersionNumber  int    `json:"versionNumber"`
	Description    string `json:"description"`
	CreateTime     string `json:"createTime"`
	ContentDigest  string `json:"contentDigest"`
	ManifestDigest string `json:"manifestDigest"`
	ContractDigest string `json:"contractDigest"`
	DependencyLock string `json:"dependencyLock"`
	CreateUserID   string `json:"createUserId"`
}

type fileDiff struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	FromSource string `json:"fromSource"`
	ToSource   string `json:"toSource"`
}

type compareResponse struct {
	ScriptID          string     `json:"scriptId"`
	FromVersionNumber int        `json:"fromVersionNumber"`
	ToVersionNumber   int        `json:"toVersionNumber"`
	FileDiffs         []fileDiff `json:"fileDiffs"`
}

func newVersionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "versions", Short: "Manage project versions", Long: "Create version requires a user credential and is not available.", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newVersionsListCommand(f), newVersionsGetCommand(f), newVersionsContentCommand(f), newVersionsCompareCommand(f), newVersionsRestoreCommand(f))
	return cmd
}

func newVersionsListCommand(f *cli.Factory) *cobra.Command {
	var project string
	var limit int
	cmd := &cobra.Command{Use: "list --project ID", Short: "List project versions", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionsList(cmd.Context(), f, project, limit)
	}
	return cmd
}

func newVersionsGetCommand(f *cli.Factory) *cobra.Command {
	var project string
	cmd := &cobra.Command{Use: "get VERSION --project ID", Short: "Get a project version", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionsGet(cmd.Context(), f, project, args[0])
	}
	return cmd
}

func newVersionsContentCommand(f *cli.Factory) *cobra.Command {
	var project string
	cmd := &cobra.Command{Use: "content VERSION --project ID", Short: "Get version content", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionsContent(cmd.Context(), f, project, args[0])
	}
	return cmd
}

func newVersionsCompareCommand(f *cli.Factory) *cobra.Command {
	var project string
	var from, to int
	cmd := &cobra.Command{Use: "compare --project ID --from N --to N", Short: "Compare project versions", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().IntVar(&from, "from", 0, "Source version number")
	cmd.Flags().IntVar(&to, "to", 0, "Target version number")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("from") || !cmd.Flags().Changed("to") {
			return &cli.UsageError{Msg: "--from and --to are required"}
		}
		return runVersionsCompare(cmd.Context(), f, project, from, to)
	}
	return cmd
}

func newVersionsRestoreCommand(f *cli.Factory) *cobra.Command {
	var project, source, draftEtag string
	cmd := &cobra.Command{Use: "restore VERSION --project ID", Short: "Restore a version to draft", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&draftEtag, "draft-etag", "", "Draft etag; fetched from draft content when absent")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runVersionsRestore(cmd.Context(), f, project, args[0], source, draftEtag, cmd.Flags().Changed("draft-etag"))
	}
	return cmd
}

func versionColumns() []output.Column[version] {
	return []output.Column[version]{
		{Header: "VERSION_NUMBER", Value: func(v version) string { return strconv.Itoa(v.VersionNumber) }},
		{Header: "DESCRIPTION", Value: func(v version) string { return v.Description }},
		{Header: "CREATE_TIME", Value: func(v version) string { return v.CreateTime }},
		{Header: "CONTENT_DIGEST", Value: func(v version) string { return v.ContentDigest }},
		{Header: "MANIFEST_DIGEST", Value: func(v version) string { return v.ManifestDigest }},
		{Header: "CONTRACT_DIGEST", Value: func(v version) string { return v.ContractDigest }},
	}
}

func fileDiffColumns() []output.Column[fileDiff] {
	return []output.Column[fileDiff]{
		{Header: "NAME", Value: func(d fileDiff) string { return d.Name }},
		{Header: "STATUS", Value: func(d fileDiff) string { return d.Status }},
	}
}

func printVersion(f *cli.Factory, raw []byte) error {
	row, err := decodeObject[version](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []version{row}, versionColumns())
}

func printVersionDiffs(f *cli.Factory, raw []byte) error {
	doc, err := decodeObject[compareResponse](raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, doc.FileDiffs, fileDiffColumns())
}

func runVersionsList(ctx context.Context, f *cli.Factory, project string, limit int) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	items, raw, err := collectScriptList[version](ctx, client, projectVersionsPath(project), "versions", nil, limit)
	if err != nil {
		return err
	}
	return printTable(f, raw, items, versionColumns())
}

func runVersionsGet(ctx context.Context, f *cli.Factory, project, number string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, projectVersionPath(project, number), nil, nil)
	if err != nil {
		return err
	}
	return printVersion(f, resp.Body)
}

func runVersionsContent(ctx context.Context, f *cli.Factory, project, number string) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, projectVersionContentPath(project, number), nil, nil)
	if err != nil {
		return err
	}
	return printContent(f, resp.Body)
}

func runVersionsCompare(ctx context.Context, f *cli.Factory, project string, from, to int) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("fromVersionNumber", strconv.Itoa(from))
	query.Set("toVersionNumber", strconv.Itoa(to))
	resp, err := doScript(ctx, client, http.MethodGet, projectVersionsComparePath(project), query, nil)
	if err != nil {
		return err
	}
	return printVersionDiffs(f, resp.Body)
}

func runVersionsRestore(ctx context.Context, f *cli.Factory, project, number, source, draftEtag string, draftEtagSet bool) error {
	if err := requireProject(project); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	body, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := applyDraftEtag(body, draftEtag, draftEtagSet, func() (string, error) {
		return fetchDraftContentEtag(ctx, client, project)
	}); err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodPost, projectVersionRestorePath(project, number), nil, raw)
	if err != nil {
		return err
	}
	return printContent(f, resp.Body)
}
