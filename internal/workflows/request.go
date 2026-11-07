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
	return client.Do(ctx, apiclient.Request{Product: productWorkflows, Method: method, Path: path, Query: query, Body: body})
}

func workflowPath(id string) string {
	return workflowsPath + "/" + url.PathEscape(id)
}

func workflowSubpath(workflow string, segments ...string) string {
	path := workflowPath(workflow)
	for _, segment := range segments {
		path += "/" + url.PathEscape(segment)
	}
	return path
}

func workflowCustomPath(workflow, verb string) string {
	return workflowPath(workflow) + ":" + verb
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

func readWorkflowObject(stdin io.Reader, source, flagField, flagName, flagValue string, flagChanged bool) (json.RawMessage, error) {
	if source == "" && !flagChanged {
		return nil, &cli.UsageError{Msg: "--input or --" + flagName + " is required"}
	}
	object, err := input.ReadObject(stdin, source)
	if err != nil {
		return nil, err
	}
	if flagField != "" {
		if err := object.MergeFlag(flagField, flagName, flagValue, flagChanged); err != nil {
			return nil, err
		}
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

func encodeCollection(field string, items []json.RawMessage) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{field: items})
	if err != nil {
		return nil, fmt.Errorf("workflows: encode %s: %w", field, err)
	}
	return raw, nil
}

func listPages[T any](ctx context.Context, f *cli.Factory, path string, limit, maxPage int, field string, fixedQuery url.Values) ([]T, []byte, error) {
	client, err := f.Client()
	if err != nil {
		return nil, nil, err
	}
	pageSize := pageSizeForLimit(limit, maxPage)
	var lastRaw []byte
	pages := 0
	rawItems, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		query := pageQuery(pageSize, pageToken)
		for key, values := range fixedQuery {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		resp, err := client.Do(ctx, apiclient.Request{Product: productWorkflows, Method: http.MethodGet, Path: path, Query: query})
		if err != nil {
			return nil, "", err
		}
		lastRaw = resp.Body
		pages++
		return apiclient.DecodePage[json.RawMessage](resp.Body, field, "nextPageToken")
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]T, 0, len(rawItems))
	for _, item := range rawItems {
		var row T
		if err := json.Unmarshal(item, &row); err != nil {
			return nil, nil, fmt.Errorf("workflows: decode %s item: %w", field, err)
		}
		items = append(items, row)
	}
	if pages <= 1 {
		if lastRaw == nil {
			lastRaw = []byte("{}")
		}
		return items, lastRaw, nil
	}
	encoded, err := encodeCollection(field, rawItems)
	if err != nil {
		return nil, nil, err
	}
	return items, encoded, nil
}

func printRows[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	if err := output.Table(f.Printer(), raw, rows, cols); err != nil {
		return fmt.Errorf("workflows: print: %w", err)
	}
	return nil
}

func decodeRow[T any](raw []byte) (T, error) {
	var row T
	if err := json.Unmarshal(raw, &row); err != nil {
		return row, fmt.Errorf("workflows: decode resource: %w", err)
	}
	return row, nil
}

func printJSON(f *cli.Factory, raw []byte) error {
	if err := f.Printer().Object(raw); err != nil {
		return fmt.Errorf("workflows: print: %w", err)
	}
	return nil
}

func printWorkflow(f *cli.Factory, raw []byte) error {
	row, err := decodeRow[Workflow](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []Workflow{row}, workflowColumns())
}

func printRevision(f *cli.Factory, raw []byte) error {
	row, err := decodeRow[Revision](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []Revision{row}, revisionColumns())
}

func printExecution(f *cli.Factory, raw []byte) error {
	row, err := decodeRow[Execution](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []Execution{row}, executionColumns())
}

func printStepRun(f *cli.Factory, raw []byte) error {
	row, err := decodeRow[StepRun](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []StepRun{row}, stepRunColumns())
}

func printTrigger(f *cli.Factory, raw []byte) error {
	row, err := decodeRow[Trigger](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []Trigger{row}, triggerColumns())
}

func formatInt(value int) string { return strconv.Itoa(value) }

func formatBool(value bool) string { return strconv.FormatBool(value) }
