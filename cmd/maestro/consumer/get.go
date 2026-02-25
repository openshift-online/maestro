package consumer

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
		Short: "Get a consumer by ID",
		Long: `Get a single consumer by its ID.

Example:
  maestro consumer get <consumer-id>
  maestro consumer get <consumer-id> --output json`,
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
	consumerID := args[0]

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

	// Get the consumer
	ctx := context.Background()
	consumer, err := restClient.GetConsumer(ctx, consumerID)
	if err != nil {
		return err
	}

	// Output the result
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		return output.PrintConsumer(os.Stdout, consumer)
	}

	return output.PrintJSON(os.Stdout, consumer)
}
