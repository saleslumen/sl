package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

const (
	defaultListLimit = 10
	emailsPageMax    = 100
)

func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "emails", Short: "Manage emails", SilenceUsage: true, SilenceErrors: true}
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &cli.UsageError{Msg: err.Error()}
	})
	cmd.AddCommand(newAccountsCommand(f))
	cmd.AddCommand(newMessagesCommand(f))
	cmd.AddCommand(newDraftsCommand(f))
	cmd.AddCommand(newAttachmentsCommand(f))
	cmd.AddCommand(newThreadsCommand(f))
	cmd.AddCommand(newLabelsCommand(f))
	cmd.AddCommand(newToolsCommand(f))
	return cmd
}

func doRequest(ctx context.Context, f *cli.Factory, method, path string, query url.Values, body any) (*apiclient.Response, error) {
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(ctx, apiclient.Request{Product: "emails", Method: method, Path: path, Query: query, Body: body})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func printObject(f *cli.Factory, raw []byte) error {
	return f.Printer().Object(raw)
}

func printTable[T any](f *cli.Factory, raw []byte, rows []T, cols []output.Column[T]) error {
	return output.Table(f.Printer(), raw, rows, cols)
}

func itemPath(collection, id string) string {
	return collection + "/" + url.PathEscape(id)
}

func validateLimit(limit int) error {
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	return nil
}

func requirePathID(id, name string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", &cli.UsageError{Msg: name + " is required"}
	}
	return id, nil
}

func requestID(f *cli.Factory, value string) (string, error) {
	if value != "" {
		return value, nil
	}
	value = strings.TrimSpace(f.NewRequestID())
	if value == "" {
		return "", fmt.Errorf("emails: generated request id is empty")
	}
	return value, nil
}

func pageSize(limit, fetched int) int {
	if limit <= 0 {
		return emailsPageMax
	}
	remaining := min(limit-fetched, emailsPageMax)
	if remaining < 1 {
		return 1
	}
	return remaining
}

func jsonScalarText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return string(raw)
}
