package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func DecodePage[T any](body []byte, itemsField, nextTokenField string) ([]T, string, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", fmt.Errorf("apiclient: decode page: %w", err)
	}
	if payload == nil {
		return nil, "", fmt.Errorf("apiclient: page must be a JSON object")
	}
	items := []T{}
	if raw := bytes.TrimSpace(payload[itemsField]); len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, "", fmt.Errorf("apiclient: decode page field %s: %w", itemsField, err)
		}
	}
	var next string
	if raw := bytes.TrimSpace(payload[nextTokenField]); len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, "", fmt.Errorf("apiclient: decode page token %s: %w", nextTokenField, err)
		}
	}
	return items, next, nil
}
