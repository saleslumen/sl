package marketplace

import (
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func NewCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "marketplace",
		Short:         "Manage Marketplace apps and installations",
		Long:          "Catalog list accepts the active profile credential. Every other command requires user OAuth (`sl auth login`).",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAppCommands(f)...)
	cmd.AddCommand(newAppOperationsCommand(f))
	cmd.AddCommand(newPublisherAppsCommand(f))
	cmd.AddCommand(newListingsCommand(f))
	cmd.AddCommand(newListingRevisionsCommand(f))
	cmd.AddCommand(newOAuthClientsCommand(f))
	cmd.AddCommand(newReleasesCommand(f))
	cmd.AddCommand(newSubmissionsCommand(f))
	cmd.AddCommand(newReviewQueueCommand(f))
	cmd.AddCommand(newInstallationsCommand(f))
	cmd.AddCommand(newInstallationOperationsCommand(f))
	cmd.AddCommand(newInstallationConfigurationCommand(f))
	cmd.AddCommand(newConnectionsCommand(f))
	cmd.AddCommand(newAPIKeyOperationsCommand(f))
	cmd.AddCommand(newOrganizationPolicyCommand(f))
	cmd.AddCommand(newInstallCapabilitiesCommand(f))
	return cmd
}
