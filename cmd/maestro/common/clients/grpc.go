package clients

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"k8s.io/klog/v2"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// GRPCClient handles gRPC CloudEvents communication
type GRPCClient struct {
	conn     *grpc.ClientConn
	client   pbv1.CloudEventServiceClient
	sourceID string
}

// loadCA loads and validates CA certificate, returns error if CA is not provided
func loadCA(caFile string) (*x509.CertPool, error) {
	if caFile == "" {
		return nil, fmt.Errorf("CA file is required for secure gRPC connection")
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return caCertPool, nil
}

// connect establishes gRPC connection to the server
func connect(serverAddress string, opts []grpc.DialOption, sourceID string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(serverAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	client := pbv1.NewCloudEventServiceClient(conn)
	klog.V(4).Infof("Connected to gRPC server at %s", serverAddress)

	return &GRPCClient{
		conn:     conn,
		client:   client,
		sourceID: sourceID,
	}, nil
}

// NewGRPCClient creates a new gRPC client based on configuration
// Supports three modes:
// 1. Token authentication: Requires CA + TokenFile
// 2. Mutual TLS: Requires CA + ClientCert + ClientKey
// 3. Insecure: No authentication (development only)
func NewGRPCClient(cfg *Config) (*GRPCClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	if cfg.GRPCConfig.ServerAddress == "" {
		return nil, fmt.Errorf("gRPC server address is required")
	}

	if cfg.GRPCConfig.SourceID == "" {
		return nil, fmt.Errorf("gRPC source ID is required")
	}

	var opts []grpc.DialOption

	// Validate configuration: cannot have both token and client certs
	if cfg.GRPCConfig.TokenFile != "" && (cfg.GRPCConfig.ClientCert != "" || cfg.GRPCConfig.ClientKey != "") {
		return nil, fmt.Errorf("cannot use both token authentication and client certificates")
	}

	// Case 1: Token authentication - load CA and configure token
	if cfg.GRPCConfig.TokenFile != "" {
		caCertPool, err := loadCA(cfg.GRPCConfig.CAFile)
		if err != nil {
			return nil, err
		}

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    caCertPool,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))

		tokenBytes, err := os.ReadFile(cfg.GRPCConfig.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read token file: %w", err)
		}

		token := strings.TrimSpace(string(tokenBytes))
		if token == "" {
			return nil, fmt.Errorf("token file is empty")
		}

		perRPCCred := oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: token,
			}),
		}

		opts = append(opts, grpc.WithPerRPCCredentials(perRPCCred))
		klog.V(4).Infof("Using TLS with token authentication")

		return connect(cfg.GRPCConfig.ServerAddress, opts, cfg.GRPCConfig.SourceID)
	}

	// Case 2: Mutual TLS - load CA and configure TLS cert
	if cfg.GRPCConfig.ClientCert != "" && cfg.GRPCConfig.ClientKey != "" {
		caCertPool, err := loadCA(cfg.GRPCConfig.CAFile)
		if err != nil {
			return nil, err
		}

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    caCertPool,
		}

		cert, err := tls.LoadX509KeyPair(cfg.GRPCConfig.ClientCert, cfg.GRPCConfig.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		klog.V(4).Infof("Using mutual TLS with client certificate")

		return connect(cfg.GRPCConfig.ServerAddress, opts, cfg.GRPCConfig.SourceID)
	}

	// Validate that both cert and key are provided together
	if cfg.GRPCConfig.ClientCert != "" || cfg.GRPCConfig.ClientKey != "" {
		return nil, fmt.Errorf("both client certificate and key must be provided for mutual TLS")
	}

	// Default case: no cert or token, fall back to insecure connection
	if cfg.GRPCConfig.CAFile != "" {
		return nil, fmt.Errorf("grpc-ca-file requires grpc-token-file or grpc-client-cert-file + grpc-client-key-file")
	}
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	klog.V(4).Infof("Using insecure gRPC connection (no TLS)")

	return connect(cfg.GRPCConfig.ServerAddress, opts, cfg.GRPCConfig.SourceID)
}

// publish a CloudEvent to the gRPC server
func (c *GRPCClient) publish(ctx context.Context, evt *cloudevents.Event) error {
	// Convert CloudEvent to protobuf format
	pbEvt := &pbv1.CloudEvent{}
	if err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt); err != nil {
		return fmt.Errorf("failed to convert CloudEvent to protobuf: %w", err)
	}

	// Publish the event
	_, err := c.client.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
	if err != nil {
		return fmt.Errorf("failed to publish CloudEvent: %w", err)
	}

	klog.V(4).Infof("Published CloudEvent: id=%s, type=%s, source=%s", evt.ID(), evt.Type(), evt.Source())
	return nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Apply creates or updates a resource bundle via CloudEvent
func (c *GRPCClient) Apply(ctx context.Context, bundle *openapi.ResourceBundle, action cetypes.EventAction) error {
	// Validate required fields
	if bundle == nil {
		return fmt.Errorf("resource bundle is required")
	}
	if bundle.Id == nil || *bundle.Id == "" {
		return fmt.Errorf("resource bundle ID is required")
	}
	if bundle.Version == nil {
		return fmt.Errorf("resource bundle version is required")
	}
	if bundle.ConsumerName == nil || *bundle.ConsumerName == "" {
		return fmt.Errorf("consumer name is required")
	}
	if len(bundle.Manifests) == 0 {
		return fmt.Errorf("manifest must specify at least one item in 'manifests'")
	}

	resourceID := *bundle.Id

	// Build the CloudEvent data structure
	data := map[string]interface{}{
		"manifests": bundle.Manifests,
	}

	if len(bundle.ManifestConfigs) > 0 {
		data["manifestConfigs"] = bundle.ManifestConfigs
	}

	if bundle.DeleteOption != nil {
		data["deleteOption"] = bundle.DeleteOption
	}

	switch action {
	case cetypes.CreateRequestAction, cetypes.UpdateRequestAction:
		// supported
	default:
		return fmt.Errorf("unsupported action for Apply: %s", action)
	}

	// Create CloudEvent
	evt := cloudevents.NewEvent()
	evt.SetID(uuid.New().String())
	evt.SetSource(c.sourceID)

	// Build event type based on action
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              action,
	}
	evt.SetType(eventType.String())
	evt.SetDataContentType(cloudevents.ApplicationJSON)

	// Set CloudEvent extensions
	evt.SetExtension(cetypes.ExtensionResourceID, resourceID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, *bundle.Version)
	evt.SetExtension(cetypes.ExtensionClusterName, *bundle.ConsumerName)

	// Add metadata as extension if present
	if bundle.Metadata != nil {
		metadataBytes, err := json.Marshal(bundle.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		evt.SetExtension(cetypes.ExtensionWorkMeta, string(metadataBytes))
	}

	// Set data
	if err := evt.SetData(cloudevents.ApplicationJSON, data); err != nil {
		return fmt.Errorf("failed to set CloudEvent data: %w", err)
	}

	// Publish the CloudEvent
	if err := c.publish(ctx, &evt); err != nil {
		return fmt.Errorf("failed to publish CloudEvent: %w", err)
	}

	return nil
}

// Delete deletes a resource bundle via CloudEvent
func (c *GRPCClient) Delete(ctx context.Context, resourceID, consumerName string, resourceVersion int32) error {
	if resourceID == "" {
		return fmt.Errorf("resource ID is required")
	}
	if consumerName == "" {
		return fmt.Errorf("consumer name is required")
	}

	// Create CloudEvent
	evt := cloudevents.NewEvent()
	evt.SetID(uuid.New().String())
	evt.SetSource(c.sourceID)

	// Build event type with delete action
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.DeleteRequestAction,
	}
	evt.SetType(eventType.String())
	evt.SetDataContentType(cloudevents.ApplicationJSON)

	// Set CloudEvent extensions
	evt.SetExtension(cetypes.ExtensionResourceID, resourceID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, resourceVersion)
	evt.SetExtension(cetypes.ExtensionClusterName, consumerName)

	// No data payload needed for delete

	// Publish the CloudEvent
	if err := c.publish(ctx, &evt); err != nil {
		return fmt.Errorf("failed to publish delete CloudEvent: %w", err)
	}

	return nil
}
