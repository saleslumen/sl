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
	uncovered := map[string]string{}
	for _, r := range routes {
		if r.Method == "" || r.Path == "" || r.RPC == "" {
			t.Fatalf("incomplete route %+v", r)
		}
		key := r.Method + " " + r.Path
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = struct{}{}
		if r.Covered {
			if r.Reason != "" {
				t.Fatalf("covered %s has reason %q", r.RPC, r.Reason)
			}
			continue
		}
		uncovered[r.RPC] = r.Reason
	}
	want := map[string]string{
		"StartExecution":        "user credential required",
		"ResumeExecution":       "user credential required",
		"CreateWorkflowTrigger": "user credential required",
		"InvokeWebhook":         "anonymous webhook ingress",
	}
	if len(uncovered) != len(want) {
		t.Fatalf("uncovered=%d want %d: %v", len(uncovered), len(want), uncovered)
	}
	for rpc, reason := range want {
		if uncovered[rpc] != reason {
			t.Fatalf("%s reason %q want %q", rpc, uncovered[rpc], reason)
		}
	}
}

func TestWorkflowsCommandTree(t *testing.T) {
	f, _, _, calls := newLazyFactory()
	cmd := root.NewRootCommand(f, NewCommand)
	if *calls != 0 {
		t.Fatal("construction resolved factory")
	}
	wf := commandByName(cmd, "workflows")
	if wf == nil {
		t.Fatal("workflows missing")
	}
	got := collectPaths(wf)
	want := []string{
		"sl workflows",
		"sl workflows activate",
		"sl workflows create",
		"sl workflows deactivate",
		"sl workflows delete",
		"sl workflows event-types",
		"sl workflows event-types list",
		"sl workflows executions",
		"sl workflows executions cancel",
		"sl workflows executions get",
		"sl workflows executions list",
		"sl workflows get",
		"sl workflows history",
		"sl workflows list",
		"sl workflows publish",
		"sl workflows triggers",
		"sl workflows triggers delete",
		"sl workflows triggers get",
		"sl workflows triggers list",
		"sl workflows triggers rotate-webhook-token",
		"sl workflows triggers update",
		"sl workflows update",
		"sl workflows versions",
		"sl workflows versions get",
		"sl workflows versions revert",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tree\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNoExcludedCommands(t *testing.T) {
	f, _, _, _ := newLazyFactory()
	cmd := root.NewRootCommand(f, NewCommand)
	wf := commandByName(cmd, "workflows")
	forbidden := map[string]struct{}{"start": {}, "resume": {}, "hooks": {}, "invoke-webhook": {}}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Name() == "help" {
			return
		}
		if _, ok := forbidden[c.Name()]; ok {
			t.Fatalf("excluded command %s", c.CommandPath())
		}
		if c.Name() == "triggers" {
			if commandByName(c, "create") != nil {
				t.Fatal("triggers create is excluded")
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(wf)
}

func TestEmptyBodyMethodsDoNotAcceptInput(t *testing.T) {
	f, _, _, _ := newLazyFactory()
	cmd := NewCommand(f)
	paths := [][]string{
		{"publish"},
		{"activate"},
		{"deactivate"},
		{"versions", "revert"},
		{"triggers", "rotate-webhook-token"},
		{"executions", "cancel"},
	}
	for _, path := range paths {
		target := cmd
		for _, name := range path {
			target = commandByName(target, name)
			if target == nil {
				t.Fatalf("command %s missing", strings.Join(path, " "))
			}
		}
		if target.Flags().Lookup("input") != nil {
			t.Fatalf("%s accepts --input", strings.Join(path, " "))
		}
	}
}

func TestRequiredFlagsAppearInUse(t *testing.T) {
	f, _, _, _ := newLazyFactory()
	root := NewCommand(f)
	cases := []struct {
		path []string
		use  string
	}{
		{path: []string{"update"}, use: "update WORKFLOW_ID --input FILE"},
		{path: []string{"versions", "get"}, use: "get VERSION --workflow ID"},
		{path: []string{"versions", "revert"}, use: "revert VERSION --workflow ID"},
		{path: []string{"triggers", "list"}, use: "list --workflow ID"},
		{path: []string{"triggers", "get"}, use: "get TRIGGER_ID --workflow ID"},
		{path: []string{"triggers", "update"}, use: "update TRIGGER_ID --workflow ID --update-mask FIELDS --input FILE"},
		{path: []string{"triggers", "delete"}, use: "delete TRIGGER_ID --workflow ID"},
		{path: []string{"triggers", "rotate-webhook-token"}, use: "rotate-webhook-token TRIGGER_ID --workflow ID"},
		{path: []string{"executions", "list"}, use: "list --workflow ID"},
		{path: []string{"executions", "get"}, use: "get EXECUTION_ID --workflow ID"},
		{path: []string{"executions", "cancel"}, use: "cancel EXECUTION_ID --workflow ID"},
	}
	for _, tc := range cases {
		target := root
		for _, name := range tc.path {
			target = commandByName(target, name)
			if target == nil {
				t.Fatalf("command %s missing", strings.Join(tc.path, " "))
			}
		}
		if target.Use != tc.use {
			t.Fatalf("%s Use=%q want %q", strings.Join(tc.path, " "), target.Use, tc.use)
		}
	}
	if commandByName(root, "history").Flags().Lookup("limit") != nil || commandByName(commandByName(root, "event-types"), "list").Flags().Lookup("limit") != nil {
		t.Fatal("non-paginated command accepts --limit")
	}
}

func TestHelpStatesUserOnlyNotices(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{args: []string{"workflows", "--help"}, want: []string{"user credential", "anonymous"}},
		{args: []string{"workflows", "executions", "--help"}, want: []string{"user credential"}},
		{args: []string{"workflows", "triggers", "--help"}, want: []string{"user credential"}},
	}
	for _, tc := range cases {
		f, stdout, stderr, calls := newLazyFactory()
		err := executeRoot(f, tc.args...)
		if err != nil {
			t.Fatalf("%v: %v stderr=%q", tc.args, err, stderr.String())
		}
		if *calls != 0 {
			t.Fatalf("%v resolved factory", tc.args)
		}
		help := stdout.String()
		for _, phrase := range tc.want {
			if !strings.Contains(help, phrase) {
				t.Fatalf("%v help missing %q:\n%s", tc.args, phrase, help)
			}
		}
	}
}

func TestHelpDoesNotResolveFactory(t *testing.T) {
	f, _, stderr, calls := newLazyFactory()
	if err := executeRoot(f, "workflows", "--help"); err != nil {
		t.Fatalf("help: %v stderr=%q", err, stderr.String())
	}
	if *calls != 0 {
		t.Fatalf("help resolved factory; calls=%d", *calls)
	}
}

func newLazyFactory() (*cli.Factory, *bytes.Buffer, *bytes.Buffer, *int) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	calls := 0
	resolve := func() { calls++ }
	f := &cli.Factory{
		IO: &cli.IOStreams{In: bytes.NewReader(nil), Out: stdout, Err: stderr},
		Client: func() (*apiclient.Client, error) {
			resolve()
			return nil, cli.ErrNoCredentials
		},
		Organization: func() (string, error) {
			resolve()
			return "", &cli.UsageError{Msg: "organization required"}
		},
		Printer: func() *output.Printer {
			resolve()
			return output.New(stdout, output.Options{})
		},
		Confirm: func(string) error {
			resolve()
			return cli.ErrCancelled
		},
		NewRequestID: func() string {
			resolve()
			return "test-request-id"
		},
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
