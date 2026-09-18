package campaigns

import (
	"bytes"
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
	"github.com/spf13/cobra"
)

func ensureRequestID(f *cli.Factory, object input.Object, value string, changed bool) error {
	if err := object.MergeFlag("request_id", "request-id", value, changed); err != nil {
		return err
	}
	if object.Has("request_id") {
		return nil
	}
	id := f.NewRequestID()
	if id == "" {
		return fmt.Errorf("campaigns: request id generator returned empty")
	}
	return object.Set("request_id", id)
}

func ensureOrganization(f *cli.Factory, object input.Object) error {
	if object.Has("organization") {
		return nil
	}
	id, err := f.Organization()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return &cli.UsageError{Msg: "organization required; pass --organization or run 'sl config set organization_id <id>'"}
	}
	return object.Set("organization", "organizations/"+id)
}

func ensureEtag(ctx context.Context, f *cli.Factory, object input.Object, value string, changed bool, getPath string) error {
	if err := object.MergeFlag("etag", "etag", value, changed); err != nil {
		return err
	}
	if object.Has("etag") {
		return nil
	}
	if getPath == "" {
		return &cli.UsageError{Msg: "--etag is required"}
	}
	resp, err := do(ctx, f, http.MethodGet, getPath, nil, nil)
	if err != nil {
		return fmt.Errorf("campaigns: fetch etag %s: %w", getPath, err)
	}
	var resource input.Object
	if err := json.Unmarshal(resp.Body, &resource); err != nil {
		return fmt.Errorf("campaigns: decode etag %s: %w", getPath, err)
	}
	etag := resource.String("etag")
	if etag == "" {
		return fmt.Errorf("campaigns: %s has no etag", getPath)
	}
	return object.Set("etag", etag)
}

func do(ctx context.Context, f *cli.Factory, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	return client.Do(ctx, apiclient.Request{Product: "campaigns", Method: method, Path: path, Query: query, Body: body})
}

func requireArg(args []string, name string) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return "", &cli.UsageError{Msg: fmt.Sprintf("%s is required", name)}
	}
	return args[0], nil
}

func exactArg(name string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		_, err := requireArg(args, name)
		return err
	}
}

func requireCampaign(id string) (string, error) {
	id = resourceID(id)
	if id == "" {
		return "", &cli.UsageError{Msg: "--campaign is required"}
	}
	return id, nil
}

func readRequiredObject(f *cli.Factory, source string) (input.Object, error) {
	if strings.TrimSpace(source) == "" {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	return input.ReadObject(f.IO.In, source)
}

func listQuery(limit int, pageToken string) url.Values {
	const maxSize = 200
	query := url.Values{}
	if limit > 0 {
		size := limit
		if size > maxSize {
			size = maxSize
		}
		query.Set("page_size", strconv.Itoa(size))
	}
	if pageToken != "" {
		query.Set("page_token", pageToken)
	}
	return query
}

func resourceID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func formatScalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	case json.RawMessage:
		raw := bytes.TrimSpace(typed)
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
		return string(raw)
	default:
		return fmt.Sprint(typed)
	}
}

func formatCount(count int) string {
	return strconv.Itoa(count)
}
