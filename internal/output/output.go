package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/itchyny/gojq"
)

type Column[T any] struct {
	Header string
	Value  func(T) string
}

type Options struct {
	JSON bool
	JQ   string
	TTY  bool
}

type Printer struct {
	out       io.Writer
	json      bool
	jq        string
	stdoutTTY bool
}

func New(out io.Writer, opts Options) *Printer {
	return &Printer{out: out, json: opts.JSON || opts.JQ != "", jq: opts.JQ, stdoutTTY: opts.TTY}
}

func (p *Printer) Object(raw []byte) error {
	if p.jq != "" {
		return p.applyJQ(raw)
	}
	return p.writeJSON(raw)
}

func Table[T any](p *Printer, raw []byte, rows []T, cols []Column[T]) error {
	if p.json {
		return p.Object(raw)
	}
	if _, err := io.WriteString(p.out, renderTable(p.stdoutTTY, rows, cols)); err != nil {
		return fmt.Errorf("write table: %w", err)
	}
	return nil
}

func (p *Printer) writeJSON(raw []byte) error {
	out := raw
	if p.stdoutTTY {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err == nil {
			out = pretty.Bytes()
		}
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(append([]byte{}, out...), '\n')
		}
	}
	if _, err := p.out.Write(out); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func (p *Printer) applyJQ(raw []byte) error {
	query, err := gojq.Parse(p.jq)
	if err != nil {
		return fmt.Errorf("jq: %w", err)
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode json for jq: %w", err)
	}
	iter := query.Run(payload)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, ok := v.(error); ok {
			if halt, ok := err.(*gojq.HaltError); ok && halt.Value() == nil {
				return nil
			}
			return fmt.Errorf("jq: %w", err)
		}
		line, err := formatJQValue(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(p.out, line); err != nil {
			return fmt.Errorf("write jq: %w", err)
		}
	}
}

func formatJQValue(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "null", nil
	case bool:
		return strconv.FormatBool(t), nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case uint64:
		return strconv.FormatUint(t, 10), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case *big.Int:
		return t.String(), nil
	case *big.Float:
		return t.Text('f', -1), nil
	case *big.Rat:
		return t.FloatString(16), nil
	case json.Number:
		return t.String(), nil
	case string:
		return t, nil
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("encode jq value: %w", err)
		}
		return string(b), nil
	}
}

func renderTable[T any](tty bool, rows []T, cols []Column[T]) string {
	if len(cols) == 0 {
		return ""
	}
	headers := make([]string, len(cols))
	widths := make([]int, len(cols))
	for i, col := range cols {
		headers[i] = col.Header
		if tty {
			widths[i] = utf8.RuneCountInString(col.Header)
		}
	}
	values := make([][]string, len(rows))
	for r, row := range rows {
		values[r] = make([]string, len(cols))
		for i, col := range cols {
			cell := tableCell(col.Value(row))
			values[r][i] = cell
			if tty {
				if n := utf8.RuneCountInString(cell); n > widths[i] {
					widths[i] = n
				}
			}
		}
	}
	var b strings.Builder
	if tty {
		writeAlignedRow(&b, headers, widths)
		for _, row := range values {
			writeAlignedRow(&b, row, widths)
		}
		return b.String()
	}
	for _, row := range values {
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

func tableCell(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
}

func writeAlignedRow(b *strings.Builder, cells []string, widths []int) {
	for i, cell := range cells {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(cell)
		if i < len(cells)-1 {
			if pad := widths[i] - utf8.RuneCountInString(cell); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
	}
	b.WriteByte('\n')
}
