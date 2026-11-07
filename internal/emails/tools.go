package emails

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newToolsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "tools", Short: "Run Emails tools"}
	cmd.AddCommand(newToolsDiscoverCommand(f))
	cmd.AddCommand(newToolsVerifyCommand(f))
	cmd.AddCommand(newToolsIMAPCommand(f))
	cmd.AddCommand(newToolsSMTPCommand(f))
	return cmd
}

func newToolsDiscoverCommand(f *cli.Factory) *cobra.Command {
	var domain, name string
	cmd := &cobra.Command{Use: "discover --domain DOMAIN --name NAME", Short: "Discover an email address", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&domain, "domain", "", "Domain to search")
	cmd.Flags().StringVar(&name, "name", "", "Person name to search")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runToolsDiscover(cmd.Context(), f, domain, name)
	}
	return cmd
}

func newToolsIMAPCommand(f *cli.Factory) *cobra.Command {
	return newToolsInputPostCommand(f, "imap --input FILE", "Test an IMAP connection", "/v1/tools:imap")
}

func newToolsVerifyCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "verify --input FILE", Short: "Verify email addresses", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runToolsVerify(cmd.Context(), f, input)
	}
	return cmd
}

func newToolsSMTPCommand(f *cli.Factory) *cobra.Command {
	return newToolsInputPostCommand(f, "smtp --input FILE", "Test an SMTP connection", "/v1/tools:smtp")
}

func newToolsInputPostCommand(f *cli.Factory, use, short, path string) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runToolsInputPost(cmd.Context(), f, path, input)
	}
	return cmd
}

func runToolsDiscover(ctx context.Context, f *cli.Factory, domain, name string) error {
	domain = strings.TrimSpace(domain)
	name = strings.TrimSpace(name)
	if domain == "" || name == "" {
		return &cli.UsageError{Msg: "--domain and --name are required"}
	}
	query := url.Values{}
	query.Set("domain", domain)
	query.Set("name", name)
	resp, err := doRequest(ctx, f, http.MethodGet, "/v1/tools:discover", query, nil)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runToolsVerify(ctx context.Context, f *cli.Factory, input string) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	request := apiclient.Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools:verify", Body: raw}
	return client.StreamLines(ctx, request, func(line []byte) error {
		return f.Printer().Object(append(bytes.Clone(line), '\n'))
	})
}

func runToolsInputPost(ctx context.Context, f *cli.Factory, path, input string) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path, nil, raw)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}
