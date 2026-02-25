package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAddFormatFlag(t *testing.T) {
	cmd := &cobra.Command{}
	AddFormatFlag(cmd)

	flag := cmd.Flags().Lookup(FlagOutput)
	if flag == nil {
		t.Fatal("AddFormatFlag() did not add output flag")
	}

	if flag.DefValue != "table" {
		t.Errorf("AddFormatFlag() default value = %v, want table", flag.DefValue)
	}

	if flag.Shorthand != "o" {
		t.Errorf("AddFormatFlag() shorthand = %v, want o", flag.Shorthand)
	}
}

func TestGetFormat(t *testing.T) {
	tests := []struct {
		name        string
		flagValue   string
		want        Format
		wantErr     bool
		errContains string
	}{
		{
			name:      "json format",
			flagValue: "json",
			want:      FormatJSON,
			wantErr:   false,
		},
		{
			name:      "table format",
			flagValue: "table",
			want:      FormatTable,
			wantErr:   false,
		},
		{
			name:        "invalid format",
			flagValue:   "yaml",
			wantErr:     true,
			errContains: "invalid output format",
		},
		{
			name:        "empty format",
			flagValue:   "",
			wantErr:     true,
			errContains: "invalid output format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			AddFormatFlag(cmd)
			cmd.Flags().Set(FlagOutput, tt.flagValue)

			got, err := GetFormat(cmd)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetFormat() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		wantJSON string
	}{
		{
			name: "simple map",
			data: map[string]interface{}{
				"id":   "test-id",
				"name": "test-name",
			},
			wantJSON: `{
  "id": "test-id",
  "name": "test-name"
}`,
		},
		{
			name: "nested structure",
			data: map[string]interface{}{
				"id": "123",
				"metadata": map[string]interface{}{
					"key": "value",
				},
			},
			wantJSON: `{
  "id": "123",
  "metadata": {
    "key": "value"
  }
}`,
		},
		{
			name: "array",
			data: []string{"item1", "item2"},
			wantJSON: `[
  "item1",
  "item2"
]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := PrintJSON(&buf, tt.data)

			if err != nil {
				t.Errorf("PrintJSON() error = %v", err)
				return
			}

			got := strings.TrimSpace(buf.String())
			want := strings.TrimSpace(tt.wantJSON)

			if got != want {
				t.Errorf("PrintJSON() output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestFormatConstants(t *testing.T) {
	// Verify format constants have expected values
	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %v, want json", FormatJSON)
	}

	if FormatTable != "table" {
		t.Errorf("FormatTable = %v, want table", FormatTable)
	}

	if FlagOutput != "output" {
		t.Errorf("FlagOutput = %v, want output", FlagOutput)
	}
}
