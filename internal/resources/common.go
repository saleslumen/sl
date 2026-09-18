package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
)

const (
	resourcesProduct        = "resources"
	organizationRequiredMsg = "organization required; pass --organization or run 'sl config set organization_id <id>'"
)

func doResources(ctx context.Context, client *apiclient.Client, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	return client.Do(ctx, apiclient.Request{Product: resourcesProduct, Method: method, Path: path, Query: query, Body: body})
}

func resolveOrganization(f *cli.Factory) (string, error) {
	id, err := f.Organization()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(id) == "" {
		return "", &cli.UsageError{Msg: organizationRequiredMsg}
	}
	return id, nil
}

func requireID(id, name string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", &cli.UsageError{Msg: name + " is required"}
	}
	return id, nil
}

func organizationPath(id string) string {
	return "/v1/organizations/" + url.PathEscape(id)
}

func namespacesPath(org string) string {
	return organizationPath(org) + "/namespaces"
}

func namespacePath(org, id string) string {
	return namespacesPath(org) + "/" + url.PathEscape(id)
}

func readRequiredObject(f *cli.Factory, source string) (input.Object, error) {
	if strings.TrimSpace(source) == "" {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	return input.ReadObject(f.IO.In, source)
}

func decodeObject[T any](body []byte) (T, error) {
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("resources: decode response: %w", err)
	}
	return out, nil
}

func decodeArray[T any](body []byte) ([]T, error) {
	var out []T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("resources: decode response: %w", err)
	}
	if out == nil {
		out = []T{}
	}
	return out, nil
}

func printTable[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	if err := output.Table(f.Printer(), raw, rows, cols); err != nil {
		return fmt.Errorf("resources: print table: %w", err)
	}
	return nil
}
