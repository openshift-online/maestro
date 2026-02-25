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

func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a consumer",
		Long: `Update a consumer's labels.

Labels can be added/updated using the --label flag.
Labels can be removed using the --remove-label flag.

Examples:
  maestro consumer update <consumer-id> --label tier=premium
  maestro consumer update <consumer-id> --label env=production --label tier=gold
  maestro consumer update <consumer-id> --remove-label deprecated
  maestro consumer update <consumer-id> --label tier=silver --remove-label old-tier --output json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runUpdate(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringSlice("label", []string{}, "Labels to add/update in key=value format (can be specified multiple times)")
	cmd.Flags().StringSlice("remove-label", []string{}, "Label keys to remove (can be specified multiple times)")
	output.AddFormatFlag(cmd)

	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	consumerID := args[0]

	// Parse labels to add/update
	labelStrings, _ := cmd.Flags().GetStringSlice("label")
	labelsToAdd := make(map[string]string)
	for _, labelStr := range labelStrings {
		parts := strings.SplitN(labelStr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid label format: %s (expected key=value)", labelStr)
		}
		labelsToAdd[parts[0]] = parts[1]
	}

	// Parse labels to remove
	labelsToRemove, _ := cmd.Flags().GetStringSlice("remove-label")
	for _, k := range labelsToRemove {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("invalid remove-label: key cannot be empty")
		}
	}

	// Validate that at least one operation is specified
	if len(labelsToAdd) == 0 && len(labelsToRemove) == 0 {
		return fmt.Errorf("at least one --label or --remove-label must be specified")
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

	// First, get the current consumer to read existing labels
	ctx := context.Background()
	current, err := restClient.GetConsumer(ctx, consumerID)
	if err != nil {
		return err
	}

	// Merge labels
	mergedLabels := make(map[string]string)
	if current.Labels != nil {
		for k, v := range *current.Labels {
			mergedLabels[k] = v
		}
	}

	// Add/update labels
	for k, v := range labelsToAdd {
		mergedLabels[k] = v
	}

	// Remove labels
	for _, k := range labelsToRemove {
		delete(mergedLabels, k)
	}

	// Build patch request
	patchRequest := openapi.ConsumerPatchRequest{
		Labels: &mergedLabels,
	}

	// Update the consumer
	updated, err := restClient.UpdateConsumer(ctx, consumerID, patchRequest)
	if err != nil {
		return err
	}

	// Output the result
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		return output.PrintConsumer(os.Stdout, updated)
	}

	return output.PrintJSON(os.Stdout, updated)
}
