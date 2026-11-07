package emails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/saleslumen/sl/internal/cli"
	foundationinput "github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type account struct {
	ID                     string  `json:"id"`
	AccountID              string  `json:"account_id"`
	NamespaceID            *string `json:"namespace_id"`
	DisplayName            string  `json:"display_name"`
	EmailAddress           string  `json:"email_address"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
	OAuth2Connected        bool    `json:"oauth2_connected"`
	IMAPPasswordConfigured bool    `json:"imap_password_configured"`
	SMTPPasswordConfigured bool    `json:"smtp_password_configured"`
}

func newAccountsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Manage email accounts"}
	cmd.AddCommand(newAccountsListCommand(f))
	cmd.AddCommand(newAccountsGetCommand(f))
	cmd.AddCommand(newAccountsCreateCommand(f))
	cmd.AddCommand(newAccountsUpdateCommand(f))
	cmd.AddCommand(newAccountsDeleteCommand(f))
	cmd.AddCommand(newAccountsBatchCommand(f))
	return cmd
}

func newAccountsListCommand(f *cli.Factory) *cobra.Command {
	var email string
	cmd := &cobra.Command{Use: "list", Short: "List email accounts (the API returns at most 100)", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&email, "email", "", "Filter accounts by email address")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsList(cmd.Context(), f, email)
	}
	return cmd
}

func newAccountsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get an email account", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newAccountsCreateCommand(f *cli.Factory) *cobra.Command {
	var accountID, emailAddress, displayName, input string
	cmd := &cobra.Command{Use: "create [--input FILE | --account-id ACCOUNT_ID --email-address EMAIL --display-name NAME]", Short: "Create an email account", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Account identifier")
	cmd.Flags().StringVar(&emailAddress, "email-address", "", "Email address")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsCreate(cmd.Context(), f, accountID, emailAddress, displayName, input, cmd.Flags().Changed("account-id"), cmd.Flags().Changed("email-address"), cmd.Flags().Changed("display-name"))
	}
	return cmd
}

func newAccountsUpdateCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "update ID --input FILE", Short: "Update an email account", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsUpdate(cmd.Context(), f, args[0], input)
	}
	return cmd
}

func newAccountsDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete an email account", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func newAccountsBatchCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "batch --input FILE", Short: "Create email accounts in batch", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAccountsBatch(cmd.Context(), f, input)
	}
	return cmd
}

func runAccountsList(ctx context.Context, f *cli.Factory, email string) error {
	query := url.Values{}
	if email != "" {
		query.Set("email", email)
	}
	resp, err := doRequest(ctx, f, http.MethodGet, "/v1/accounts", query, nil)
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return fmt.Errorf("emails: decode accounts: %w", err)
	}
	rows := make([]account, 0, len(items))
	for _, item := range items {
		var row account
		if err := json.Unmarshal(item, &row); err != nil {
			return fmt.Errorf("emails: decode account: %w", err)
		}
		rows = append(rows, row)
	}
	return printTable(f, resp.Body, rows, accountListColumns())
}

func runAccountsGet(ctx context.Context, f *cli.Factory, id string) error {
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/accounts", id), nil, nil)
	if err != nil {
		return err
	}
	var row account
	if err := json.Unmarshal(resp.Body, &row); err != nil {
		return fmt.Errorf("emails: decode account: %w", err)
	}
	return printTable(f, resp.Body, []account{row}, accountGetColumns())
}

func runAccountsCreate(ctx context.Context, f *cli.Factory, accountID, emailAddress, displayName, input string, accountIDSet, emailSet, nameSet bool) error {
	body, err := foundationinput.ReadObject(f.IO.In, input)
	if err != nil {
		return err
	}
	if err := body.MergeFlag("account_id", "account-id", accountID, accountIDSet); err != nil {
		return err
	}
	if err := body.MergeFlag("email_address", "email-address", emailAddress, emailSet); err != nil {
		return err
	}
	if err := body.MergeFlag("display_name", "display-name", displayName, nameSet); err != nil {
		return err
	}
	for _, key := range []string{"account_id", "email_address", "display_name"} {
		if !body.Has(key) {
			return &cli.UsageError{Msg: fmt.Sprintf("%s is required", key)}
		}
	}
	payload, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/accounts", nil, payload)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runAccountsUpdate(ctx context.Context, f *cli.Factory, id, input string) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPut, itemPath("/v1/accounts", id), nil, raw)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runAccountsDelete(ctx context.Context, f *cli.Factory, id string) error {
	if err := f.Confirm(fmt.Sprintf("Delete account %s?", id)); err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, itemPath("/v1/accounts", id), nil, nil)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runAccountsBatch(ctx context.Context, f *cli.Factory, input string) error {
	raw, err := readRequiredArray(f, input)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/accounts:batch", nil, raw)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func accountListColumns() []output.Column[account] {
	return []output.Column[account]{
		{Header: "ID", Value: func(a account) string { return a.ID }},
		{Header: "EMAIL", Value: func(a account) string { return a.EmailAddress }},
		{Header: "DISPLAY_NAME", Value: func(a account) string { return a.DisplayName }},
		{Header: "NAMESPACE", Value: func(a account) string { return stringValue(a.NamespaceID) }},
		{Header: "OAUTH", Value: func(a account) string { return strconv.FormatBool(a.OAuth2Connected) }},
		{Header: "IMAP", Value: func(a account) string { return strconv.FormatBool(a.IMAPPasswordConfigured) }},
		{Header: "SMTP", Value: func(a account) string { return strconv.FormatBool(a.SMTPPasswordConfigured) }},
		{Header: "UPDATED", Value: func(a account) string { return a.UpdatedAt }},
	}
}

func accountGetColumns() []output.Column[account] {
	return append(accountListColumns(),
		output.Column[account]{Header: "ACCOUNT_ID", Value: func(a account) string { return a.AccountID }},
		output.Column[account]{Header: "CREATED", Value: func(a account) string { return a.CreatedAt }},
	)
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
