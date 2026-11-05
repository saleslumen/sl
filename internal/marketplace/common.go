package marketplace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
)

const (
	marketplaceProduct      = "marketplace"
	scriptProduct           = "script"
	defaultPageSize         = 50
	maxPageSize             = 100
	maxAPIKeyBytes          = 8192
	organizationRequiredMsg = "organization required; pass --organization or run 'sl config set organization_id <id>'"
)

func do(ctx context.Context, client *apiclient.Client, productName, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	return client.Do(ctx, apiclient.Request{Product: productName, Method: method, Path: path, Query: query, Body: body})
}

func requireUser(f *cli.Factory) error {
	if f.RequireUserOAuth == nil {
		return cli.ErrUserOAuthRequired
	}
	return f.RequireUserOAuth()
}

func requireID(id, name string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", &cli.UsageError{Msg: name + " is required"}
	}
	return id, nil
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

func checkLimit(limit int) error {
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be greater than or equal to 0"}
	}
	return nil
}

func pageSizeForLimit(limit int) int {
	if limit <= 0 || limit > maxPageSize {
		return maxPageSize
	}
	return limit
}

func pageQuery(pageSize int, pageToken, filter, orderBy string) url.Values {
	query := url.Values{}
	query.Set("pageSize", strconv.Itoa(pageSize))
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	if filter != "" {
		query.Set("filter", filter)
	}
	if orderBy != "" {
		query.Set("orderBy", orderBy)
	}
	return query
}

func readRequiredObject(f *cli.Factory, source string) (input.Object, error) {
	if strings.TrimSpace(source) == "" {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	return input.ReadObject(f.IO.In, source)
}

func readOptionalObject(f *cli.Factory, source string) (input.Object, error) {
	if strings.TrimSpace(source) == "" {
		return input.Object{}, nil
	}
	return input.ReadObject(f.IO.In, source)
}

func confirm(f *cli.Factory, prompt string) error {
	if f.Confirm == nil {
		return cli.ErrCancelled
	}
	return f.Confirm(prompt)
}

func decodeObject[T any](body []byte) (T, error) {
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("marketplace: decode response: %w", err)
	}
	return out, nil
}

func printRows[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	if err := output.Table(f.Printer(), raw, rows, cols); err != nil {
		return fmt.Errorf("marketplace: print table: %w", err)
	}
	return nil
}

func printObject(f *cli.Factory, raw []byte) error {
	if len(bytesTrim(raw)) == 0 {
		return nil
	}
	if err := f.Printer().Object(raw); err != nil {
		return fmt.Errorf("marketplace: print: %w", err)
	}
	return nil
}

func bytesTrim(raw []byte) string {
	return strings.TrimSpace(string(raw))
}

func listPages[T any](ctx context.Context, f *cli.Factory, path string, limit int, filter, orderBy, field string) ([]T, []byte, error) {
	if err := checkLimit(limit); err != nil {
		return nil, nil, err
	}
	client, err := f.Client()
	if err != nil {
		return nil, nil, err
	}
	pageSize := pageSizeForLimit(limit)
	var lastRaw []byte
	pages := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]T, string, error) {
		resp, err := do(ctx, client, marketplaceProduct, http.MethodGet, path, pageQuery(pageSize, pageToken, filter, orderBy), nil)
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
	raw, err := json.Marshal(map[string]any{field: items})
	if err != nil {
		return nil, nil, fmt.Errorf("marketplace: encode %s: %w", field, err)
	}
	return items, raw, nil
}

func getRows[T any](ctx context.Context, f *cli.Factory, path, field string) ([]T, []byte, error) {
	client, err := f.Client()
	if err != nil {
		return nil, nil, err
	}
	resp, err := do(ctx, client, marketplaceProduct, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	rows, _, err := apiclient.DecodePage[T](resp.Body, field, "nextPageToken")
	if err != nil {
		return nil, nil, err
	}
	return rows, resp.Body, nil
}

func callRaw(ctx context.Context, f *cli.Factory, method, path string, query url.Values, body any) ([]byte, error) {
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	resp, err := do(ctx, client, marketplaceProduct, method, path, query, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func appPath(id string) string {
	return "/v1/apps/" + url.PathEscape(id)
}

func publisherAppPath(id string) string {
	return "/v1/publisherApps/" + url.PathEscape(id)
}

func listingPath(appID, listingID string) string {
	return appPath(appID) + "/listings/" + url.PathEscape(listingID)
}

func revisionPath(appID, listingID, revisionID string) string {
	return listingPath(appID, listingID) + "/revisions/" + url.PathEscape(revisionID)
}

func oauthClientPath(appID, connectionKey string) string {
	return appPath(appID) + "/connectionOAuthClients/" + url.PathEscape(connectionKey)
}

func releasePath(appID, releaseID string) string {
	return appPath(appID) + "/releases/" + url.PathEscape(releaseID)
}

func submissionPath(appID, submissionID string) string {
	return appPath(appID) + "/submissions/" + url.PathEscape(submissionID)
}

func installationPath(id string) string {
	return "/v1/installations/" + url.PathEscape(id)
}

func connectionPath(installationID, connectionKey string) string {
	return installationPath(installationID) + "/connections/" + url.PathEscape(connectionKey)
}

func organizationPolicyPath(org string) string {
	return "/v1/organizations/" + url.PathEscape(org) + "/marketplacePolicy"
}

func installCapabilitiesPath(org string) string {
	return "/v1/organizations/" + url.PathEscape(org) + "/installCapabilities"
}

func newOperationID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("marketplace: operation id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func readAPIKey(stdin io.Reader, source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", &cli.UsageError{Msg: "--api-key-file is required"}
	}
	var raw []byte
	var err error
	if source == "-" {
		if stdin == nil {
			return "", fmt.Errorf("marketplace: stdin is required for --api-key-file -")
		}
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(source)
	}
	if err != nil {
		return "", fmt.Errorf("marketplace: read api key file: %w", err)
	}
	secret := string(raw)
	if !utf8.ValidString(secret) || strings.TrimSpace(secret) == "" || len(secret) > maxAPIKeyBytes {
		return "", &cli.UsageError{Msg: "--api-key-file must be a non-empty UTF-8 value of at most 8192 bytes"}
	}
	return secret, nil
}

func requireMerged(obj input.Object, field, flag string) error {
	if obj.Has(field) {
		return nil
	}
	return &cli.UsageError{Msg: "--" + flag + " is required"}
}
