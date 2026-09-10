package script

import (
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

// NewCommand is the `sl script` product root. Script has no root resource, so every
// verb lives under its resource noun.
func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "script", Short: "Manage Apps Script projects", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newProjectsCommand(f), newContentCommand(f), newVersionsCommand(f), newDeploymentsCommand(f), newMetricsCommand(f), newProcessesCommand(f), newContractsCommand(f), newLibrariesCommand(f))
	return cmd
}
