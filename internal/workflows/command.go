package workflows

import (
	"fmt"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "workflows",
		Short:             "Manage workflows",
		Long:              "Creating or signaling an execution and creating a trigger require user OAuth (`sl auth login`). The anonymous webhook endpoint is available only through the API.",
		Args:              groupArgs,
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		RunE:              func(c *cobra.Command, args []string) error { return c.Help() },
	}
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &cli.UsageError{Msg: err.Error()}
	})
	cmd.AddCommand(newWorkflowCommands(f)...)
	cmd.AddCommand(newRevisionsCommand(f), newExecutionsCommand(f), newStepRunsCommand(f), newTriggersCommand(f), newEventTypesCommand(f))
	return cmd
}

func groupArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return &cli.UsageError{Msg: fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())}
}
