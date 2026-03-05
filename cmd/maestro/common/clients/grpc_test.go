package clients

import (
	"context"
	"strings"
	"testing"
	"time"

	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/common/clients/mock"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func TestNewGRPCClient_Insecure(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	if client.sourceID != "test-source" {
		t.Errorf("Expected sourceID=test-source, got %s", client.sourceID)
	}
}

func TestNewGRPCClient_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "both token and client cert",
			config: &Config{
				GRPCConfig: GRPCConfig{
					ServerAddress: "127.0.0.1:8080",
					SourceID:      "test",
					TokenFile:     "/tmp/token",
					ClientCert:    "/tmp/cert",
					ClientKey:     "/tmp/key",
				},
			},
			wantErr:     true,
			errContains: "cannot use both token",
		},
		{
			name: "client cert without key",
			config: &Config{
				GRPCConfig: GRPCConfig{
					ServerAddress: "127.0.0.1:8080",
					SourceID:      "test",
					ClientCert:    "/tmp/cert",
				},
			},
			wantErr:     true,
			errContains: "both client certificate and key must be provided",
		},
		{
			name: "client key without cert",
			config: &Config{
				GRPCConfig: GRPCConfig{
					ServerAddress: "127.0.0.1:8080",
					SourceID:      "test",
					ClientKey:     "/tmp/key",
				},
			},
			wantErr:     true,
			errContains: "both client certificate and key must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGRPCClient(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewGRPCClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewGRPCClient() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestGRPCClient_Apply(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name        string
		bundle      *openapi.ResourceBundle
		action      cetypes.EventAction
		wantErr     bool
		errContains string
	}{
		{
			name: "successful apply",
			bundle: &openapi.ResourceBundle{
				Id:           openapi.PtrString("test-bundle-1"),
				ConsumerName: openapi.PtrString("consumer1"),
				Version:      openapi.PtrInt32(1),
				Manifests: []map[string]interface{}{
					{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
			},
			action:  cetypes.CreateRequestAction,
			wantErr: false,
		},
		{
			name: "missing bundle ID",
			bundle: &openapi.ResourceBundle{
				ConsumerName: openapi.PtrString("consumer1"),
				Version:      openapi.PtrInt32(1),
				Manifests: []map[string]interface{}{
					{"kind": "ConfigMap"},
				},
			},
			action:      cetypes.CreateRequestAction,
			wantErr:     true,
			errContains: "resource bundle ID is required",
		},
		{
			name: "missing consumer name",
			bundle: &openapi.ResourceBundle{
				Id:      openapi.PtrString("test-bundle-1"),
				Version: openapi.PtrInt32(1),
				Manifests: []map[string]interface{}{
					{"kind": "ConfigMap"},
				},
			},
			action:      cetypes.CreateRequestAction,
			wantErr:     true,
			errContains: "consumer name is required",
		},
		{
			name: "empty manifests",
			bundle: &openapi.ResourceBundle{
				Id:           openapi.PtrString("test-bundle-1"),
				ConsumerName: openapi.PtrString("consumer1"),
				Version:      openapi.PtrInt32(1),
				Manifests:    []map[string]interface{}{},
			},
			action:      cetypes.CreateRequestAction,
			wantErr:     true,
			errContains: "must specify at least one item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcServer.ClearPublishedEvents()

			ctx := context.Background()
			err := client.Apply(ctx, tt.bundle, tt.action)

			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Apply() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr {
				// Verify event was published
				events := grpcServer.GetPublishedEvents()
				if len(events) != 1 {
					t.Errorf("Expected 1 published event, got %d", len(events))
				}
			}
		})
	}
}

func TestGRPCClient_Delete(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name            string
		resourceID      string
		consumerName    string
		resourceVersion int32
		wantErr         bool
		errContains     string
	}{
		{
			name:            "successful delete",
			resourceID:      "test-bundle-1",
			consumerName:    "consumer1",
			resourceVersion: 1,
			wantErr:         false,
		},
		{
			name:            "missing resource ID",
			resourceID:      "",
			consumerName:    "consumer1",
			resourceVersion: 1,
			wantErr:         true,
			errContains:     "resource ID is required",
		},
		{
			name:            "missing consumer name",
			resourceID:      "test-bundle-1",
			consumerName:    "",
			resourceVersion: 1,
			wantErr:         true,
			errContains:     "consumer name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcServer.ClearPublishedEvents()

			ctx := context.Background()
			err := client.Delete(ctx, tt.resourceID, tt.consumerName, tt.resourceVersion)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Delete() error = %v, should contain %v", err, tt.errContains)
				}
			}

			if !tt.wantErr {
				// Verify event was published
				events := grpcServer.GetPublishedEvents()
				if len(events) != 1 {
					t.Errorf("Expected 1 published event, got %d", len(events))
				}
			}
		})
	}
}

func TestGRPCClient_Close(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestGRPCClient_PublishWithMetadata(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}
	defer client.Close()

	// Test with metadata
	bundle := &openapi.ResourceBundle{
		Id:           openapi.PtrString("test-bundle-1"),
		ConsumerName: openapi.PtrString("consumer1"),
		Version:      openapi.PtrInt32(1),
		Manifests: []map[string]interface{}{
			{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "test-cm",
				},
			},
		},
		Metadata: map[string]interface{}{
			"custom-key": "custom-value",
		},
	}

	ctx := context.Background()
	err = client.Apply(ctx, bundle, cetypes.CreateRequestAction)
	if err != nil {
		t.Errorf("Apply() error = %v", err)
	}

	// Verify event was published
	events := grpcServer.GetPublishedEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 published event, got %d", len(events))
	}
}

func TestGRPCClient_ApplyWithTimeout(t *testing.T) {
	grpcServer, err := mock.NewGRPCServer()
	if err != nil {
		t.Fatalf("Failed to create mock gRPC server: %v", err)
	}
	defer grpcServer.Stop()

	cfg := &Config{
		GRPCConfig: GRPCConfig{
			ServerAddress: grpcServer.Address(),
			SourceID:      "test-source",
		},
	}

	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Fatalf("NewGRPCClient() failed: %v", err)
	}
	defer client.Close()

	bundle := &openapi.ResourceBundle{
		Id:           openapi.PtrString("test-bundle-1"),
		ConsumerName: openapi.PtrString("consumer1"),
		Version:      openapi.PtrInt32(1),
		Manifests: []map[string]interface{}{
			{"kind": "ConfigMap"},
		},
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Apply(ctx, bundle, cetypes.CreateRequestAction)
	if err != nil {
		t.Errorf("Apply() with timeout error = %v", err)
	}
}
