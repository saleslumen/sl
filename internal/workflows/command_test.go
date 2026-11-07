package workflows

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/saleslumen/sl/internal/root"
	"github.com/spf13/cobra"
)

func TestRoutesClosedSurface(t *testing.T) {
	if len(routes) != 24 {
		t.Fatalf("routes=%d want 24", len(routes))
	}
	seen := map[string]struct{}{}
	for _, route := range routes {
		if route.Method == "" || route.Path == "" || route.RPC == "" {
			t.Fatalf("incomplete route %+v", route)
		}
		key := route.Method + " " + route.Path
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestWorkflowsCommandTree(t *testing.T) {
	f, _, _, calls := newLazyFactory()
	cmd := root.NewRootCommand(f, NewCommand)
	if *calls != 0 {
		t.Fatal("construction resolved factory")
	}
	wf := commandByName(cmd, "workflows")
	got := collectPaths(wf)
	want := []string{
		"sl workflows", "sl workflows create", "sl workflows delete", "sl workflows deploy", "sl workflows event-types", "sl workflows event-types list", "sl workflows executions", "sl workflows executions cancel", "sl workflows executions create", "sl workflows executions get", "sl workflows executions list", "sl workflows executions signal", "sl workflows get", "sl workflows list", "sl workflows revisions", "sl workflows revisions create", "sl workflows revisions get", "sl workflows revisions list", "sl workflows step-runs", "sl workflows step-runs get", "sl workflows step-runs list", "sl workflows triggers", "sl workflows triggers create", "sl workflows triggers delete", "sl workflows triggers get", "sl workflows triggers list", "sl workflows triggers rotate-webhook-token", "sl workflows triggers update", "sl workflows undeploy", "sl workflows update",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tree\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRetiredCommandsAreNotAliases(t *testing.T) {
	for _, args := range [][]string{{"versions"}, {"publish", "wf-1"}, {"activate", "wf-1"}, {"deactivate", "wf-1"}, {"history", "wf-1"}, {"executions", "start"}, {"executions", "resume", "e-1"}} {
		f, _, stderr, calls := newLazyFactory()
		err := executeRoot(f, append([]string{"workflows"}, args...)...)
		if err == nil {
			t.Fatalf("retired command %v succeeded", args)
		}
		if *calls != 0 {
			t.Fatalf("retired command %v resolved factory", args)
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr=%q", stderr.String())
		}
	}
}

func TestUserOAuthCommandsAreMarkedInHelp(t *testing.T) {
	for _, args := range [][]string{{"executions", "create"}, {"executions", "signal"}, {"triggers", "create"}} {
		f, _, _, _ := newLazyFactory()
		cmd := NewCommand(f)
		for _, name := range args {
			cmd = commandByName(cmd, name)
			if cmd == nil {
				t.Fatalf("missing %v", args)
			}
		}
		if !strings.Contains(cmd.Long, "user OAuth") || !strings.Contains(cmd.Long, "sl auth login") {
			t.Fatalf("%v Long=%q", args, cmd.Long)
		}
	}
}

func TestHelpDoesNotResolveFactory(t *testing.T) {
	f, stdout, stderr, calls := newLazyFactory()
	if err := executeRoot(f, "workflows", "--help"); err != nil {
		t.Fatalf("help: %v stderr=%q", err, stderr.String())
	}
	if *calls != 0 || stdout.Len() == 0 {
		t.Fatalf("calls=%d help=%q", *calls, stdout.String())
	}
}

func newLazyFactory() (*cli.Factory, *bytes.Buffer, *bytes.Buffer, *int) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	calls := 0
	resolve := func() { calls++ }
	f := &cli.Factory{
		IO:               &cli.IOStreams{In: bytes.NewReader(nil), Out: stdout, Err: stderr},
		Client:           func() (*apiclient.Client, error) { resolve(); return nil, cli.ErrNoCredentials },
		Organization:     func() (string, error) { resolve(); return "", &cli.UsageError{Msg: "organization required"} },
		Printer:          func() *output.Printer { resolve(); return output.New(stdout, output.Options{}) },
		Confirm:          func(string) error { resolve(); return cli.ErrCancelled },
		RequireUserOAuth: func() error { return nil },
	}
	return f, stdout, stderr, &calls
}

func executeRoot(f *cli.Factory, args ...string) error {
	cmd := root.NewRootCommand(f, NewCommand)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func commandByName(cmd *cobra.Command, name string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func collectPaths(cmd *cobra.Command) []string {
	var paths []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Name() == "help" || c.Hidden {
			return
		}
		paths = append(paths, c.CommandPath())
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(cmd)
	slices.Sort(paths)
	return paths
}
