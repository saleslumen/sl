package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
)

func doRequest(ctx context.Context, f *cli.Factory, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(ctx, apiclient.Request{Product: productWorkflows, Method: method, Path: path, Query: query, Body: body})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func emptyObject() map[string]any {
	return map[string]any{}
}

func workflowPath(id string) string {
	return workflowsPath + "/" + url.PathEscape(id)
}

func workflowCustomPath(id, verb string) string {
	return workflowPath(id) + ":" + verb
}

func workflowSubpath(id string, segments ...string) string {
	path := workflowPath(id)
	for _, segment := range segments {
		path += "/" + url.PathEscape(segment)
	}
	return path
}

func requirePositional(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &cli.UsageError{Msg: name + " is required"}
	}
	return value, nil
}

func requireFlag(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &cli.UsageError{Msg: "--" + name + " is required"}
	}
	return value, nil
}

func createWorkflowBody(stdin io.Reader, source, name string, nameChanged bool) (json.RawMessage, error) {
	if source == "" && !nameChanged {
		return nil, &cli.UsageError{Msg: "--input or --name is required"}
	}
	object, err := input.ReadObject(stdin, source)
	if err != nil {
		return nil, err
	}
	if err := object.MergeFlag("name", "name", name, nameChanged); err != nil {
		return nil, err
	}
	return object.Encode()
}

func requiredJSONObject(stdin io.Reader, source string) (json.RawMessage, error) {
	if source == "" {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	object, err := input.ReadObject(stdin, source)
	if err != nil {
		return nil, err
	}
	return object.Encode()
}

func pageQuery(pageSize int, pageToken string) url.Values {
	query := url.Values{}
	if pageSize > 0 {
		query.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	return query
}

func pageSizeForLimit(limit, maxPage int) int {
	pageSize := limit
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if maxPage > 0 && pageSize > maxPage {
		return maxPage
	}
	return pageSize
}

func encodeCollection[T any](field string, items []T) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{field: items})
	if err != nil {
		return nil, fmt.Errorf("workflows: encode %s: %w", field, err)
	}
	return raw, nil
}

func listPages[T any](ctx context.Context, f *cli.Factory, path string, limit, maxPage int, field string) ([]T, []byte, error) {
	client, err := f.Client()
	if err != nil {
		return nil, nil, err
	}
	pageSize := pageSizeForLimit(limit, maxPage)
	var lastRaw []byte
	pages := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]T, string, error) {
		resp, err := client.Do(ctx, apiclient.Request{Product: productWorkflows, Method: http.MethodGet, Path: path, Query: pageQuery(pageSize, pageToken)})
		if err != nil {
			return nil, "", err
		}
		lastRaw = resp.Body
		pages++
		return apiclient.DecodePage[T](resp.Body, field, "nextPageToken")
	})
	if err != nil {
		return nil, nil, err
	}
	if pages <= 1 {
		if lastRaw == nil {
			lastRaw = []byte("{}")
		}
		return items, lastRaw, nil
	}
	raw, err := encodeCollection(field, items)
	if err != nil {
		return nil, nil, err
	}
	return items, raw, nil
}

func printRows[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	printer := f.Printer()
	if err := output.Table(printer, raw, rows, cols); err != nil {
		return fmt.Errorf("workflows: print: %w", err)
	}
	return nil
}

func printJSON(f *cli.Factory, raw []byte) error {
	printer := f.Printer()
	if err := printer.Object(raw); err != nil {
		return fmt.Errorf("workflows: print: %w", err)
	}
	return nil
}

func printWorkflow(f *cli.Factory, raw []byte) error {
	var resp workflowResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("workflows: decode workflow: %w", err)
	}
	return printRows(f, raw, []Workflow{resp.Workflow}, workflowColumns())
}

func printVersions(f *cli.Factory, raw []byte, rows []WorkflowVersion) error {
	return printRows(f, raw, rows, versionColumns())
}
