package consumer

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func TestRunUpdate(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	tests := []struct {
		name         string
		args         []string
		labels       []string
		removeLabels []string
		output       string
		wantErr      bool
		errContains  string
	}{
		{
			name:    "successful update with add labels",
			args:    []string{"consumer-1"},
			labels:  []string{"env=staging", "tier=bronze"},
			output:  "table",
			wantErr: false,
		},
		{
			name:         "successful update with remove labels",
			args:         []string{"consumer-1"},
			removeLabels: []string{"old-key", "deprecated"},
			output:       "table",
			wantErr:      false,
		},
		{
			name:         "successful update with both add and remove labels",
			args:         []string{"consumer-1"},
			labels:       []string{"env=prod"},
			removeLabels: []string{"old-env"},
			output:       "json",
			wantErr:      false,
		},
		{
			name:        "update with no operations specified",
			args:        []string{"consumer-1"},
			output:      "table",
			wantErr:     true,
			errContains: "at least one --label or --remove-label must be specified",
		},
		{
			name:        "update with invalid label format",
			args:        []string{"consumer-1"},
			labels:      []string{"invalid"},
			output:      "table",
			wantErr:     true,
			errContains: "invalid label format",
		},
		{
			name:        "update non-existent consumer",
			args:        []string{"not-found"},
			labels:      []string{"env=prod"},
			output:      "table",
			wantErr:     true,
			errContains: "not found",
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
			cmd.Flags().StringSlice("remove-label", []string{}, "Labels to remove")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set(output.FlagOutput, tt.output)
			for _, label := range tt.labels {
				cmd.Flags().Set("label", label)
			}
			for _, label := range tt.removeLabels {
				cmd.Flags().Set("remove-label", label)
			}

			err := runUpdate(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("runUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("runUpdate() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}
