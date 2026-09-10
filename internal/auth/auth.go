package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var ErrNoStore = errors.New("auth: store resolver is not configured")

type Streams struct {
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
	IsStdinTTY bool
}

type ProductProbe func(ctx context.Context, product, method, path string, query url.Values) error

type Deps struct {
	Streams      Streams
	Store        func() (credentials.Store, error)
	Profile      func() string
	Organization func() string
	Resolve      func() (credentials.Resolved, error)
	Probe        ProductProbe
	ReadSecret   func() (string, error)
}

type probeRoute struct {
	path     string
	queryKey string
	queryVal string
}

func lookupProbe(product string) (probeRoute, bool) {
	switch product {
	case "campaigns":
		return probeRoute{path: "/v1/campaigns", queryKey: "page_size", queryVal: "1"}, true
	case "emails":
		return probeRoute{path: "/v1/accounts"}, true
	case "workflows":
		return probeRoute{path: "/v1/workflows", queryKey: "pageSize", queryVal: "1"}, true
	case "script":
		return probeRoute{path: "/v1/projects", queryKey: "pageSize", queryVal: "1"}, true
	default:
		return probeRoute{}, false
	}
}

func NewCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate sl", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newLoginCommand(d), newStatusCommand(d), newLogoutCommand(d))
	return cmd
}

func newLoginCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "login", Short: "Record an API key", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().Bool("with-token", false, "Read the API key from standard input")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLogin(cmd, d)
	}
	return cmd
}

func newStatusCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "status", Short: "Show authentication status", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().String("product", "", "Product to probe with a one-item list")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd, d)
	}
	return cmd
}

func newLogoutCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "logout", Short: "Remove a credentials profile", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLogout(d)
	}
	return cmd
}

func runLogin(cmd *cobra.Command, d Deps) error {
	withToken, err := cmd.Flags().GetBool("with-token")
	if err != nil {
		return err
	}
	profile, err := profileName(d)
	if err != nil {
		return err
	}
	key, err := readToken(d, withToken)
	if err != nil {
		return err
	}
	if err := credentials.ValidateAPIKey(key); err != nil {
		return &cli.UsageError{Msg: err.Error()}
	}
	store, err := openStore(d)
	if err != nil {
		return err
	}
	file, err := store.Load()
	if err != nil {
		return err
	}
	file.Set(profile, credentials.KeyAPIKey, key)
	if organization := organizationID(d); organization != "" {
		file.Set(profile, credentials.KeyOrganizationID, organization)
	}
	if err := store.Save(file); err != nil {
		return err
	}
	fmt.Fprintf(d.Streams.Out, "Logged in to profile %s\n", profile)
	return nil
}

func runStatus(cmd *cobra.Command, d Deps) error {
	product, err := cmd.Flags().GetString("product")
	if err != nil {
		return err
	}
	if d.Resolve == nil {
		return fmt.Errorf("auth: credentials resolver is not configured")
	}
	resolved, err := d.Resolve()
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Streams.Out, "profile: %s\n", resolved.Profile)
	fmt.Fprintf(d.Streams.Out, "api_key: %s\n", credentials.KeyPrefix(resolved.APIKey))
	fmt.Fprintf(d.Streams.Out, "organization_id: %s\n", resolved.OrganizationID)
	fmt.Fprintf(d.Streams.Out, "namespace_id: %s\n", resolved.NamespaceID)
	fmt.Fprintf(d.Streams.Out, "api_domain: %s\n", resolved.APIDomain)
	if product == "" {
		return nil
	}
	return writeProbe(cmd.Context(), d, product)
}

func runLogout(d Deps) error {
	profile, err := profileName(d)
	if err != nil {
		return err
	}
	store, err := openStore(d)
	if err != nil {
		return err
	}
	file, err := store.Load()
	if err != nil {
		return err
	}
	if !file.Has(profile) {
		return fmt.Errorf("auth: profile %q is not logged in", profile)
	}
	file.RemoveProfile(profile)
	if err := store.Save(file); err != nil {
		return err
	}
	fmt.Fprintf(d.Streams.Out, "Logged out of profile %s\n", profile)
	return nil
}

func writeProbe(ctx context.Context, d Deps, product string) error {
	route, ok := lookupProbe(product)
	if !ok {
		return &cli.UsageError{Msg: fmt.Sprintf("unknown product %q", product)}
	}
	if d.Probe == nil {
		return fmt.Errorf("auth: product probe is not configured")
	}
	query := url.Values{}
	if route.queryKey != "" {
		query.Set(route.queryKey, route.queryVal)
	}
	if err := d.Probe(ctx, product, http.MethodGet, route.path, query); err != nil {
		return err
	}
	fmt.Fprintf(d.Streams.Out, "product %s: ok\n", product)
	return nil
}

func readToken(d Deps, withToken bool) (string, error) {
	if withToken {
		return readStdinToken(d.Streams.In)
	}
	if !d.Streams.IsStdinTTY {
		return "", &cli.UsageError{Msg: "login requires a terminal or --with-token"}
	}
	if d.ReadSecret == nil {
		return "", fmt.Errorf("auth: secret reader is not configured")
	}
	return d.ReadSecret()
}

func readStdinToken(r io.Reader) (string, error) {
	if r == nil {
		return "", &cli.UsageError{Msg: credentials.ErrInvalidAPIKey.Error()}
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("auth: read token: %w", err)
	}
	return "", &cli.UsageError{Msg: credentials.ErrInvalidAPIKey.Error()}
}

func ReadTerminalSecret(s Streams) (string, error) {
	file, ok := s.In.(*os.File)
	if !ok {
		return "", &cli.UsageError{Msg: "login requires a terminal or --with-token"}
	}
	fd := int(file.Fd())
	if !term.IsTerminal(fd) {
		return "", &cli.UsageError{Msg: "login requires a terminal or --with-token"}
	}
	if s.Err != nil {
		fmt.Fprint(s.Err, "API key: ")
	}
	secret, err := term.ReadPassword(fd)
	if s.Err != nil {
		fmt.Fprintln(s.Err)
	}
	if err != nil {
		return "", fmt.Errorf("auth: read token: %w", err)
	}
	return strings.TrimSpace(string(secret)), nil
}

func openStore(d Deps) (credentials.Store, error) {
	if d.Store == nil {
		return credentials.Store{}, ErrNoStore
	}
	return d.Store()
}

func profileName(d Deps) (string, error) {
	if d.Profile == nil {
		return "", fmt.Errorf("auth: profile resolver is not configured")
	}
	name := d.Profile()
	if name == "" {
		return "", fmt.Errorf("auth: profile is empty")
	}
	return name, nil
}

func organizationID(d Deps) string {
	if d.Organization == nil {
		return ""
	}
	return d.Organization()
}
