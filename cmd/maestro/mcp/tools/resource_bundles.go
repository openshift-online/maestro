package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// ResourceBundleTools provides handlers for resource bundle operations
type ResourceBundleTools struct {
	restClient *clients.RESTClient
	grpcClient *clients.GRPCClient
}

// NewResourceBundleTools creates a new resource bundle tools handler
func NewResourceBundleTools(restClient *clients.RESTClient, grpcClient *clients.GRPCClient) *ResourceBundleTools {
	return &ResourceBundleTools{
		restClient: restClient,
		grpcClient: grpcClient,
	}
}

// HandleListResourceBundles lists resource bundles with pagination
func (t *ResourceBundleTools) HandleListResourceBundles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	page := getIntParam(args, "page", 1)
	size := getIntParam(args, "size", 100)
	search := getStringParam(args, "search", "")

	result, err := t.restClient.ListResourceBundles(ctx, page, size, search)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleGetResourceBundle gets a single resource bundle by ID
func (t *ResourceBundleTools) HandleGetResourceBundle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	result, err := t.restClient.GetResourceBundle(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleDeleteResourceBundle deletes a resource bundle by ID
func (t *ResourceBundleTools) HandleDeleteResourceBundle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	// Get resource bundle details first (to get consumer name and version)
	bundle, err := t.restClient.GetResourceBundle(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get resource bundle: %v", err)), nil
	}

	// Validate required fields are present
	if bundle.ConsumerName == nil {
		return mcp.NewToolResultError("resource bundle has no consumer name"), nil
	}
	if bundle.Version == nil {
		return mcp.NewToolResultError("resource bundle has no version"), nil
	}

	// Delete via gRPC
	if err := t.grpcClient.Delete(ctx, id, *bundle.ConsumerName, *bundle.Version); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete resource bundle: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted resource bundle %s", id)), nil
}

// HandleGetResourceBundleStatus gets the status field of a resource bundle
func (t *ResourceBundleTools) HandleGetResourceBundleStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	result, err := t.restClient.GetResourceBundle(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Extract status field
	status := result.GetStatus()
	if status == nil {
		return mcp.NewToolResultText(fmt.Sprintf("No status available for resource bundle %s", id)), nil
	}

	return formatToolResult(status)
}

// HandleSearchResourceBundles searches resource bundles using SQL query
func (t *ResourceBundleTools) HandleSearchResourceBundles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	search, ok := args["search"].(string)
	if !ok || search == "" {
		return mcp.NewToolResultError("search parameter is required"), nil
	}

	page := getIntParam(args, "page", 1)
	size := getIntParam(args, "size", 100)

	result, err := t.restClient.ListResourceBundles(ctx, page, size, search)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleListResourceBundlesByConsumer lists resource bundles for a specific consumer
func (t *ResourceBundleTools) HandleListResourceBundlesByConsumer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	consumerName, ok := args["consumer_name"].(string)
	if !ok || consumerName == "" {
		return mcp.NewToolResultError("consumer_name parameter is required"), nil
	}

	page := getIntParam(args, "page", 1)
	size := getIntParam(args, "size", 100)

	escapedName := strings.ReplaceAll(consumerName, "'", "''")
	search := fmt.Sprintf("consumer_name = '%s'", escapedName)

	result, err := t.restClient.ListResourceBundles(ctx, page, size, search)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// Helper functions

func getIntParam(args map[string]any, key string, defaultVal int) int {
	val, ok := args[key]
	if !ok {
		return defaultVal
	}

	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	default:
		return defaultVal
	}
}

func getStringParam(args map[string]any, key string, defaultVal string) string {
	val, ok := args[key].(string)
	if !ok {
		return defaultVal
	}
	return val
}

func formatToolResult(data any) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// HandleApplyResourceBundle creates or updates a resource bundle
func (t *ResourceBundleTools) HandleApplyResourceBundle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// Parse the bundle from arguments
	bundleData, ok := args["bundle"]
	if !ok {
		return mcp.NewToolResultError("bundle parameter is required"), nil
	}

	// Marshal and unmarshal to convert to openapi.ResourceBundle
	bundleJSON, err := json.Marshal(bundleData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal bundle: %v", err)), nil
	}

	var bundle openapi.ResourceBundle
	if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse bundle: %v", err)), nil
	}

	// Determine action based on whether ID was provided
	var action cetypes.EventAction
	idWasProvided := bundle.Id != nil && *bundle.Id != ""

	if idWasProvided {
		// ID was provided - verify it exists and get version
		resourceID := *bundle.Id
		existingBundle, err := t.restClient.GetResourceBundle(ctx, resourceID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get resource bundle '%s': %v", resourceID, err)), nil
		}

		// Validate version for optimistic concurrency control
		if bundle.Version == nil || *bundle.Version == 0 {
			bundle.Version = existingBundle.Version
		} else if existingBundle.Version != nil && *bundle.Version != *existingBundle.Version {
			return mcp.NewToolResultError(fmt.Sprintf("version mismatch: bundle specifies version %d but resource bundle has version %d", *bundle.Version, *existingBundle.Version)), nil
		}

		action = cetypes.UpdateRequestAction
	} else {
		// ID not provided - generate one and create
		resourceID := uuid.New().String()
		bundle.Id = &resourceID
		bundle.Version = openapi.PtrInt32(0)
		action = cetypes.CreateRequestAction
	}

	// Apply the resource bundle via gRPC
	if err := t.grpcClient.Apply(ctx, &bundle, action); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to apply resource bundle: %v", err)), nil
	}

	actionVerb := "created"
	if action == cetypes.UpdateRequestAction {
		actionVerb = "updated"
	}

	return mcp.NewToolResultText(fmt.Sprintf("Resource bundle '%s' %s successfully", *bundle.Id, actionVerb)), nil
}
