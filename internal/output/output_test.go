package output

import (
	"bytes"
	"strings"
	"testing"
)

type row struct {
	ID   string
	Name string
}

func testCols() []Column[row] {
	return []Column[row]{
		{Header: "ID", Value: func(r row) string { return r.ID }},
		{Header: "NAME", Value: func(r row) string { return r.Name }},
	}
}

func TestTableTTYAlignedWithHeaders(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{TTY: true})
	err := Table(p, []byte(`{"ignored":true}`), []row{{ID: "a", Name: "beta"}, {ID: "abc", Name: "b"}}, testCols())
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	want := "ID   NAME\na    beta\nabc  b\n"
	if out.String() != want {
		t.Fatalf("stdout=%q want=%q", out.String(), want)
	}
}

func TestTableNonTTYTabSeparatedNoHeaders(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{})
	err := Table(p, nil, []row{{ID: "a", Name: "beta"}, {ID: "abc", Name: "b"}}, testCols())
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	want := "a\tbeta\nabc\tb\n"
	if out.String() != want {
		t.Fatalf("stdout=%q want=%q", out.String(), want)
	}
}

func TestTableReplacesCellControlCharacters(t *testing.T) {
	rows := []row{{ID: "a\nb", Name: "x\ty\rz"}}
	for _, tc := range []struct {
		name string
		opts Options
		want string
	}{
		{name: "tty", opts: Options{TTY: true}, want: "ID   NAME\na b  x y z\n"},
		{name: "non-tty", opts: Options{}, want: "a b\tx y z\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Table(New(&out, tc.opts), nil, rows, testCols()); err != nil {
				t.Fatalf("Table: %v", err)
			}
			if out.String() != tc.want {
				t.Fatalf("stdout=%q want=%q", out.String(), tc.want)
			}
		})
	}
}

func TestObjectJSONPrettyOnTTY(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JSON: true, TTY: true})
	if err := p.Object([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("Object: %v", err)
	}
	want := "{\n  \"a\": 1\n}\n"
	if out.String() != want {
		t.Fatalf("stdout=%q want=%q", out.String(), want)
	}
}

func TestObjectJSONRawNonTTY(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JSON: true})
	raw := []byte(`{"a":1}`)
	if err := p.Object(raw); err != nil {
		t.Fatalf("Object: %v", err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("stdout=%q want=%q", out.Bytes(), raw)
	}
}

func TestTableJSONUsesRawBody(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JSON: true})
	raw := []byte(`{"items":[{"id":"1"}]}`)
	if err := Table(p, raw, []row{{ID: "ignored", Name: "x"}}, testCols()); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("stdout=%q want=%q", out.Bytes(), raw)
	}
}

func TestJQImpliesJSON(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JQ: ".name"})
	if err := Table(p, []byte(`{"name":"Ada"}`), []row{{ID: "1", Name: "Ada"}}, testCols()); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if out.String() != "Ada\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestJQScalarsOnePerLine(t *testing.T) {
	cases := []struct {
		name string
		jq   string
		raw  string
		want string
	}{
		{name: "string", jq: ".name", raw: `{"name":"Ada"}`, want: "Ada\n"},
		{name: "number", jq: ".n", raw: `{"n":2}`, want: "2\n"},
		{name: "bool", jq: ".ok", raw: `{"ok":true}`, want: "true\n"},
		{name: "null", jq: ".missing", raw: `{}`, want: "null\n"},
		{name: "array scalars", jq: ".items[]", raw: `{"items":[1,2]}`, want: "1\n2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := New(&out, Options{JSON: true, JQ: tc.jq})
			if err := p.Object([]byte(tc.raw)); err != nil {
				t.Fatalf("Object: %v", err)
			}
			if out.String() != tc.want {
				t.Fatalf("stdout=%q want=%q", out.String(), tc.want)
			}
		})
	}
}

func TestJQObjectCompactJSON(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JSON: true, JQ: "."})
	if err := p.Object([]byte(`{"name":"Ada"}`)); err != nil {
		t.Fatalf("Object: %v", err)
	}
	if out.String() != "{\"name\":\"Ada\"}\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestJQInvalidExpression(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{JSON: true, JQ: "[[["})
	err := p.Object([]byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "jq:") {
		t.Fatalf("err=%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTableEmptyNonTTY(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{})
	if err := Table(p, nil, []row{}, testCols()); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if out.String() != "" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTableEmptyTTYHeadersOnly(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, Options{TTY: true})
	if err := Table(p, nil, []row{}, testCols()); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if out.String() != "ID  NAME\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}
