package resourcebundle

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Get the status of a resource bundle",
		Long: `Get the status field of a resource bundle by its ID.

This command retrieves only the status information, which includes conditions
and other status details about the resource bundle deployment.

Examples:
  maestro resourcebundle status 2faPrp3ZoCMkzdHnBBWd9wqwVXd
  maestro resourcebundle status 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runStatus(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	output.AddFormatFlag(cmd)

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	bundleID := args[0]

	// Load REST client configuration
	cfg, err := clients.LoadRESTConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	// Create REST client
	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	// Get the resource bundle
	ctx := context.Background()
	bundle, err := restClient.GetResourceBundle(ctx, bundleID)
	if err != nil {
		return err
	}

	// Output the status field
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		return output.PrintResourceBundleStatus(os.Stdout, bundleID, bundle.Status)
	}

	return output.PrintJSON(os.Stdout, bundle.Status)
}
