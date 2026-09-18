package campaigns

import (
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "campaigns", Short: "Manage campaigns", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	for _, child := range newCampaignCommands(f) {
		cmd.AddCommand(child)
	}
	cmd.AddCommand(newSchedulesCommand(f), newPeopleCommand(f), newSequencesCommand(f), newDeliveriesCommand(f), newMetricsCommand(f), newSuppressionsCommand(f), newOperationsCommand(f), newTasksCommand(f))
	return cmd
}
