package rawapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

const maxPaginatePages = 256

func NewCommand(f *cli.Factory) *cobra.Command {
	var method, input string
	var fields []string
	var paginate bool
	cmd := &cobra.Command{Use: "api <product> <path>", Short: "Call a Saleslumen API path", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(2)}
	cmd.Flags().StringVarP(&method, "method", "X", http.MethodGet, "HTTP method")
	cmd.Flags().StringArrayVarP(&fields, "field", "F", nil, "Query field (key=value)")
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "Follow page tokens and concatenate the list field")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), f, args[0], args[1], method, fields, input, paginate)
	}
	return cmd
}

func run(ctx context.Context, f *cli.Factory, product, path, method string, fields []string, input string, paginate bool) error {
	if !validProduct(product) {
		return &cli.UsageError{Msg: "product must be campaigns, emails, workflows, script, or resources"}
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return &cli.UsageError{Msg: "method is required"}
	}
	if paginate && method != http.MethodGet {
		return &cli.UsageError{Msg: "--paginate requires GET"}
	}
	path, pathQuery, err := splitPath(path)
	if err != nil {
		return err
	}
	query, err := parseFields(fields)
	if err != nil {
		return err
	}
	for key, values := range pathQuery {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	body, err := readInput(f.IO.In, input)
	if err != nil {
		return err
	}
	client, err := requireClient(f)
	if err != nil {
		return err
	}
	if err := requirePrint(f); err != nil {
		return err
	}
	req := apiclient.Request{Product: product, Method: method, Path: path, Query: query, Body: body}
	if paginate {
		combined, err := paginateResponses(ctx, client, req)
		if err != nil {
			return err
		}
		return f.Printer().Object(combined)
	}
	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}
	return f.Printer().Object(resp.Body)
}

func validProduct(product string) bool {
	switch product {
	case "campaigns", "emails", "workflows", "script", "resources":
		return true
	default:
		return false
	}
}

func splitPath(path string) (string, url.Values, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil, &cli.UsageError{Msg: "path is required"}
	}
	query := url.Values{}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		parsed, err := url.ParseQuery(path[i+1:])
		if err != nil {
			return "", nil, &cli.UsageError{Msg: "invalid path query"}
		}
		query = parsed
		path = path[:i]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path, query, nil
}

func parseFields(fields []string) (url.Values, error) {
	query := url.Values{}
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key == "" {
			return nil, &cli.UsageError{Msg: "-F must be key=value"}
		}
		query.Add(key, value)
	}
	return query, nil
}

func readInput(stdin io.Reader, input string) (any, error) {
	if input == "" {
		return nil, nil
	}
	var raw []byte
	var err error
	if input == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("rawapi: stdin is required for --input -")
		}
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(input)
	}
	if err != nil {
		return nil, fmt.Errorf("rawapi: read --input: %w", err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, &cli.UsageError{Msg: "--input must be JSON"}
	}
	return json.RawMessage(raw), nil
}

func requireClient(f *cli.Factory) (*apiclient.Client, error) {
	if f.Client == nil {
		return nil, fmt.Errorf("rawapi: client is required")
	}
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("rawapi: client is required")
	}
	return client, nil
}

func requirePrint(f *cli.Factory) error {
	if f.Printer == nil {
		return fmt.Errorf("rawapi: printer is required")
	}
	return nil
}
