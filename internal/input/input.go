package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/saleslumen/sl/internal/cli"
)

type Object map[string]json.RawMessage

func Read(stdin io.Reader, source string) ([]byte, error) {
	if source == "" {
		return nil, nil
	}
	var raw []byte
	var err error
	if source == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("input: stdin is required for --input -")
		}
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, fmt.Errorf("input: read --input: %w", err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, &cli.UsageError{Msg: "--input must be JSON"}
	}
	return raw, nil
}

func ReadObject(stdin io.Reader, source string) (Object, error) {
	raw, err := Read(stdin, source)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return Object{}, nil
	}
	var object Object
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, &cli.UsageError{Msg: "--input must be a JSON object"}
	}
	return object, nil
}

func (o Object) Has(field string) bool {
	_, ok := o[field]
	return ok
}

func (o Object) String(field string) string {
	raw, ok := o[field]
	if !ok {
		return ""
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func (o Object) Set(field string, value any) error {
	if o == nil {
		return fmt.Errorf("input: object is nil")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("input: encode %s: %w", field, err)
	}
	o[field] = raw
	return nil
}

func (o Object) SetAbsent(field string, value any) error {
	if o.Has(field) {
		return nil
	}
	return o.Set(field, value)
}

func (o Object) MergeFlag(field, flag string, value any, changed bool) error {
	if !changed {
		return nil
	}
	if o.Has(field) {
		return &cli.UsageError{Msg: fmt.Sprintf("--%s conflicts with %s in --input", flag, field)}
	}
	return o.Set(field, value)
}

func (o Object) Encode() (json.RawMessage, error) {
	if o == nil {
		o = Object{}
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("input: encode object: %w", err)
	}
	return raw, nil
}
