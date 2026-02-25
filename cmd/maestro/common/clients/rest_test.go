package clients

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func TestNewRESTClient(t *testing.T) {
	cfg := &RESTConfig{
		BaseURL:            "https://example.com",
		InsecureSkipVerify: true,
		Timeout:            30 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewRESTClient() returned nil client")
	}

	if client.client == nil {
		t.Fatal("RESTClient.client is nil")
	}
}

func TestListResourceBundles(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name    string
		page    int
		size    int
		search  string
		wantErr bool
	}{
		{
			name:    "list all resource bundles",
			page:    1,
			size:    10,
			search:  "",
			wantErr: false,
		},
		{
			name:    "list with search filter",
			page:    1,
			size:    10,
			search:  "test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.ListResourceBundles(ctx, tt.page, tt.size, tt.search)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListResourceBundles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("ListResourceBundles() returned nil result")
			}
		})
	}
}

func TestGetResourceBundle(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "get existing resource bundle",
			id:      "bundle-1",
			wantErr: false,
		},
		{
			name:        "get non-existent resource bundle",
			id:          "not-found",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "unauthorized request",
			id:          "unauthorized",
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "forbidden request",
			id:          "forbidden",
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.GetResourceBundle(ctx, tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetResourceBundle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetResourceBundle() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr && result == nil {
				t.Error("GetResourceBundle() returned nil result")
			}
		})
	}
}

func TestDeleteResourceBundle(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "delete existing resource bundle",
			id:      "bundle-1",
			wantErr: false,
		},
		{
			name:        "delete non-existent resource bundle",
			id:          "not-found",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "unauthorized request",
			id:          "unauthorized",
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "forbidden request",
			id:          "forbidden",
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := client.DeleteResourceBundle(ctx, tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteResourceBundle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DeleteResourceBundle() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestListConsumers(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name    string
		page    int
		size    int
		search  string
		wantErr bool
	}{
		{
			name:    "list all consumers",
			page:    1,
			size:    10,
			search:  "",
			wantErr: false,
		},
		{
			name:    "list with search filter",
			page:    1,
			size:    10,
			search:  "test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.ListConsumers(ctx, tt.page, tt.size, tt.search)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListConsumers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("ListConsumers() returned nil result")
			}
		})
	}
}

func TestGetConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "get existing consumer",
			id:      "consumer-1",
			wantErr: false,
		},
		{
			name:        "get non-existent consumer",
			id:          "not-found",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "unauthorized request",
			id:          "unauthorized",
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "forbidden request",
			id:          "forbidden",
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.GetConsumer(ctx, tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetConsumer() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr && result == nil {
				t.Error("GetConsumer() returned nil result")
			}
		})
	}
}

func TestCreateConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		consumer    openapi.Consumer
		wantErr     bool
		errContains string
	}{
		{
			name: "create new consumer",
			consumer: openapi.Consumer{
				Name: openapi.PtrString("new-consumer"),
			},
			wantErr: false,
		},
		{
			name: "create with conflict",
			consumer: openapi.Consumer{
				Name: openapi.PtrString("conflict"),
			},
			wantErr:     true,
			errContains: "already exists",
		},
		{
			name: "create with bad request",
			consumer: openapi.Consumer{
				Name: openapi.PtrString("bad-request"),
			},
			wantErr:     true,
			errContains: "bad request",
		},
		{
			name: "unauthorized request",
			consumer: openapi.Consumer{
				Name: openapi.PtrString("unauthorized"),
			},
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name: "forbidden request",
			consumer: openapi.Consumer{
				Name: openapi.PtrString("forbidden"),
			},
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.CreateConsumer(ctx, tt.consumer)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("CreateConsumer() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr && result == nil {
				t.Error("CreateConsumer() returned nil result")
			}
		})
	}
}

func TestUpdateConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		patch       openapi.ConsumerPatchRequest
		wantErr     bool
		errContains string
	}{
		{
			name: "update existing consumer",
			id:   "consumer-1",
			patch: openapi.ConsumerPatchRequest{
				Labels: &map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
		{
			name: "update non-existent consumer",
			id:   "not-found",
			patch: openapi.ConsumerPatchRequest{
				Labels: &map[string]string{"env": "prod"},
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "update with bad request",
			id:   "bad-request",
			patch: openapi.ConsumerPatchRequest{
				Labels: &map[string]string{"env": "prod"},
			},
			wantErr:     true,
			errContains: "bad request",
		},
		{
			name: "unauthorized request",
			id:   "unauthorized",
			patch: openapi.ConsumerPatchRequest{
				Labels: &map[string]string{"env": "prod"},
			},
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name: "forbidden request",
			id:   "forbidden",
			patch: openapi.ConsumerPatchRequest{
				Labels: &map[string]string{"env": "prod"},
			},
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := client.UpdateConsumer(ctx, tt.id, tt.patch)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("UpdateConsumer() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr && result == nil {
				t.Error("UpdateConsumer() returned nil result")
			}
		})
	}
}

func TestDeleteConsumer(t *testing.T) {
	server := mock.NewMaestroServer()
	defer server.Close()

	cfg := &RESTConfig{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            10 * time.Second,
	}

	client, err := NewRESTClient(cfg)
	if err != nil {
		t.Fatalf("NewRESTClient() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "delete existing consumer",
			id:      "consumer-1",
			wantErr: false,
		},
		{
			name:        "delete non-existent consumer",
			id:          "not-found",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "delete with conflict",
			id:          "conflict",
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name:        "unauthorized request",
			id:          "unauthorized",
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "forbidden request",
			id:          "forbidden",
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := client.DeleteConsumer(ctx, tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DeleteConsumer() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}
