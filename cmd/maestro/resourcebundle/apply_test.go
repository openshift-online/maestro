package resourcebundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
)

func setupTestEnv(_ *testing.T, server *mock.Server, grpcServer *mock.GRPCServer) func() {
	os.Setenv(clients.EnvRESTURL, server.URL)
	if grpcServer != nil {
		os.Setenv(clients.EnvGRPCServerAddress, grpcServer.Address())
		os.Setenv(clients.EnvGRPCSourceID, "test-source")
	}
	return func() {
		os.Unsetenv(clients.EnvRESTURL)
		os.Unsetenv(clients.EnvGRPCServerAddress)
		os.Unsetenv(clients.EnvGRPCSourceID)
	}
}

func TestRunApply(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	tests := []struct {
		name        string
		manifest    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful create without id",
			manifest: `{
				"consumer_name": "test-consumer",
				"manifests": [
					{
						"apiVersion": "v1",
						"kind": "ConfigMap",
						"metadata": {"name": "test-cm"}
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "successful update with id and version",
			manifest: `{
				"id": "bundle-1",
				"consumer_name": "test-consumer",
				"version": 1,
				"manifests": [
					{
						"apiVersion": "v1",
						"kind": "ConfigMap",
						"metadata": {"name": "test-cm"}
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "update with non-existent id",
			manifest: `{
				"id": "not-found",
				"consumer_name": "test-consumer",
				"manifests": [
					{
						"apiVersion": "v1",
						"kind": "ConfigMap",
						"metadata": {"name": "test-cm"}
					}
				]
			}`,
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "invalid json format",
			manifest:    `{invalid json}`,
			wantErr:     true,
			errContains: "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server, grpcServer)
			defer cleanup()

			// Create temporary manifest file
			tmpDir := t.TempDir()
			manifestFile := filepath.Join(tmpDir, "manifest.json")
			if err := os.WriteFile(manifestFile, []byte(tt.manifest), 0644); err != nil {
				t.Fatalf("Failed to create manifest file: %v", err)
			}

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			clients.AddGRPCClientFlags(cmd, "test-source")
			cmd.Flags().StringP("file", "f", "", "Path to the manifest file")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set("file", manifestFile)

			err := runApply(cmd, []string{})

			if (err != nil) != tt.wantErr {
				t.Errorf("runApply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("runApply() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestRunApply_FileNotFound(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cleanup := setupTestEnv(t, server, grpcServer)
	defer cleanup()

	cmd := &cobra.Command{}
	clients.AddRESTClientFlags(cmd)
	clients.AddGRPCClientFlags(cmd, "test-source")
	cmd.Flags().StringP("file", "f", "", "Path to the manifest file")

	// Parse flags to initialize them
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	cmd.Flags().Set("file", "/nonexistent/file.json")

	err = runApply(cmd, []string{})

	if err == nil {
		t.Error("runApply() should error for non-existent file")
		return
	}

	if !strings.Contains(err.Error(), "failed to read manifest file") {
		t.Errorf("runApply() error = %v, should contain 'failed to read manifest file'", err)
	}
}
