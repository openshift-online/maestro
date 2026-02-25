package consumer

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new consumer",
		Long: `Create a new consumer with the specified name and optional labels.

Labels can be specified using the --label flag (can be used multiple times).
Each label should be in the format key=value.

Examples:
  maestro consumer create prod-cluster-01
  maestro consumer create prod-cluster-01 --label env=production --label region=us-east
  maestro consumer create dev-cluster-01 --label env=dev --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runCreate(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringSlice("label", []string{}, "Labels in key=value format (can be specified multiple times)")
	output.AddFormatFlag(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Parse labels
	labelStrings, err := cmd.Flags().GetStringSlice("label")
	if err != nil {
		return fmt.Errorf("failed to read --label: %w", err)
	}

	labels := make(map[string]string)
	for _, labelStr := range labelStrings {
		parts := strings.SplitN(labelStr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid label format: %s (expected key=value)", labelStr)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return fmt.Errorf("invalid label format: %s (key cannot be empty)", labelStr)
		}
		labels[key] = parts[1]
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

	// Build consumer object
	consumer := openapi.Consumer{
		Name: &name,
	}
	if len(labels) > 0 {
		consumer.Labels = &labels
	}

	// Create the consumer
	ctx := context.Background()
	created, err := restClient.CreateConsumer(ctx, consumer)
	if err != nil {
		return err
	}

	// Output the result
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		return output.PrintConsumer(os.Stdout, created)
	}

	return output.PrintJSON(os.Stdout, created)
}
