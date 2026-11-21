package script

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
)

const (
	scriptProduct    = "script"
	defaultListLimit = 50
	maxPageSize      = 100
)

func doScript(ctx context.Context, client *apiclient.Client, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	return client.Do(ctx, apiclient.Request{Product: scriptProduct, Method: method, Path: path, Query: query, Body: body})
}

func projectsPath() string {
	return "/v1/projects"
}

func projectPath(id string) string {
	return "/v1/projects/" + url.PathEscape(id)
}

func projectContentPath(id string) string {
	return projectPath(id) + "/content"
}

func projectVersionsPath(id string) string {
	return projectPath(id) + "/versions"
}

func projectVersionPath(project, version string) string {
	return projectVersionsPath(project) + "/" + url.PathEscape(version)
}

func projectVersionContentPath(project, version string) string {
	return projectVersionPath(project, version) + "/content"
}

func projectVersionsComparePath(project string) string {
	return projectVersionsPath(project) + ":compare"
}

func projectVersionRestorePath(project, version string) string {
	return projectVersionPath(project, version) + ":restore"
}

func requireProject(project string) error {
	if strings.TrimSpace(project) == "" {
		return &cli.UsageError{Msg: "--project is required"}
	}
	return nil
}

func requireLimit(limit int) error {
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	return nil
}

func listPageSize(limit, collected int) int {
	remaining := limit - collected
	if remaining < 1 {
		return 1
	}
	return min(remaining, maxPageSize)
}

func listPageQuery(filter url.Values, pageSize int, pageToken string) url.Values {
	query := url.Values{}
	for key, values := range filter {
		query[key] = values
	}
	query.Set("pageSize", strconv.Itoa(pageSize))
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	return query
}

func collectScriptList[T any](ctx context.Context, client *apiclient.Client, path, field string, filter url.Values, limit int) ([]T, []byte, error) {
	if err := requireLimit(limit); err != nil {
		return nil, nil, err
	}
	collected := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]T, string, error) {
		resp, err := doScript(ctx, client, http.MethodGet, path, listPageQuery(filter, listPageSize(limit, collected), pageToken), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[T](resp.Body, field, "nextPageToken")
		if err != nil {
			return nil, "", err
		}
		collected += len(page)
		return page, next, nil
	})
	if err != nil {
		return nil, nil, err
	}
	envelope := input.Object{}
	if err := envelope.Set(field, items); err != nil {
		return nil, nil, err
	}
	raw, err := envelope.Encode()
	return items, raw, err
}

func decodeObject[T any](body []byte) (T, error) {
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("script: decode response: %w", err)
	}
	return out, nil
}

func printTable[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	if err := output.Table(f.Printer(), raw, rows, cols); err != nil {
		return fmt.Errorf("script: print table: %w", err)
	}
	return nil
}

func contentEtag(body []byte) (string, error) {
	var payload input.Object
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("script: decode content etag: %w", err)
	}
	return payload.String("etag"), nil
}

func fetchDraftContentEtag(ctx context.Context, client *apiclient.Client, project string) (string, error) {
	resp, err := doScript(ctx, client, http.MethodGet, projectContentPath(project), nil, nil)
	if err != nil {
		return "", err
	}
	etag, err := contentEtag(resp.Body)
	if err != nil {
		return "", err
	}
	if etag == "" {
		return "", fmt.Errorf("script: draft content etag is empty")
	}
	return etag, nil
}

func applyDraftEtag(body input.Object, flag string, flagSet bool, fetch func() (string, error)) error {
	if err := body.MergeFlag("draftEtag", "draft-etag", flag, flagSet); err != nil {
		return err
	}
	if body.Has("draftEtag") {
		return nil
	}
	etag, err := fetch()
	if err != nil {
		return err
	}
	return body.SetAbsent("draftEtag", etag)
}
