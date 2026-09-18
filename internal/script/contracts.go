package script

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type contractError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type contract struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Inputs      map[string]json.RawMessage `json:"inputs"`
	Outputs     map[string]json.RawMessage `json:"outputs"`
	Errors      []contractError            `json:"errors"`
}

type getContractsResponse struct {
	Contracts      []contract `json:"contracts"`
	ContractDigest string     `json:"contractDigest"`
}

func newContractsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "contracts", Short: "Read script contracts", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newContractsGetCommand(f))
	return cmd
}

func newContractsGetCommand(f *cli.Factory) *cobra.Command {
	var version int
	cmd := &cobra.Command{Use: "get PROJECT_ID", Short: "Get script contracts", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&version, "version", 0, "Version number")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runContractsGet(cmd.Context(), f, args[0], version, cmd.Flags().Changed("version"))
	}
	return cmd
}

func runContractsGet(ctx context.Context, f *cli.Factory, project string, version int, versionSet bool) error {
	if strings.TrimSpace(project) == "" {
		return &cli.UsageError{Msg: "PROJECT_ID is required"}
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	query := url.Values{}
	if versionSet {
		query.Set("versionNumber", strconv.Itoa(version))
	}
	resp, err := doScript(ctx, client, http.MethodGet, contractsPath(project), query, nil)
	if err != nil {
		return err
	}
	payload, err := decodeObject[getContractsResponse](resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, payload.Contracts, contractColumns())
}

func contractColumns() []output.Column[contract] {
	return []output.Column[contract]{
		{Header: "NAME", Value: func(c contract) string { return c.Name }},
		{Header: "DESCRIPTION", Value: func(c contract) string { return c.Description }},
		{Header: "INPUTS", Value: func(c contract) string { return contractMapNames(c.Inputs) }},
		{Header: "OUTPUTS", Value: func(c contract) string { return contractMapNames(c.Outputs) }},
		{Header: "ERRORS", Value: func(c contract) string { return contractErrorCodes(c.Errors) }},
	}
}

func contractMapNames(fields map[string]json.RawMessage) string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func contractErrorCodes(items []contractError) string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.Code)
	}
	return strings.Join(codes, ",")
}

func contractsPath(project string) string {
	return "/v1/scripts/" + url.PathEscape(project) + "/contracts"
}
