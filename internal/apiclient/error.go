package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type Error struct {
	Status                int
	Code                  string
	Message               string
	Details               json.RawMessage
	RequestID             string
	Product, Method, Path string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s %s %s: %s: %s", e.Product, e.Method, e.Path, e.Code, e.Message)
}

func normalizeError(status int, body []byte, product, method, path, requestID string) (result *Error) {
	fallback := http.StatusText(status)
	err := &Error{Status: status, Code: fallback, Message: fallback, RequestID: requestID, Product: product, Method: method, Path: path}
	defer func() {
		err.Code = oneLine(err.Code)
		err.Message = oneLine(err.Message)
		err.RequestID = oneLine(err.RequestID)
		result = err
	}()
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return err
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(trimmed, &top) != nil {
		err.Message = string(trimmed)
		return err
	}
	if codeRaw, ok := top["code"]; ok && len(bytes.TrimSpace(codeRaw)) > 0 {
		err.Code = canonicalCode(codeRaw, err.Code)
		if msg := decodeJSONString(top["message"]); msg != "" {
			err.Message = msg
		}
		err.Details = optionalDetails(top["details"])
		return err
	}
	errorRaw, ok := top["error"]
	if !ok {
		err.Message = string(trimmed)
		return err
	}
	if msg := decodeJSONString(errorRaw); msg != "" {
		err.Message = msg
		return err
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(errorRaw, &nested) != nil {
		err.Message = string(trimmed)
		return err
	}
	if codeRaw, exists := nested["code"]; exists && len(bytes.TrimSpace(codeRaw)) > 0 {
		err.Code = canonicalCode(codeRaw, err.Code)
	}
	if msg := decodeJSONString(nested["message"]); msg != "" {
		err.Message = msg
	}
	err.Details = optionalDetails(nested["details"])
	return err
}

func oneLine(value string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value))
}

func canonicalCode(raw json.RawMessage, fallback string) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return fallback
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return fallback
		}
		if n, err := strconv.Atoi(s); err == nil {
			return grpcName(n, s)
		}
		return s
	}
	var n int
	if json.Unmarshal(raw, &n) != nil {
		return fallback
	}
	return grpcName(n, strconv.Itoa(n))
}

func grpcName(n int, fallback string) string {
	switch n {
	case 0:
		return "OK"
	case 1:
		return "CANCELLED"
	case 2:
		return "UNKNOWN"
	case 3:
		return "INVALID_ARGUMENT"
	case 4:
		return "DEADLINE_EXCEEDED"
	case 5:
		return "NOT_FOUND"
	case 6:
		return "ALREADY_EXISTS"
	case 7:
		return "PERMISSION_DENIED"
	case 8:
		return "RESOURCE_EXHAUSTED"
	case 9:
		return "FAILED_PRECONDITION"
	case 10:
		return "ABORTED"
	case 11:
		return "OUT_OF_RANGE"
	case 12:
		return "UNIMPLEMENTED"
	case 13:
		return "INTERNAL"
	case 14:
		return "UNAVAILABLE"
	case 15:
		return "DATA_LOSS"
	case 16:
		return "UNAUTHENTICATED"
	default:
		return fallback
	}
}

func decodeJSONString(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func optionalDetails(raw json.RawMessage) json.RawMessage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
