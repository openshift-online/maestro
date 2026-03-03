package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// ConsumerTools provides handlers for consumer operations
type ConsumerTools struct {
	restClient *clients.RESTClient
}

// NewConsumerTools creates a new consumer tools handler
func NewConsumerTools(restClient *clients.RESTClient) *ConsumerTools {
	return &ConsumerTools{
		restClient: restClient,
	}
}

// HandleListConsumers lists consumers with pagination
func (t *ConsumerTools) HandleListConsumers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	page := getIntParam(args, "page", 1)
	size := getIntParam(args, "size", 100)
	search := getStringParam(args, "search", "")

	result, err := t.restClient.ListConsumers(ctx, page, size, search)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleGetConsumer gets a single consumer by ID
func (t *ConsumerTools) HandleGetConsumer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	result, err := t.restClient.GetConsumer(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleCreateConsumer creates a new consumer
func (t *ConsumerTools) HandleCreateConsumer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// Create consumer object
	consumer := openapi.Consumer{}
	consumer.SetName(name)

	// Add labels if provided
	if labels, ok := args["labels"].(map[string]any); ok {
		labelMap := make(map[string]string)
		for k, v := range labels {
			if strVal, ok := v.(string); ok {
				labelMap[k] = strVal
			}
		}
		consumer.SetLabels(labelMap)
	}

	result, err := t.restClient.CreateConsumer(ctx, consumer)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleUpdateConsumerLabels updates labels for an existing consumer
func (t *ConsumerTools) HandleUpdateConsumerLabels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	labels, ok := args["labels"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("labels parameter is required"), nil
	}

	// Convert labels to map[string]string
	labelMap := make(map[string]string)
	for k, v := range labels {
		if strVal, ok := v.(string); ok {
			labelMap[k] = strVal
		}
	}

	// Create patch request
	patch := openapi.ConsumerPatchRequest{}
	patch.SetLabels(labelMap)

	result, err := t.restClient.UpdateConsumer(ctx, id, patch)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return formatToolResult(result)
}

// HandleDeleteConsumer deletes a consumer by ID
func (t *ConsumerTools) HandleDeleteConsumer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	err := t.restClient.DeleteConsumer(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted consumer %s", id)), nil
}
