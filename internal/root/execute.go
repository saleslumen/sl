package root

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func Execute(products ...Product) int {
	ios := systemIO()
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f, products...)
	return run(os.Args[1:], f, command)
}

func run(args []string, f *cli.Factory, command *cobra.Command) int {
	command.SetArgs(args)
	command.SetIn(f.IO.In)
	command.SetOut(f.IO.Out)
	command.SetErr(f.IO.Err)
	return finish(command.Execute(), f.IO, command)
}

func bindFactory(f *cli.Factory, command func() *cobra.Command) {
	f.NewRequestID = uuid.NewString
	f.Printer = func() *output.Printer {
		jsonMode, _ := command().PersistentFlags().GetBool("json")
		jq, _ := command().PersistentFlags().GetString("jq")
		return output.New(f.IO.Out, output.Options{JSON: jsonMode, JQ: jq, TTY: f.IO.IsStdoutTTY})
	}
	f.Confirm = func(prompt string) error {
		yes, _ := command().PersistentFlags().GetBool("yes")
		return confirm(f.IO, yes, prompt)
	}
	f.Client = func() (*apiclient.Client, error) {
		return newClient(command(), f)
	}
	f.Organization = func() (string, error) {
		resolved, err := resolveProfileFromRoot(command())
		if err != nil {
			return "", err
		}
		if resolved.OrganizationID == "" {
			return "", &cli.UsageError{Msg: "organization required; pass --organization or run 'sl config set organization_id <id>'"}
		}
		return resolved.OrganizationID, nil
	}
}

func newClient(command *cobra.Command, f *cli.Factory) (*apiclient.Client, error) {
	resolved, err := resolveFromRoot(command)
	if err != nil {
		if errors.Is(err, credentials.ErrNoCredentials) {
			return nil, cli.ErrNoCredentials
		}
		return nil, err
	}
	var logger func(string)
	if verbose, _ := command.PersistentFlags().GetBool("verbose"); verbose {
		logger = func(line string) {
			fmt.Fprintln(f.IO.Err, line)
		}
	}
	return apiclient.New(apiclient.Options{
		APIKey:      resolved.APIKey,
		NamespaceID: resolved.NamespaceID,
		APIDomain:   resolved.APIDomain,
		UserAgent:   "sl/" + cli.Version(),
		Logger:      logger,
	}), nil
}

func resolveFromRoot(command *cobra.Command) (credentials.Resolved, error) {
	resolved, err := resolveProfileFromRoot(command)
	if err != nil {
		return credentials.Resolved{}, err
	}
	if resolved.APIKey == "" {
		return credentials.Resolved{}, credentials.ErrNoCredentials
	}
	return resolved, nil
}

func resolveProfileFromRoot(command *cobra.Command) (credentials.Resolved, error) {
	env := credentials.EnvFrom(os.Getenv)
	store, err := credentials.NewStore(env, os.UserHomeDir)
	if err != nil {
		return credentials.Resolved{}, err
	}
	file, err := store.Load()
	if err != nil {
		return credentials.Resolved{}, err
	}
	return credentials.ResolveProfile(credentials.Flags{
		Profile:        flagValue(command, "profile"),
		OrganizationID: flagValue(command, "organization"),
		NamespaceID:    flagValue(command, "namespace"),
	}, env, file), nil
}

func resolveStore() (credentials.Store, error) {
	store, err := credentials.NewStore(credentials.EnvFrom(os.Getenv), os.UserHomeDir)
	if err != nil {
		return credentials.Store{}, err
	}
	if store.Dir == "" {
		return credentials.Store{}, fmt.Errorf("credentials directory is empty")
	}
	return store, nil
}

func flagValue(command *cobra.Command, name string) string {
	flag := command.PersistentFlags().Lookup(name)
	if flag == nil || !flag.Changed {
		return ""
	}
	return flag.Value.String()
}

func systemIO() *cli.IOStreams {
	return &cli.IOStreams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, IsStdinTTY: isTTY(os.Stdin), IsStdoutTTY: isTTY(os.Stdout)}
}

func isTTY(file *os.File) bool {
	return term.IsTerminal(int(file.Fd()))
}

func confirm(ios *cli.IOStreams, yes bool, prompt string) error {
	if yes {
		return nil
	}
	if !ios.IsStdinTTY {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}
	if _, err := fmt.Fprintf(ios.Err, "%s [y/N] ", prompt); err != nil {
		return fmt.Errorf("write confirm prompt: %w", err)
	}
	line, err := readLine(ios.In)
	if err != nil {
		return fmt.Errorf("read confirm: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return cli.ErrCancelled
	}
}

func readLine(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return scanner.Text(), nil
}

func finish(err error, ios *cli.IOStreams, command *cobra.Command) int {
	if err == nil {
		return 0
	}
	verbose, _ := command.PersistentFlags().GetBool("verbose")
	writeCommandError(ios.Err, err, verbose)
	return exitCode(err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, cli.ErrNoCredentials) || errors.Is(err, credentials.ErrNoCredentials) {
		return 4
	}
	var usage *cli.UsageError
	if errors.As(err, &usage) {
		return 2
	}
	var api *apiclient.Error
	if errors.As(err, &api) {
		if api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden {
			return 4
		}
	}
	return 1
}

func writeCommandError(writer io.Writer, err error, verbose bool) {
	var api *apiclient.Error
	if errors.As(err, &api) {
		fmt.Fprintf(writer, "sl: %s %s %s: %s: %s\n", api.Product, api.Method, api.Path, api.Code, api.Message)
		if verbose {
			fmt.Fprintf(writer, "HTTP %d\n", api.Status)
			if api.RequestID != "" {
				fmt.Fprintf(writer, "request id: %s\n", api.RequestID)
			}
		}
		return
	}
	fmt.Fprintf(writer, "sl: %s\n", err.Error())
}
