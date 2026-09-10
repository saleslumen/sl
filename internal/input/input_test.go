package input

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

func TestReadSources(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		raw, err := Read(strings.NewReader(`{"ignored":true}`), "")
		if err != nil || raw != nil {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
	t.Run("stdin", func(t *testing.T) {
		raw, err := Read(strings.NewReader(" \n{\"name\":\"stdin\"}\r\n"), "-")
		if err != nil || string(raw) != `{"name":"stdin"}` {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte("\n[1,2]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		raw, err := Read(nil, path)
		if err != nil || string(raw) != `[1,2]` {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
}

func TestReadRejectsInvalidJSON(t *testing.T) {
	for _, raw := range []string{"", " ", "not-json", `{"open":`} {
		t.Run(raw, func(t *testing.T) {
			_, err := Read(strings.NewReader(raw), "-")
			var usage *cli.UsageError
			if !errors.As(err, &usage) || usage.Error() != "--input must be JSON" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReadObject(t *testing.T) {
	empty, err := ReadObject(nil, "")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%v err=%v", empty, err)
	}
	object, err := ReadObject(strings.NewReader(`{"name":"Ada"}`), "-")
	if err != nil || object.String("name") != "Ada" {
		t.Fatalf("object=%v err=%v", object, err)
	}
	for _, raw := range []string{"null", "[]", `"text"`, "1"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ReadObject(strings.NewReader(raw), "-")
			var usage *cli.UsageError
			if !errors.As(err, &usage) || usage.Error() != "--input must be a JSON object" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestObjectMutationAndEncoding(t *testing.T) {
	object := Object{"nullable": json.RawMessage("null")}
	if err := object.Set("name", "Ada"); err != nil {
		t.Fatal(err)
	}
	if err := object.SetAbsent("name", "Grace"); err != nil {
		t.Fatal(err)
	}
	if err := object.SetAbsent("enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := object.SetAbsent("nullable", "replacement"); err != nil {
		t.Fatal(err)
	}
	if object.String("name") != "Ada" || !object.Has("enabled") {
		t.Fatalf("object=%v", object)
	}
	if string(object["nullable"]) != "null" {
		t.Fatalf("nullable=%s", object["nullable"])
	}
	raw, err := object.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["name"] != "Ada" || decoded["enabled"] != true {
		t.Fatalf("decoded=%v", decoded)
	}
}

func TestObjectMergeFlag(t *testing.T) {
	object := Object{"name": json.RawMessage(`"input"`)}
	err := object.MergeFlag("name", "display-name", "flag", true)
	var usage *cli.UsageError
	want := "--display-name conflicts with name in --input"
	if !errors.As(err, &usage) || usage.Error() != want {
		t.Fatalf("err=%v", err)
	}
	if object.String("name") != "input" {
		t.Fatalf("name=%q", object.String("name"))
	}
	if err := object.MergeFlag("description", "description", "value", false); err != nil {
		t.Fatal(err)
	}
	if object.Has("description") {
		t.Fatal("unchanged flag mutated object")
	}
	if err := object.MergeFlag("description", "description", "value", true); err != nil {
		t.Fatal(err)
	}
	if object.String("description") != "value" {
		t.Fatalf("description=%q", object.String("description"))
	}
}

func TestObjectStringRejectsNullAndNonString(t *testing.T) {
	object := Object{
		"null":   json.RawMessage("null"),
		"number": json.RawMessage("42"),
		"string": json.RawMessage(`"value"`),
	}
	if object.String("missing") != "" || object.String("null") != "" || object.String("number") != "" {
		t.Fatal("non-string field returned text")
	}
	if object.String("string") != "value" {
		t.Fatalf("string=%q", object.String("string"))
	}
}
