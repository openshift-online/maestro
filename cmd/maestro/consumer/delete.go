package consumer

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
		Short: "Delete a consumer by ID",
		Long: `Delete a consumer by its ID.

By default, this command will prompt for confirmation before deleting.
Use the --yes flag to skip the confirmation prompt.

Note: A consumer cannot be deleted if it has existing resource bundles.

Examples:
  maestro consumer delete <consumer-id>
  maestro consumer delete <consumer-id> --yes`,
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
	consumerID := args[0]
	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to read --yes flag: %w", err)
	}

	// Confirmation prompt
	if !skipConfirm {
		fmt.Printf("Are you sure you want to delete consumer %s? (y/N): ", consumerID)
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

	// Delete the consumer
	ctx := context.Background()
	if err := restClient.DeleteConsumer(ctx, consumerID); err != nil {
		return err
	}

	fmt.Printf("Consumer %s deleted successfully\n", consumerID)
	return nil
}
