package resourcebundle

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func newGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a resource bundle by ID",
		Long: `Get a single resource bundle by its ID.

Example:
  maestro resourcebundle get 2faPrp3ZoCMkzdHnBBWd9wqwVXd
  maestro resourcebundle get 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runGet(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	output.AddFormatFlag(cmd)

	return cmd
}

func runGet(cmd *cobra.Command, args []string) error {
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

	// Output the result
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		return output.PrintResourceBundle(os.Stdout, bundle)
	}

	return output.PrintJSON(os.Stdout, bundle)
}
