package server

import (
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/cmd/maestro/mcp/tools"
)

func TestNewMaestroMCPServer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.Config{
		RESTConfig: clients.RESTConfig{
			BaseURL:            server.URL,
			InsecureSkipVerify: true,
			Timeout:            10 * time.Second,
		},
		GRPCConfig: clients.GRPCConfig{
			ServerAddress: "localhost:8090",
			SourceID:      "test-source-id",
		},
	}

	mcpServer, err := NewMaestroMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMaestroMCPServer() failed: %v", err)
	}

	if mcpServer == nil {
		t.Fatal("NewMaestroMCPServer() returned nil server")
	}

	if mcpServer.restClient == nil {
		t.Error("MaestroMCPServer.restClient is nil")
	}

	if mcpServer.grpcClient == nil {
		t.Error("MaestroMCPServer.grpcClient is nil")
	}

	if mcpServer.mcpServer == nil {
		t.Error("MaestroMCPServer.mcpServer is nil")
	}

	if mcpServer.resourceBundleTools == nil {
		t.Error("MaestroMCPServer.resourceBundleTools is nil")
	}

	if mcpServer.consumerTools == nil {
		t.Error("MaestroMCPServer.consumerTools is nil")
	}
}

func TestRegisterTools(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.Config{
		RESTConfig: clients.RESTConfig{
			BaseURL:            server.URL,
			InsecureSkipVerify: true,
			Timeout:            10 * time.Second,
		},
		GRPCConfig: clients.GRPCConfig{
			ServerAddress: "localhost:8090",
			SourceID:      "test-source-id",
		},
	}

	mcpServer, err := NewMaestroMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMaestroMCPServer() failed: %v", err)
	}

	// Verify all tools are registered
	toolDefs := tools.GetMaestroTools()
	expectedToolCount := len(toolDefs)

	// We can't directly check the registered tools without exposing internal state,
	// but we can verify that registration completed without error
	if expectedToolCount == 0 {
		t.Error("Expected tools to be defined, but got 0")
	}

	// Verify the server was initialized
	if mcpServer.mcpServer == nil {
		t.Fatal("MCP server not initialized after registration")
	}
}

func TestGetMaestroTools(t *testing.T) {
	toolDefs := tools.GetMaestroTools()

	expectedTools := []string{
		"maestro_list_resource_bundles",
		"maestro_get_resource_bundle",
		"maestro_apply_resource_bundle",
		"maestro_delete_resource_bundle",
		"maestro_get_resource_bundle_status",
		"maestro_search_resource_bundles",
		"maestro_list_resource_bundles_by_consumer",
		"maestro_list_consumers",
		"maestro_get_consumer",
		"maestro_create_consumer",
		"maestro_update_consumer_labels",
		"maestro_delete_consumer",
	}

	if len(toolDefs) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(toolDefs))
	}

	// Create a map of tool names for easy lookup
	toolMap := make(map[string]bool)
	for _, tool := range toolDefs {
		toolMap[tool.Name] = true
	}

	// Verify all expected tools are present
	for _, toolName := range expectedTools {
		if !toolMap[toolName] {
			t.Errorf("Expected tool %s not found in tool definitions", toolName)
		}
	}

	// Verify tool schemas are properly defined
	for _, tool := range toolDefs {
		if tool.Name == "" {
			t.Error("Tool has empty name")
		}

		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}

		if tool.InputSchema.Type != "object" {
			t.Errorf("Tool %s has invalid input schema type: %s", tool.Name, tool.InputSchema.Type)
		}

		if tool.InputSchema.Properties == nil {
			t.Errorf("Tool %s has nil properties", tool.Name)
		}
	}
}

func TestToolInputSchemas(t *testing.T) {
	toolDefs := tools.GetMaestroTools()

	tests := []struct {
		toolName       string
		requiredParams []string
		optionalParams []string
	}{
		{
			toolName:       "maestro_get_resource_bundle",
			requiredParams: []string{"id"},
		},
		{
			toolName:       "maestro_delete_resource_bundle",
			requiredParams: []string{"id"},
		},
		{
			toolName:       "maestro_get_resource_bundle_status",
			requiredParams: []string{"id"},
		},
		{
			toolName:       "maestro_search_resource_bundles",
			requiredParams: []string{"search"},
			optionalParams: []string{"page", "size"},
		},
		{
			toolName:       "maestro_list_resource_bundles_by_consumer",
			requiredParams: []string{"consumer_name"},
			optionalParams: []string{"page", "size"},
		},
		{
			toolName:       "maestro_get_consumer",
			requiredParams: []string{"id"},
		},
		{
			toolName:       "maestro_create_consumer",
			requiredParams: []string{"name"},
			optionalParams: []string{"labels"},
		},
		{
			toolName:       "maestro_update_consumer_labels",
			requiredParams: []string{"id", "labels"},
		},
		{
			toolName:       "maestro_delete_consumer",
			requiredParams: []string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			// Find the tool
			var tool *mcp.Tool
			for i := range toolDefs {
				if toolDefs[i].Name == tt.toolName {
					tool = &toolDefs[i]
					break
				}
			}

			if tool == nil {
				t.Fatalf("Tool %s not found", tt.toolName)
			}

			// Verify required parameters
			if len(tt.requiredParams) > 0 {
				if len(tool.InputSchema.Required) != len(tt.requiredParams) {
					t.Errorf("Tool %s: expected %d required params, got %d",
						tt.toolName, len(tt.requiredParams), len(tool.InputSchema.Required))
				}

				for _, param := range tt.requiredParams {
					found := false
					for _, required := range tool.InputSchema.Required {
						if required == param {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Tool %s: required parameter %s not found in schema", tt.toolName, param)
					}
				}
			}

			// Verify all required parameters are defined in properties
			for _, param := range tt.requiredParams {
				if _, ok := tool.InputSchema.Properties[param]; !ok {
					t.Errorf("Tool %s: required parameter %s not defined in properties", tt.toolName, param)
				}
			}

			// Verify optional parameters are defined in properties
			for _, param := range tt.optionalParams {
				if _, ok := tool.InputSchema.Properties[param]; !ok {
					t.Errorf("Tool %s: optional parameter %s not defined in properties", tt.toolName, param)
				}
			}
		})
	}
}
