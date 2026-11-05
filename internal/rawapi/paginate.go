package rawapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/saleslumen/sl/internal/apiclient"
)

func paginateResponses(ctx context.Context, client *apiclient.Client, base apiclient.Request) ([]byte, error) {
	var field string
	var first map[string]json.RawMessage
	var items []json.RawMessage
	tokenKey := ""
	seenToken := ""
	query := cloneValues(base.Query)
	for page := 0; page < maxPaginatePages; page++ {
		resp, err := client.Do(ctx, apiclient.Request{Product: base.Product, Method: base.Method, Path: base.Path, Query: query, Body: base.Body})
		if err != nil {
			return nil, err
		}
		obj, err := decodeObject(resp.Body)
		if err != nil {
			return nil, err
		}
		arrays := arrayFields(obj)
		if len(arrays) != 1 {
			return nil, fmt.Errorf("rawapi: paginate requires exactly one array field, found %d", len(arrays))
		}
		name := arrays[0]
		if field == "" {
			field = name
			first = obj
		} else if name != field {
			return nil, fmt.Errorf("rawapi: paginate array field changed from %s to %s", field, name)
		}
		pageItems, err := decodeArray(obj[name])
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		next, requestKey := nextPageToken(obj)
		if next == "" {
			return encodeCombined(first, field, items)
		}
		if next == seenToken {
			return nil, fmt.Errorf("rawapi: pagination repeated page token")
		}
		if tokenKey == "" {
			tokenKey = requestKey
		}
		seenToken = next
		query = cloneValues(base.Query)
		query.Set(tokenKey, next)
	}
	return nil, fmt.Errorf("rawapi: pagination exceeded %d pages", maxPaginatePages)
}

func decodeObject(body []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("rawapi: paginate response must be a JSON object")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, fmt.Errorf("rawapi: decode page: %w", err)
	}
	return obj, nil
}

func arrayFields(obj map[string]json.RawMessage) []string {
	var names []string
	for key, raw := range obj {
		value := bytes.TrimSpace(raw)
		if len(value) > 0 && value[0] == '[' {
			names = append(names, key)
		}
	}
	sort.Strings(names)
	return names
}

func decodeArray(raw json.RawMessage) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("rawapi: decode array field: %w", err)
	}
	return items, nil
}

func nextPageToken(obj map[string]json.RawMessage) (token, requestKey string) {
	if token = jsonString(obj["nextPageToken"]); token != "" {
		return token, "pageToken"
	}
	if token = jsonString(obj["next_page_token"]); token != "" {
		return token, "page_token"
	}
	return "", ""
}

func jsonString(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func encodeCombined(first map[string]json.RawMessage, field string, items []json.RawMessage) ([]byte, error) {
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("rawapi: encode items: %w", err)
	}
	out := make(map[string]json.RawMessage, len(first))
	for key, value := range first {
		if key == "nextPageToken" || key == "next_page_token" {
			continue
		}
		out[key] = value
	}
	out[field] = encoded
	body, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("rawapi: encode page: %w", err)
	}
	return body, nil
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for key, values := range in {
		out[key] = append([]string{}, values...)
	}
	return out
}
