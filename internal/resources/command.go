package resources

import (
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

// NewCommand is the `sl resources` product root.
func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "resources", Short: "Manage organizations and namespaces", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newOrganizationsCommand(f), newNamespacesCommand(f))
	return cmd
}
