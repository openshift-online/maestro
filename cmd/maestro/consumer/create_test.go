package consumer

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func setupTestEnv(_ *testing.T, server *mock.Server) func() {
	os.Setenv(clients.EnvRESTURL, server.URL)
	return func() {
		os.Unsetenv(clients.EnvRESTURL)
	}
}

func TestRunCreate(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	tests := []struct {
		name        string
		args        []string
		labels      []string
		output      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful create without labels",
			args:    []string{"new-consumer"},
			labels:  []string{},
			output:  "table",
			wantErr: false,
		},
		{
			name:    "successful create with labels",
			args:    []string{"new-consumer-with-labels"},
			labels:  []string{"env=prod", "tier=gold"},
			output:  "json",
			wantErr: false,
		},
		{
			name:        "create with invalid label format",
			args:        []string{"new-consumer"},
			labels:      []string{"invalid-label-format"},
			output:      "table",
			wantErr:     true,
			errContains: "invalid label format",
		},
		{
			name:        "create with conflict",
			args:        []string{"conflict"},
			labels:      []string{},
			output:      "table",
			wantErr:     true,
			errContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server)
			defer cleanup()

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			output.AddFormatFlag(cmd)
			cmd.Flags().StringSlice("label", []string{}, "Labels")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set(output.FlagOutput, tt.output)
			for _, label := range tt.labels {
				cmd.Flags().Set("label", label)
			}

			err := runCreate(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("runCreate() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestRunCreate_LabelParsing(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	tests := []struct {
		name    string
		labels  []string
		wantErr bool
	}{
		{
			name:    "single label",
			labels:  []string{"env=prod"},
			wantErr: false,
		},
		{
			name:    "multiple labels",
			labels:  []string{"env=prod", "tier=gold", "region=us-east"},
			wantErr: false,
		},
		{
			name:    "label with equals in value",
			labels:  []string{"config=key=value"},
			wantErr: false,
		},
		{
			name:    "invalid label - no equals",
			labels:  []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "valid label - empty value",
			labels:  []string{"key="},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server)
			defer cleanup()

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			output.AddFormatFlag(cmd)
			cmd.Flags().StringSlice("label", []string{}, "Labels")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set(output.FlagOutput, "json")

			for _, label := range tt.labels {
				cmd.Flags().Set("label", label)
			}

			err := runCreate(cmd, []string{"test-consumer"})

			if (err != nil) != tt.wantErr {
				t.Errorf("runCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
