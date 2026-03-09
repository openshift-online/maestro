package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients"
	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
)

func TestResourceBundleTools_HandleListResourceBundles(t *testing.T) {
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

	tools := NewResourceBundleTools(restClient, nil)

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
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
				"search": "consumer_name = 'test'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_list_resource_bundles",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleListResourceBundles(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("HandleListResourceBundles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("HandleListResourceBundles() returned nil result")
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleListResourceBundles() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestResourceBundleTools_HandleGetResourceBundle(t *testing.T) {
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

	tools := NewResourceBundleTools(restClient, nil)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "get existing resource bundle",
			args: map[string]any{
				"id": "bundle-1",
			},
		},
		{
			name: "get non-existent resource bundle",
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
					Name:      "maestro_get_resource_bundle",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleGetResourceBundle(context.Background(), req)

			if err != nil {
				t.Errorf("HandleGetResourceBundle() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleGetResourceBundle() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleGetResourceBundle() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleGetResourceBundle() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestResourceBundleTools_HandleGetResourceBundleStatus(t *testing.T) {
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

	tools := NewResourceBundleTools(restClient, nil)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "get status for existing bundle",
			args: map[string]any{
				"id": "bundle-1",
			},
		},
		{
			name: "get status for non-existent bundle",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_get_resource_bundle_status",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleGetResourceBundleStatus(context.Background(), req)

			if err != nil {
				t.Errorf("HandleGetResourceBundleStatus() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleGetResourceBundleStatus() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleGetResourceBundleStatus() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleGetResourceBundleStatus() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestResourceBundleTools_HandleSearchResourceBundles(t *testing.T) {
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

	tools := NewResourceBundleTools(restClient, nil)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "search with query",
			args: map[string]any{
				"search": "consumer_name = 'test'",
			},
		},
		{
			name: "search with pagination",
			args: map[string]any{
				"search": "version > 1",
				"page":   1,
				"size":   50,
			},
		},
		{
			name:        "missing search parameter",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "search parameter is required",
		},
		{
			name: "empty search parameter",
			args: map[string]any{
				"search": "",
			},
			wantErr:     true,
			errContains: "search parameter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_search_resource_bundles",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleSearchResourceBundles(context.Background(), req)

			if err != nil {
				t.Errorf("HandleSearchResourceBundles() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleSearchResourceBundles() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleSearchResourceBundles() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleSearchResourceBundles() unexpected error result: %v", getResultText(result))
			}
		})
	}
}

func TestResourceBundleTools_HandleListResourceBundlesByConsumer(t *testing.T) {
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

	tools := NewResourceBundleTools(restClient, nil)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "list bundles by consumer",
			args: map[string]any{
				"consumer_name": "test-consumer",
			},
		},
		{
			name: "list bundles with pagination",
			args: map[string]any{
				"consumer_name": "test-consumer",
				"page":          1,
				"size":          50,
			},
		},
		{
			name:        "missing consumer_name parameter",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "consumer_name parameter is required",
		},
		{
			name: "empty consumer_name parameter",
			args: map[string]any{
				"consumer_name": "",
			},
			wantErr:     true,
			errContains: "consumer_name parameter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "maestro_list_resource_bundles_by_consumer",
					Arguments: tt.args,
				},
			}

			result, err := tools.HandleListResourceBundlesByConsumer(context.Background(), req)

			if err != nil {
				t.Errorf("HandleListResourceBundlesByConsumer() unexpected error = %v", err)
				return
			}

			if tt.wantErr {
				if !result.IsError {
					t.Errorf("HandleListResourceBundlesByConsumer() expected error result")
				}
				resultText := getResultText(result)
				if tt.errContains != "" && !strings.Contains(resultText, tt.errContains) {
					t.Errorf("HandleListResourceBundlesByConsumer() error = %v, should contain %v", resultText, tt.errContains)
				}
			}

			if !tt.wantErr && result.IsError {
				t.Errorf("HandleListResourceBundlesByConsumer() unexpected error result: %v", getResultText(result))
			}
		})
	}
}
