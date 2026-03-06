package resourcebundle

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
)

func TestRunDelete(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	tests := []struct {
		name        string
		args        []string
		skipConfirm bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful delete with --yes flag",
			args:        []string{"bundle-1"},
			skipConfirm: true,
			wantErr:     false,
		},
		{
			name:        "delete non-existent bundle",
			args:        []string{"not-found"},
			skipConfirm: true,
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server, grpcServer)
			defer cleanup()

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			clients.AddGRPCClientFlags(cmd, "test-source")
			cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			if tt.skipConfirm {
				cmd.Flags().Set("yes", "true")
			}

			err := runDelete(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("runDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("runDelete() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestRunDelete_WithConfirmation(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	// Test cancellation by providing "n" as input
	t.Run("delete cancelled by user", func(t *testing.T) {
		cleanup := setupTestEnv(t, server, grpcServer)
		defer cleanup()

		// Create a pipe to simulate user input
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		// Write "n" to simulate user cancelling
		go func() {
			w.WriteString("n\n")
			w.Close()
		}()

		defer func() {
			os.Stdin = oldStdin
		}()

		cmd := &cobra.Command{}
		clients.AddRESTClientFlags(cmd)
		clients.AddGRPCClientFlags(cmd, "test-source")
		cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")

		// Parse flags to initialize them
		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("Failed to parse flags: %v", err)
		}

		// Don't set --yes flag to trigger confirmation prompt

		err := runDelete(cmd, []string{"bundle-1"})

		// Should not error when cancelled
		if err != nil {
			t.Errorf("runDelete() should not error when cancelled, got %v", err)
		}
	})
}
