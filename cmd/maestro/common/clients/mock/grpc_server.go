package mock

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
)

// GRPCServer is a mock gRPC CloudEvent server for testing
type GRPCServer struct {
	pbv1.UnimplementedCloudEventServiceServer
	server          *grpc.Server
	listener        net.Listener
	publishedEvents []*pbv1.CloudEvent
	mu              sync.RWMutex
	shouldFail      bool
	failureCode     codes.Code
}

// NewGRPCServer creates a new mock gRPC server
func NewGRPCServer() (*GRPCServer, error) {
	// Create a listener on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	grpcServer := grpc.NewServer()
	mockServer := &GRPCServer{
		server:          grpcServer,
		listener:        listener,
		publishedEvents: make([]*pbv1.CloudEvent, 0),
	}

	pbv1.RegisterCloudEventServiceServer(grpcServer, mockServer)

	// Start the server in background
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	return mockServer, nil
}

// Address returns the server address
func (s *GRPCServer) Address() string {
	return s.listener.Addr().String()
}

// Stop stops the mock server
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
	s.listener.Close()
}

// Publish implements the CloudEventService Publish RPC
func (s *GRPCServer) Publish(ctx context.Context, req *pbv1.PublishRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shouldFail {
		return nil, status.Errorf(s.failureCode, "mock publish failure")
	}

	s.publishedEvents = append(s.publishedEvents, req.Event)
	return &emptypb.Empty{}, nil
}

// Subscribe implements the CloudEventService Subscribe RPC
func (s *GRPCServer) Subscribe(req *pbv1.SubscriptionRequest, stream pbv1.CloudEventService_SubscribeServer) error {
	// For testing, we don't need to implement actual subscription
	// Just keep the stream open until cancelled
	<-stream.Context().Done()
	return nil
}

// GetPublishedEvents returns all published events
func (s *GRPCServer) GetPublishedEvents() []*pbv1.CloudEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]*pbv1.CloudEvent, len(s.publishedEvents))
	copy(events, s.publishedEvents)
	return events
}

// ClearPublishedEvents clears all published events
func (s *GRPCServer) ClearPublishedEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publishedEvents = make([]*pbv1.CloudEvent, 0)
}

// SetShouldFail configures the server to fail with the given code
func (s *GRPCServer) SetShouldFail(shouldFail bool, code codes.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shouldFail = shouldFail
	s.failureCode = code
}
