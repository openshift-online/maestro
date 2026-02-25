package resourcebundle

import (
	"flag"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
)

// NewResourceBundleCommand creates the resourcebundle subcommand
func NewResourceBundleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resourcebundle",
		Short: "Manage resource bundles",
		Long: `Manage Maestro resource bundles.

Resource bundles are collections of Kubernetes manifests that are deployed to consumer clusters.

Commands:
  apply  - Create or update a resource bundle via gRPC
  get    - Get a resource bundle by ID via REST API
  list   - List resource bundles via REST API
  delete - Delete a resource bundle via gRPC
  status - Get resource bundle status via REST API`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Suppress verbose logs by default for CLI commands
			// Only suppress if user hasn't set -v flag
			userSetVerbosity := cmd.Flags().Changed("v") || (cmd.Parent() != nil && cmd.Parent().Flags().Changed("v"))
			if !userSetVerbosity {
				_ = flag.Set("logtostderr", "false")
			}
		},
	}

	// Add common client flags with CLI source ID
	clients.AddClientFlags(cmd, "maestro-cli")

	// Add subcommands
	cmd.AddCommand(
		newApplyCommand(),
		newGetCommand(),
		newListCommand(),
		newDeleteCommand(),
		newStatusCommand(),
	)

	return cmd
}
