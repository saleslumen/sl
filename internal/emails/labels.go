package emails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	foundationinput "github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type label struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Type string          `json:"type"`
	Etag json.RawMessage `json:"etag"`
}

func newLabelsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "labels", Short: "Manage organization labels"}
	cmd.AddCommand(newLabelsListCommand(f), newLabelsGetCommand(f), newLabelsCreateCommand(f), newLabelsUpdateCommand(f), newLabelsDeleteCommand(f))
	return cmd
}

func newLabelsListCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List labels", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLabelsList(cmd.Context(), f)
	}
	return cmd
}

func newLabelsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get a label", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLabelsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newLabelsCreateCommand(f *cli.Factory) *cobra.Command {
	var name, input string
	cmd := &cobra.Command{Use: "create [--name NAME | --input FILE]", Short: "Create a user label", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&name, "name", "", "Label name")
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLabelsCreate(cmd.Context(), f, name, input, cmd.Flags().Changed("name"))
	}
	return cmd
}

func newLabelsUpdateCommand(f *cli.Factory) *cobra.Command {
	var input, etag, updateMask string
	cmd := &cobra.Command{Use: "update ID --input FILE", Short: "Update a user label", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&etag, "etag", "", "Label etag (fetched if omitted)")
	cmd.Flags().StringVar(&updateMask, "update-mask", "name", "Field mask")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLabelsUpdate(cmd.Context(), f, args[0], input, etag, updateMask, cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newLabelsDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a user label", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLabelsDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func runLabelsList(ctx context.Context, f *cli.Factory) error {
	resp, err := doRequest(ctx, f, http.MethodGet, "/v1/labels", nil, nil)
	if err != nil {
		return err
	}
	labels, _, err := apiclient.DecodePage[json.RawMessage](resp.Body, "labels", "nextPageToken")
	if err != nil {
		return err
	}
	return printLabelList(f, labels)
}

func runLabelsGet(ctx context.Context, f *cli.Factory, id string) error {
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/labels", id), nil, nil)
	if err != nil {
		return err
	}
	return printLabel(f, resp.Body)
}

func runLabelsCreate(ctx context.Context, f *cli.Factory, name, input string, nameSet bool) error {
	body, err := foundationinput.ReadObject(f.IO.In, input)
	if err != nil {
		return err
	}
	nested, err := nestedLabelObject(body)
	if err != nil {
		return err
	}
	if err := nested.MergeFlag("name", "name", name, nameSet); err != nil {
		return err
	}
	if labelName(nested) == "" {
		return &cli.UsageError{Msg: "--name or --input label.name is required"}
	}
	if err := body.Set("label", nested); err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/labels", nil, raw)
	if err != nil {
		return err
	}
	return printLabel(f, resp.Body)
}

func runLabelsUpdate(ctx context.Context, f *cli.Factory, id, input, etag, updateMask string, etagSet bool) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("etag", "etag", etag, etagSet); err != nil {
		return err
	}
	if err := setAbsentFetchedEtag(body, func() (json.RawMessage, error) { return fetchLabelEtag(ctx, f, id) }); err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("updateMask", updateMask)
	resp, err := doRequest(ctx, f, http.MethodPatch, itemPath("/v1/labels", id), query, raw)
	if err != nil {
		return err
	}
	return printLabel(f, resp.Body)
}

func runLabelsDelete(ctx context.Context, f *cli.Factory, id string) error {
	if err := f.Confirm(fmt.Sprintf("Delete label %s?", id)); err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, itemPath("/v1/labels", id), nil, nil)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func labelColumns() []output.Column[label] {
	return []output.Column[label]{
		{Header: "ID", Value: func(row label) string { return row.ID }},
		{Header: "NAME", Value: func(row label) string { return row.Name }},
		{Header: "TYPE", Value: func(row label) string { return row.Type }},
		{Header: "ETAG", Value: func(row label) string { return jsonScalarText(row.Etag) }},
	}
}

func printLabel(f *cli.Factory, raw []byte) error {
	var row label
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("emails: decode label: %w", err)
	}
	return printTable(f, raw, []label{row}, labelColumns())
}

func printLabelList(f *cli.Factory, raws []json.RawMessage) error {
	rows := make([]label, 0, len(raws))
	for _, raw := range raws {
		var row label
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("emails: decode label: %w", err)
		}
		rows = append(rows, row)
	}
	out, err := json.Marshal(struct {
		Labels []json.RawMessage `json:"labels"`
	}{Labels: raws})
	if err != nil {
		return fmt.Errorf("emails: encode labels: %w", err)
	}
	return printTable(f, out, rows, labelColumns())
}

func nestedLabelObject(body foundationinput.Object) (foundationinput.Object, error) {
	raw, ok := body["label"]
	if !ok {
		return foundationinput.Object{}, nil
	}
	var nested foundationinput.Object
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &nested) != nil || nested == nil {
		return nil, &cli.UsageError{Msg: "--input label must be a JSON object"}
	}
	return nested, nil
}

func labelName(nested foundationinput.Object) string {
	return strings.TrimSpace(nested.String("name"))
}

func fetchLabelEtag(ctx context.Context, f *cli.Factory, id string) (json.RawMessage, error) {
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/labels", id), nil, nil)
	if err != nil {
		return nil, err
	}
	etag, err := resourceEtag(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("emails: label %s: %w", id, err)
	}
	return etag, nil
}
