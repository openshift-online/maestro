package resourcebundle

import (
	"strconv"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/cmd/maestro/common/output"
)

func TestRunList(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	tests := []struct {
		name    string
		output  string
		page    int
		size    int
		search  string
		wantErr bool
	}{
		{
			name:    "successful list with table format",
			output:  "table",
			page:    1,
			size:    10,
			wantErr: false,
		},
		{
			name:    "successful list with json format",
			output:  "json",
			page:    1,
			size:    50,
			wantErr: false,
		},
		{
			name:    "list with search filter",
			output:  "table",
			page:    1,
			size:    10,
			search:  "consumer_name='test-consumer'",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, server, nil)
			defer cleanup()

			cmd := &cobra.Command{}
			clients.AddRESTClientFlags(cmd)
			output.AddFormatFlag(cmd)
			cmd.Flags().Int("page", 1, "Page number")
			cmd.Flags().Int("size", 100, "Page size")
			cmd.Flags().String("search", "", "Search filter")

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			cmd.Flags().Set(output.FlagOutput, tt.output)
			cmd.Flags().Set("page", strconv.Itoa(tt.page))
			cmd.Flags().Set("size", strconv.Itoa(tt.size))
			if tt.search != "" {
				cmd.Flags().Set("search", tt.search)
			}

			err := runList(cmd, []string{})

			if (err != nil) != tt.wantErr {
				t.Errorf("runList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
