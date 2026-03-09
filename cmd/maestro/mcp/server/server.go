package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/mcp/tools"
)

// MaestroMCPServer wraps the MCP server with Maestro clients and tools
type MaestroMCPServer struct {
	restClient          *clients.RESTClient
	grpcClient          *clients.GRPCClient
	mcpServer           *server.MCPServer
	resourceBundleTools *tools.ResourceBundleTools
	consumerTools       *tools.ConsumerTools
}

// NewMaestroMCPServer creates a new Maestro MCP server instance
func NewMaestroMCPServer(cfg *clients.Config) (*MaestroMCPServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Create REST client (required)
	restClient, err := clients.NewRESTClient(&cfg.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	// Create gRPC client (required)
	grpcClient, err := clients.NewGRPCClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	cleanupGRPC := true
	defer func() {
		if cleanupGRPC && grpcClient != nil {
			_ = grpcClient.Close()
		}
	}()
	klog.Info("gRPC client initialized successfully")

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"maestro-mcp-server",
		"1.0.0",
	)

	s := &MaestroMCPServer{
		restClient:          restClient,
		grpcClient:          grpcClient,
		mcpServer:           mcpServer,
		resourceBundleTools: tools.NewResourceBundleTools(restClient, grpcClient),
		consumerTools:       tools.NewConsumerTools(restClient),
	}

	// Register all tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}
	cleanupGRPC = false

	klog.Info("Maestro MCP server initialized successfully")
	return s, nil
}

// registerTools registers all Maestro tools with the MCP server
func (s *MaestroMCPServer) registerTools() error {
	// Get tool definitions
	toolDefs := tools.GetMaestroTools()

	// Map tool names to handlers (using server.ToolHandlerFunc signature)
	handlers := map[string]server.ToolHandlerFunc{
		// Resource Bundle Tools
		"maestro_list_resource_bundles":             s.resourceBundleTools.HandleListResourceBundles,
		"maestro_get_resource_bundle":               s.resourceBundleTools.HandleGetResourceBundle,
		"maestro_apply_resource_bundle":             s.resourceBundleTools.HandleApplyResourceBundle,
		"maestro_delete_resource_bundle":            s.resourceBundleTools.HandleDeleteResourceBundle,
		"maestro_get_resource_bundle_status":        s.resourceBundleTools.HandleGetResourceBundleStatus,
		"maestro_search_resource_bundles":           s.resourceBundleTools.HandleSearchResourceBundles,
		"maestro_list_resource_bundles_by_consumer": s.resourceBundleTools.HandleListResourceBundlesByConsumer,

		// Consumer Tools
		"maestro_list_consumers":         s.consumerTools.HandleListConsumers,
		"maestro_get_consumer":           s.consumerTools.HandleGetConsumer,
		"maestro_create_consumer":        s.consumerTools.HandleCreateConsumer,
		"maestro_update_consumer_labels": s.consumerTools.HandleUpdateConsumerLabels,
		"maestro_delete_consumer":        s.consumerTools.HandleDeleteConsumer,
	}

	// Register each tool
	for _, tool := range toolDefs {
		handler, ok := handlers[tool.Name]
		if !ok {
			return fmt.Errorf("no handler found for tool %s", tool.Name)
		}

		s.mcpServer.AddTool(tool, handler)
		klog.V(4).Infof("Registered tool: %s", tool.Name)
	}

	klog.Infof("Registered %d tools", len(toolDefs))
	return nil
}

// Start starts the MCP server on stdio
func (s *MaestroMCPServer) Start(ctx context.Context) error {
	klog.Info("Starting Maestro MCP server on stdio...")

	// The MCP server runs on stdio
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the MCP server
func (s *MaestroMCPServer) Stop() {
	klog.Info("Stopping Maestro MCP server...")

	// Close gRPC client if it exists
	if s.grpcClient != nil {
		if err := s.grpcClient.Close(); err != nil {
			klog.Errorf("Error closing gRPC client: %v", err)
		}
	}

	klog.Info("Maestro MCP server stopped")
}

// HandleToolCall is a generic handler that routes to specific tool handlers
// This can be used if the MCP server needs a single entry point
func (s *MaestroMCPServer) HandleToolCall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch request.Params.Name {
	// Resource Bundle Tools
	case "maestro_list_resource_bundles":
		return s.resourceBundleTools.HandleListResourceBundles(ctx, request)
	case "maestro_get_resource_bundle":
		return s.resourceBundleTools.HandleGetResourceBundle(ctx, request)
	case "maestro_apply_resource_bundle":
		return s.resourceBundleTools.HandleApplyResourceBundle(ctx, request)
	case "maestro_delete_resource_bundle":
		return s.resourceBundleTools.HandleDeleteResourceBundle(ctx, request)
	case "maestro_get_resource_bundle_status":
		return s.resourceBundleTools.HandleGetResourceBundleStatus(ctx, request)
	case "maestro_search_resource_bundles":
		return s.resourceBundleTools.HandleSearchResourceBundles(ctx, request)
	case "maestro_list_resource_bundles_by_consumer":
		return s.resourceBundleTools.HandleListResourceBundlesByConsumer(ctx, request)

	// Consumer Tools
	case "maestro_list_consumers":
		return s.consumerTools.HandleListConsumers(ctx, request)
	case "maestro_get_consumer":
		return s.consumerTools.HandleGetConsumer(ctx, request)
	case "maestro_create_consumer":
		return s.consumerTools.HandleCreateConsumer(ctx, request)
	case "maestro_update_consumer_labels":
		return s.consumerTools.HandleUpdateConsumerLabels(ctx, request)
	case "maestro_delete_consumer":
		return s.consumerTools.HandleDeleteConsumer(ctx, request)

	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", request.Params.Name)), nil
	}
}
