package workflows

import (
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "workflows",
		Short:             "Manage workflows",
		Long:              "Starting or resuming an execution and creating a trigger require user OAuth (`sl auth login`).\nWebhook ingress POST /v1/hooks/{token} is anonymous and is not a command.",
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}
	cmd.AddCommand(newWorkflowCommands(f)...)
	cmd.AddCommand(newVersionsCommand(f), newTriggersCommand(f), newExecutionsCommand(f), newEventTypesCommand(f))
	attachUserOAuthNotices(cmd)
	return cmd
}

func attachUserOAuthNotices(cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		switch child.Name() {
		case "executions":
			appendHelpNotice(child, "Starting or resuming an execution requires user OAuth (`sl auth login`).")
		case "triggers":
			appendHelpNotice(child, "Creating a trigger requires user OAuth (`sl auth login`).")
		}
	}
}

func appendHelpNotice(cmd *cobra.Command, notice string) {
	if strings.Contains(cmd.Long, "user OAuth") || strings.Contains(cmd.Short, "user OAuth") {
		return
	}
	if cmd.Long == "" {
		cmd.Long = cmd.Short + "\n\n" + notice
		return
	}
	cmd.Long = strings.TrimSpace(cmd.Long) + "\n\n" + notice
}
