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

type libraryVersion struct {
	VersionNumber int    `json:"versionNumber"`
	Description   string `json:"description"`
	CreateTime    string `json:"createTime"`
	Recommended   bool   `json:"recommended"`
}

type libraryLookup struct {
	LibraryID                string           `json:"libraryId"`
	Title                    string           `json:"title"`
	Description              string           `json:"description"`
	RecommendedVersionNumber int              `json:"recommendedVersionNumber"`
	Versions                 []libraryVersion `json:"versions"`
	CanUseDevelopmentMode    bool             `json:"canUseDevelopmentMode"`
}

type libraryReference struct {
	LibraryID     string                `json:"libraryId"`
	VersionNumber int                   `json:"versionNumber"`
	Symbols       []libraryPublicSymbol `json:"symbols"`
}

type libraryPublicSymbol struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parameters  []string `json:"parameters"`
}

type dependencyGraphValidation struct {
	Valid          bool     `json:"valid"`
	DependencyLock string   `json:"dependencyLock"`
	Errors         []string `json:"errors"`
}

func newLibrariesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "libraries", Short: "Inspect script libraries", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newLibrariesLookupCommand(f), newLibrariesListVersionsCommand(f), newLibrariesValidateDependencyGraphCommand(f), newLibrariesGetReferenceCommand(f))
	return cmd
}

func newLibrariesLookupCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "lookup ID", Short: "Lookup a published library", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLibrariesLookup(cmd.Context(), f, args[0])
	}
	return cmd
}

func newLibrariesListVersionsCommand(f *cli.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list-versions ID", Short: "List released library versions", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum items to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLibrariesListVersions(cmd.Context(), f, args[0], limit)
	}
	return cmd
}

func newLibrariesValidateDependencyGraphCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "validate-dependency-graph --input FILE", Short: "Validate a library dependency graph", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLibrariesValidateDependencyGraph(cmd.Context(), f, source)
	}
	return cmd
}

func newLibrariesGetReferenceCommand(f *cli.Factory) *cobra.Command {
	var version int
	cmd := &cobra.Command{Use: "get-reference ID --version N", Short: "Get a library version reference", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&version, "version", 0, "Library version number")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("version") {
			return &cli.UsageError{Msg: "--version is required"}
		}
		return runLibrariesGetReference(cmd.Context(), f, args[0], version)
	}
	return cmd
}

func runLibrariesLookup(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireLibraryID(id)
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, libraryLookupPath(id), nil, nil)
	if err != nil {
		return err
	}
	row, err := decodeObject[libraryLookup](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []libraryLookup{row}, libraryLookupColumns())
}

func runLibrariesListVersions(ctx context.Context, f *cli.Factory, id string, limit int) error {
	id, err := requireLibraryID(id)
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	items, raw, err := collectScriptList[libraryVersion](ctx, client, libraryVersionsPath(id), "versions", nil, limit)
	if err != nil {
		return err
	}
	return printTable(f, raw, items, libraryVersionColumns())
}

func runLibrariesValidateDependencyGraph(ctx context.Context, f *cli.Factory, source string) error {
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
	resp, err := doScript(ctx, client, http.MethodPost, "/v1/libraries:validateDependencyGraph", nil, json.RawMessage(body))
	if err != nil {
		return err
	}
	row, err := decodeObject[dependencyGraphValidation](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []dependencyGraphValidation{row}, dependencyGraphValidationColumns())
}

func runLibrariesGetReference(ctx context.Context, f *cli.Factory, id string, version int) error {
	id, err := requireLibraryID(id)
	if err != nil {
		return err
	}
	if err := requireLibraryVersion(version); err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	resp, err := doScript(ctx, client, http.MethodGet, libraryReferencePath(id, version), nil, nil)
	if err != nil {
		return err
	}
	reference, err := decodeObject[libraryReference](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, reference.Symbols, librarySymbolColumns())
}

func libraryLookupColumns() []output.Column[libraryLookup] {
	return []output.Column[libraryLookup]{
		{Header: "LIBRARY_ID", Value: func(l libraryLookup) string { return l.LibraryID }},
		{Header: "TITLE", Value: func(l libraryLookup) string { return l.Title }},
		{Header: "RECOMMENDED_VERSION", Value: func(l libraryLookup) string { return strconv.Itoa(l.RecommendedVersionNumber) }},
		{Header: "DEVELOPMENT_MODE", Value: func(l libraryLookup) string { return strconv.FormatBool(l.CanUseDevelopmentMode) }},
		{Header: "DESCRIPTION", Value: func(l libraryLookup) string { return l.Description }},
	}
}

func libraryVersionColumns() []output.Column[libraryVersion] {
	return []output.Column[libraryVersion]{
		{Header: "VERSION_NUMBER", Value: func(v libraryVersion) string { return strconv.Itoa(v.VersionNumber) }},
		{Header: "RECOMMENDED", Value: func(v libraryVersion) string { return strconv.FormatBool(v.Recommended) }},
		{Header: "CREATE_TIME", Value: func(v libraryVersion) string { return v.CreateTime }},
		{Header: "DESCRIPTION", Value: func(v libraryVersion) string { return v.Description }},
	}
}

func dependencyGraphValidationColumns() []output.Column[dependencyGraphValidation] {
	return []output.Column[dependencyGraphValidation]{
		{Header: "VALID", Value: func(v dependencyGraphValidation) string { return strconv.FormatBool(v.Valid) }},
		{Header: "DEPENDENCY_LOCK", Value: func(v dependencyGraphValidation) string { return v.DependencyLock }},
		{Header: "ERRORS", Value: func(v dependencyGraphValidation) string { return strings.Join(v.Errors, ",") }},
	}
}

func librarySymbolColumns() []output.Column[libraryPublicSymbol] {
	return []output.Column[libraryPublicSymbol]{
		{Header: "NAME", Value: func(s libraryPublicSymbol) string { return s.Name }},
		{Header: "PARAMETERS", Value: func(s libraryPublicSymbol) string { return strings.Join(s.Parameters, ",") }},
		{Header: "DESCRIPTION", Value: func(s libraryPublicSymbol) string { return s.Description }},
	}
}

func requireLibraryID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", &cli.UsageError{Msg: "library ID is required"}
	}
	return id, nil
}

func requireLibraryVersion(version int) error {
	if version < 1 {
		return &cli.UsageError{Msg: "--version must be at least 1"}
	}
	return nil
}

func libraryLookupPath(id string) string {
	return "/v1/libraries/" + url.PathEscape(id) + ":lookup"
}

func libraryVersionsPath(id string) string {
	return "/v1/libraries/" + url.PathEscape(id) + "/versions"
}

func libraryReferencePath(id string, version int) string {
	return libraryVersionsPath(id) + "/" + strconv.Itoa(version) + "/reference"
}
