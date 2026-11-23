package emails

import (
	"encoding/json"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
)

func readRequiredInput(f *cli.Factory, source string) (json.RawMessage, error) {
	raw, err := input.Read(f.IO.In, source)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	return json.RawMessage(raw), nil
}

func readRequiredObject(f *cli.Factory, source string) (input.Object, error) {
	if source == "" {
		return nil, &cli.UsageError{Msg: "--input is required"}
	}
	return input.ReadObject(f.IO.In, source)
}

func readRequiredArray(f *cli.Factory, source string) (json.RawMessage, error) {
	raw, err := readRequiredInput(f, source)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &items) != nil {
		return nil, &cli.UsageError{Msg: "--input must be a JSON array"}
	}
	return raw, nil
}
