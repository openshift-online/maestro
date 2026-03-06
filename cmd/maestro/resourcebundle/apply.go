package resourcebundle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func newApplyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply -f <file>",
		Short: "Create or update a resource bundle",
		Long: `Create or update a resource bundle from a manifest file (JSON format).

This command reads a JSON manifest file and publishes it via gRPC:
- If 'id' is not specified in the manifest, a new resource bundle will be created
  with a generated UUID
- If 'id' is specified, the existing resource bundle will be updated (errors if
  the resource bundle doesn't exist)

The manifest file should contain:
- id: Resource bundle ID (optional - if not provided, a UUID will be generated)
- name: User-friendly external identifier, must be globally unique (optional - if not
  provided, defaults to the same value as 'id')
- consumer_name: Target consumer/cluster name (required)
- version: Resource version (optional - for updates, must match current version if
  specified; omit to use the latest version)
- manifests: List of Kubernetes manifests (required)
- metadata: Optional metadata
- manifest_configs: Optional manifest configurations
- delete_option: Optional delete options

Examples:
  maestro resourcebundle apply -f bundle.json
  maestro resourcebundle apply -f bundle.json --grpc-server-address localhost:8090`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := runApply(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP("file", "f", "", "Path to the manifest file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runApply(cmd *cobra.Command, _ []string) error {
	// Read and parse the manifest file
	filePath, err := cmd.Flags().GetString("file")
	if err != nil {
		return fmt.Errorf("failed to read --file flag: %w", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Parse manifest file (JSON format only)
	var bundle openapi.ResourceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("failed to parse manifest file: %w", err)
	}

	// Load client configuration
	cfg, err := clients.LoadConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	// Create rest client
	restClient, err := clients.NewRESTClient(&cfg.RESTConfig)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	// Create gRPC client
	grpcClient, err := clients.NewGRPCClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer grpcClient.Close()

	ctx := context.Background()

	// Determine action based on whether ID was provided
	var action cetypes.EventAction

	// Check if ID was provided in the manifest
	idWasProvided := bundle.Id != nil && *bundle.Id != ""
	if idWasProvided {
		// ID was provided in manifest - get existing resource and update it
		existingBundle, err := restClient.GetResourceBundle(ctx, *bundle.Id)
		if err != nil {
			return fmt.Errorf("cannot update resource bundle %q: %w", *bundle.Id, err)
		}

		// Validate version for optimistic concurrency control
		if bundle.Version == nil || *bundle.Version == 0 {
			// Use existing version if not specified in manifest
			bundle.Version = existingBundle.Version
		} else if existingBundle.Version != nil && *bundle.Version != *existingBundle.Version {
			// Version was specified but doesn't match - reject to prevent lost updates
			return fmt.Errorf("version mismatch: manifest specifies version %d but resource bundle has version %d. Update your manifest with the current version or omit 'version' to use the latest", *bundle.Version, *existingBundle.Version)
		}
		action = cetypes.UpdateRequestAction
	} else {
		// ID was generated - create new resource bundle
		resourceID := uuid.New().String()
		bundle.Id = &resourceID
		bundle.Version = openapi.PtrInt32(0)
		action = cetypes.CreateRequestAction
	}

	// Apply the resource bundle via gRPC
	if err := grpcClient.Apply(ctx, &bundle, action); err != nil {
		return fmt.Errorf("failed to apply resource bundle: %w", err)
	}

	fmt.Printf("Resource bundle applied successfully:\nID: %s\n", *bundle.Id)

	return nil
}
