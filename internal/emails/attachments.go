package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type messageAttachment struct {
	ID   string
	Size string
	Data string
}

func newAttachmentsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "attachments", Short: "Get message attachments"}
	cmd.AddCommand(newAttachmentsGetCommand(f))
	return cmd
}

func newAttachmentsGetCommand(f *cli.Factory) *cobra.Command {
	var messageID string
	cmd := &cobra.Command{Use: "get ID --message MESSAGE_ID", Short: "Get a message attachment", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&messageID, "message", "", "Parent message ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAttachmentsGet(cmd.Context(), f, args[0], messageID)
	}
	return cmd
}

func runAttachmentsGet(ctx context.Context, f *cli.Factory, id, messageID string) error {
	id, err := requirePathID(id, "attachment ID")
	if err != nil {
		return err
	}
	messageID, err = requirePathID(messageID, "--message")
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath(itemPath("/v1/messages", messageID)+"/attachments", id), nil, nil)
	if err != nil {
		return err
	}
	row, err := messageAttachmentFrom(resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []messageAttachment{row}, attachmentColumns())
}

func messageAttachmentFrom(raw json.RawMessage) (messageAttachment, error) {
	var wire struct {
		AttachmentID string          `json:"attachmentId"`
		Size         json.RawMessage `json:"size"`
		Data         string          `json:"data"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return messageAttachment{}, fmt.Errorf("emails: decode attachment: %w", err)
	}
	return messageAttachment{ID: wire.AttachmentID, Size: attachmentSizeText(wire.Size), Data: wire.Data}, nil
}

func attachmentSizeText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}

func attachmentColumns() []output.Column[messageAttachment] {
	return []output.Column[messageAttachment]{
		{Header: "ATTACHMENT_ID", Value: func(row messageAttachment) string { return row.ID }},
		{Header: "SIZE", Value: func(row messageAttachment) string { return row.Size }},
		{Header: "DATA", Value: func(row messageAttachment) string { return row.Data }},
	}
}
