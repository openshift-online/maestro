package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// Helper function to extract text from CallToolResult
func getResultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		return textContent.Text
	}
	return ""
}

func TestConsumerTools_HandleListConsumers(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}

	tools := NewConsumerTools(restClient)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "list with default parameters",
			args: map[string]any{},
		},
		{
			name: "list with pagination",
			args: map[string]any{
				"page": 1,
				"size": 50,
			},
		},
		{
			name: "list with search filter",
			args: map[string]any{
				"search": "test",
			},
		},
		{
			name: "list with all parameters",
			args: map[string]any{
				"page":   2,
				"size":   25,
				"search": "prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_list_consumers",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleListConsumers(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("HandleListConsumers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if !result.IsError {
					t.Errorf("HandleListConsumers() expected error result")
				}
				if len(result.Content) > 0 {
					if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
						if !strings.Contains(textContent.Text, tt.errContains) {
							t.Errorf("HandleListConsumers() error = %v, should contain %v", textContent.Text, tt.errContains)
						}
					}
				}
			}

			if !tt.wantErr && result == nil {
				t.Error("HandleListConsumers() returned nil result")
			}
		})
	}
}

func TestConsumerTools_HandleGetConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}

	tools := NewConsumerTools(restClient)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "get existing consumer",
			args: map[string]any{
				"id": "consumer-1",
			},
		},
		{
			name: "get non-existent consumer",
			args: map[string]any{
				"id": "not-found",
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "missing id parameter",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "id parameter is required",
		},
		{
			name: "empty id parameter",
			args: map[string]any{
				"id": "",
			},
			wantErr:     true,
			errContains: "id parameter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_get_consumer",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleGetConsumer(context.Background(), req)

			if err != nil {
				t.Errorf("HandleGetConsumer() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleGetConsumer() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleGetConsumer() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleGetConsumer() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestConsumerTools_HandleCreateConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}

	tools := NewConsumerTools(restClient)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "create consumer with name only",
			args: map[string]any{
				"name": "new-consumer",
			},
		},
		{
			name: "create consumer with labels",
			args: map[string]any{
				"name": "new-consumer-with-labels",
				"labels": map[string]any{
					"env":    "production",
					"region": "us-east-1",
				},
			},
		},
		{
			name:        "missing name parameter",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "name parameter is required",
		},
		{
			name: "empty name parameter",
			args: map[string]any{
				"name": "",
			},
			wantErr:     true,
			errContains: "name parameter is required",
		},
		{
			name: "conflict",
			args: map[string]any{
				"name": "conflict",
			},
			wantErr:     true,
			errContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_create_consumer",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleCreateConsumer(context.Background(), req)

			if err != nil {
				t.Errorf("HandleCreateConsumer() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleCreateConsumer() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleCreateConsumer() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleCreateConsumer() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestConsumerTools_HandleUpdateConsumerLabels(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}

	tools := NewConsumerTools(restClient)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "update consumer labels",
			args: map[string]any{
				"id": "consumer-1",
				"labels": map[string]any{
					"env": "production",
				},
			},
		},
		{
			name: "missing id parameter",
			args: map[string]any{
				"labels": map[string]any{
					"env": "production",
				},
			},
			wantErr:     true,
			errContains: "id parameter is required",
		},
		{
			name: "missing labels parameter",
			args: map[string]any{
				"id": "consumer-1",
			},
			wantErr:     true,
			errContains: "labels parameter is required",
		},
		{
			name: "update non-existent consumer",
			args: map[string]any{
				"id": "not-found",
				"labels": map[string]any{
					"env": "production",
				},
			},
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_update_consumer_labels",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleUpdateConsumerLabels(context.Background(), req)

			if err != nil {
				t.Errorf("HandleUpdateConsumerLabels() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleUpdateConsumerLabels() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleUpdateConsumerLabels() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleUpdateConsumerLabels() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestConsumerTools_HandleDeleteConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &clients.RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	restClient, err := clients.NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}

	tools := NewConsumerTools(restClient)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "delete existing consumer",
			args: map[string]any{
				"id": "consumer-1",
			},
		},
		{
			name: "delete non-existent consumer",
			args: map[string]any{
				"id": "not-found",
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "missing id parameter",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "id parameter is required",
		},
		{
			name: "delete with conflict",
			args: map[string]any{
				"id": "conflict",
			},
			wantErr:     true,
			errContains: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_delete_consumer",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleDeleteConsumer(context.Background(), req)

			if err != nil {
				t.Errorf("HandleDeleteConsumer() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleDeleteConsumer() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleDeleteConsumer() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleDeleteConsumer() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestGetIntParam(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "int value",
			args:       map[string]any{"page": 5},
			key:        "page",
			defaultVal: 1,
			want:       5,
		},
		{
			name:       "float64 value",
			args:       map[string]any{"page": float64(10)},
			key:        "page",
			defaultVal: 1,
			want:       10,
		},
		{
			name:       "int32 value",
			args:       map[string]any{"page": int32(15)},
			key:        "page",
			defaultVal: 1,
			want:       15,
		},
		{
			name:       "int64 value",
			args:       map[string]any{"page": int64(20)},
			key:        "page",
			defaultVal: 1,
			want:       20,
		},
		{
			name:       "missing key",
			args:       map[string]any{},
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
		{
			name:       "invalid type",
			args:       map[string]any{"page": "invalid"},
			key:        "page",
			defaultVal: 1,
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIntParam(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getIntParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStringParam(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "string value",
			args:       map[string]any{"search": "test"},
			key:        "search",
			defaultVal: "",
			want:       "test",
		},
		{
			name:       "missing key",
			args:       map[string]any{},
			key:        "search",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "invalid type",
			args:       map[string]any{"search": 123},
			key:        "search",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringParam(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getStringParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatToolResult(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name: "consumer object",
			data: openapi.Consumer{
				Id:   openapi.PtrString("consumer-1"),
				Name: openapi.PtrString("test-consumer"),
			},
		},
		{
			name: "consumer list",
			data: openapi.ConsumerList{
				Items: []openapi.Consumer{
					{
						Id:   openapi.PtrString("consumer-1"),
						Name: openapi.PtrString("test-consumer-1"),
					},
				},
				Page:  1,
				Size:  1,
				Total: 1,
			},
		},
		{
			name: "simple map",
			data: map[string]string{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatToolResult(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("formatToolResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("formatToolResult() returned nil result")
			}

			if !tt.wantErr && len(result.Content) == 0 {
				t.Error("formatToolResult() returned empty content")
			}
		})
	}
}
