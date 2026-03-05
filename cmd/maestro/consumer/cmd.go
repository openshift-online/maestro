package consumer

import (
	"flag"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
)

// NewConsumerCommand creates the consumer subcommand
func NewConsumerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consumer",
		Short: "Manage consumers (clusters)",
		Long: `Manage Maestro consumers.

Consumers represent target clusters that receive resource bundles from Maestro.
This command provides full CRUD operations (create, get, list, update, delete) via the Maestro REST API.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Suppress verbose logs by default for CLI commands
			// Only suppress if user hasn't set -v flag
			userSetVerbosity := cmd.Flags().Changed("v") || (cmd.Parent() != nil && cmd.Parent().Flags().Changed("v"))
			if !userSetVerbosity {
				_ = flag.Set("logtostderr", "false")
			}
		},
	}

	// Add common client flags
	clients.AddRESTClientFlags(cmd)

	// Add subcommands
	cmd.AddCommand(
		newGetCommand(),
		newListCommand(),
		newCreateCommand(),
		newUpdateCommand(),
		newDeleteCommand(),
	)

	return cmd
}
