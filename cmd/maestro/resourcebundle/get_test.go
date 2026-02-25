package resourcebundle

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func TestRunGet(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	tests := []struct {
		name        string
		args        []string
		output      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful get with table format",
			args:    []string{"bundle-1"},
			output:  "table",
			wantErr: false,
		},
		{
			name:    "successful get with json format",
			args:    []string{"bundle-1"},
			output:  "json",
			wantErr: false,
		},
		{
			name:        "resource bundle not found",
			args:        []string{"not-found"},
			output:      "table",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server, nil)
			defer cleanup()

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			output.AddFormatFlag(cmd)

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set(output.FlagOutput, tt.output)

			err := runGet(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("runGet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("runGet() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}
