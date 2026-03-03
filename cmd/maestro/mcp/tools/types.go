package tools

import "github.com/mark3labs/mcp-go/mcp"

// GetMaestroTools returns all MCP tool definitions for Maestro
func GetMaestroTools() []mcp.Tool {
	return []mcp.Tool{
		// Resource Bundle Tools
		{
			Name:        "maestro_list_resource_bundles",
			Description: "List resource bundles with pagination and filtering. Returns bundles with their manifests, status, and metadata.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default: 1)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Page size for pagination (default: 100)",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "SQL search query (e.g., \"consumer_name = 'cluster1' AND version > 5\")",
					},
				},
			},
		},
		{
			Name:        "maestro_get_resource_bundle",
			Description: "Get a single resource bundle by ID. Returns full bundle details including manifests, status, and metadata.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource bundle ID",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "maestro_apply_resource_bundle",
			Description: "Create or update a resource bundle. If the bundle ID is provided and exists, it will be updated; otherwise it will fail. The bundle must include consumer_name and manifests.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"bundle": map[string]interface{}{
						"type":        "object",
						"description": "Resource bundle object with id, consumer_name, version, manifests, and optional metadata",
					},
				},
				Required: []string{"bundle"},
			},
		},
		{
			Name:        "maestro_delete_resource_bundle",
			Description: "Delete a resource bundle by ID. This will remove the bundle and trigger deletion on the target cluster.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource bundle ID to delete",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "maestro_get_resource_bundle_status",
			Description: "Get the status field of a resource bundle. Returns the feedback from the agent about the bundle's deployment state.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource bundle ID",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "maestro_search_resource_bundles",
			Description: "Search resource bundles using SQL-like query syntax. Useful for finding bundles by consumer, name, or other criteria.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"search": map[string]interface{}{
						"type":        "string",
						"description": "SQL search query (e.g., \"consumer_name LIKE 'prod-%'\")",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default: 1)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Page size for pagination (default: 100)",
					},
				},
				Required: []string{"search"},
			},
		},
		{
			Name:        "maestro_list_resource_bundles_by_consumer",
			Description: "List all resource bundles for a specific consumer (cluster). Shortcut for searching by consumer_name.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"consumer_name": map[string]interface{}{
						"type":        "string",
						"description": "Consumer (cluster) name",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default: 1)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Page size for pagination (default: 100)",
					},
				},
				Required: []string{"consumer_name"},
			},
		},

		// Consumer Tools
		{
			Name:        "maestro_list_consumers",
			Description: "List consumers (clusters) with pagination and filtering. Returns consumer metadata and labels.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default: 1)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Page size for pagination (default: 100)",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "SQL search query (e.g., \"labels->>'env' = 'production'\")",
					},
				},
			},
		},
		{
			Name:        "maestro_get_consumer",
			Description: "Get a single consumer (cluster) by ID. Returns consumer details including labels and metadata.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Consumer ID",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "maestro_create_consumer",
			Description: "Create a new consumer (cluster) registration. Consumer name must be RFC 1123 compliant (lowercase alphanumeric, hyphens, max 63 chars).",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Consumer name (RFC 1123 compliant)",
					},
					"labels": map[string]interface{}{
						"type":        "object",
						"description": "Optional labels as key-value pairs",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "maestro_update_consumer_labels",
			Description: "Update labels for an existing consumer. This performs a PATCH operation to merge new labels.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Consumer ID",
					},
					"labels": map[string]interface{}{
						"type":        "object",
						"description": "Labels to update (will be merged with existing labels)",
					},
				},
				Required: []string{"id", "labels"},
			},
		},
		{
			Name:        "maestro_delete_consumer",
			Description: "Delete a consumer (cluster) by ID. This will fail if the consumer has existing resource bundles.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Consumer ID to delete",
					},
				},
				Required: []string{"id"},
			},
		},
	}
}
