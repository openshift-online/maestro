package resourcebundle

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a resource bundle by ID",
		Long: `Delete a resource bundle by its ID.

By default, this command will prompt for confirmation before deleting.
Use the --yes flag to skip the confirmation prompt.

Examples:
  maestro resourcebundle delete 2faPrp3ZoCMkzdHnBBWd9wqwVXd
  maestro resourcebundle delete 2faPrp3ZoCMkzdHnBBWd9wqwVXd --yes`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runDelete(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
	bundleID := args[0]
	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to read --yes flag: %w", err)
	}

	// Load client configuration
	cfg, err := clients.LoadConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Get resource bundle details first (to get consumer name and version)
	restClient, err := clients.NewRESTClient(&cfg.RESTConfig)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	bundle, err := restClient.GetResourceBundle(ctx, bundleID)
	if err != nil {
		return err
	}
	if bundle == nil || bundle.ConsumerName == nil || bundle.Version == nil {
		return fmt.Errorf("resource bundle %q is missing required metadata (consumer_name/version)", bundleID)
	}
	consumerName := *bundle.ConsumerName
	version := *bundle.Version

	// Confirmation prompt
	if !skipConfirm {
		fmt.Printf("Are you sure you want to delete resource bundle %s (consumer: %s)? (y/N): ",
			bundleID, consumerName)
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Create gRPC client for deletion
	grpcClient, err := clients.NewGRPCClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer grpcClient.Close()

	// Delete the resource bundle via gRPC
	if err := grpcClient.Delete(ctx, bundleID, consumerName, version); err != nil {
		return fmt.Errorf("failed to delete resource bundle: %w", err)
	}

	fmt.Printf("Resource bundle %s deleted successfully\n", bundleID)
	return nil
}
