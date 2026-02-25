package resourcebundle

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resource bundles",
		Long: `List resource bundles with optional filtering and pagination.

Examples:
  maestro resourcebundle list
  maestro resourcebundle list --page 1 --size 50
  maestro resourcebundle list --search "consumer_name='prod-cluster-01'"
  maestro resourcebundle list --output json`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := runList(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Add list-specific flags
	cmd.Flags().Int("page", 1, "Page number (default: 1)")
	cmd.Flags().Int("size", 100, "Page size (default: 100)")
	cmd.Flags().String("search", "", "Search filter (e.g., \"consumer_name='cluster-01'\")")

	output.AddFormatFlag(cmd)

	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	// Get pagination flags
	page, _ := cmd.Flags().GetInt("page")
	size, _ := cmd.Flags().GetInt("size")
	search, _ := cmd.Flags().GetString("search")

	if page < 1 {
		return fmt.Errorf("--page must be >= 1")
	}
	if size < 1 {
		return fmt.Errorf("--size must be >= 1")
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

	// List resource bundles
	ctx := context.Background()
	result, err := restClient.ListResourceBundles(ctx, page, size, search)
	if err != nil {
		return err
	}

	// Output the result
	format, err := output.GetFormat(cmd)
	if err != nil {
		return err
	}

	if format == output.FormatTable {
		items := result.GetItems()
		return output.PrintResourceBundleList(os.Stdout, items)
	}

	return output.PrintJSON(os.Stdout, result)
}
