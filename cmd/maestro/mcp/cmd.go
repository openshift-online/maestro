package mcp

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/mcp/server"
)

// NewMCPCommand creates the MCP subcommand
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP (Model Context Protocol) server",
		Long: `Run the Maestro MCP server that provides AI assistant integration.

The MCP server runs on stdio and provides tools for managing Maestro resources
through AI assistants like Claude Code.

Both REST and gRPC clients are required for full functionality.

Configuration can be provided via flags or environment variables:
  REST API Configuration:
    --rest-url                    Maestro REST API base URL (env: MAESTRO_REST_URL)
    --insecure-skip-verify        Skip TLS verification (env: MAESTRO_REST_INSECURE_SKIP_VERIFY)
    --timeout                     HTTP client timeout (env: MAESTRO_REST_TIMEOUT)

  gRPC Configuration:
    --grpc-server-address         gRPC server address (env: MAESTRO_GRPC_SERVER_ADDRESS)
    --grpc-ca-file                CA certificate file for TLS (env: MAESTRO_GRPC_CA_FILE)
    --grpc-token-file             Token file for authentication (env: MAESTRO_GRPC_TOKEN_FILE)
    --grpc-client-cert-file       Client certificate for mTLS (env: MAESTRO_GRPC_CLIENT_CERT_FILE)
    --grpc-client-key-file        Client key for mTLS (env: MAESTRO_GRPC_CLIENT_KEY_FILE)
    --grpc-source-id              Source ID for gRPC client (env: MAESTRO_GRPC_SOURCE_ID)

Example:
  maestro mcp --rest-url https://maestro.example.com/api/maestro/v1 \
    --grpc-server-address maestro.example.com:8090 \
    --grpc-ca-file /path/to/ca.crt \
    --grpc-token-file /path/to/token
`,
		RunE: runMCP,
	}

	// Add standard client flags (REST + gRPC) with MCP source ID
	clients.AddClientFlags(cmd, "maestro-mcp")

	return cmd
}

func runMCP(cmd *cobra.Command, args []string) error {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration from flags with environment variable fallback
	cfg, err := clients.LoadConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	klog.Infof("Maestro REST URL: %s", cfg.RESTConfig.BaseURL)
	klog.Infof("Maestro gRPC address: %s", cfg.GRPCConfig.ServerAddress)

	// Create Maestro MCP server
	mcpServer, err := server.NewMaestroMCPServer(cfg)
	if err != nil {
		return err
	}

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			klog.Errorf("Server error: %v", err)
			errCh <- err
			cancel()
		}
	}()

	// Wait for interrupt signal or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		klog.Info("Received interrupt signal, shutting down gracefully...")
	case <-ctx.Done():
		klog.Info("Context cancelled, shutting down...")
	case err := <-errCh:
		klog.Errorf("Server error: %v", err)
	}

	// Stop the server
	mcpServer.Stop()
	klog.Info("MCP server stopped successfully")

	return nil
}
