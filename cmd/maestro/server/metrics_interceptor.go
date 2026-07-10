package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

func init() {
	// Register the metrics:
	RegisterGRPCMetrics()
}

// extractOperation extracts the operation name from a CloudEvent.
// Returns the action from the CloudEvent type (e.g., "create", "update", "delete")
// or "unknown" if the type cannot be parsed.
func extractOperation(evt *ce.Event) string {
	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return "unknown"
	}
	// Action is an EventAction type, convert to string
	action := string(eventType.Action)
	// Extract just the action part before "_request" if present
	if strings.HasSuffix(action, "_request") {
		action = strings.TrimSuffix(action, "_request")
	}
	return action
}

// NewMetricsUnaryInterceptor creates a unary server interceptor for server metrics.
// Currently supports the Publish method with PublishRequest.
func newMetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// extract the type from the method name
		methodInfo := strings.Split(info.FullMethod, "/")
		if len(methodInfo) != 3 || methodInfo[2] != "Publish" {
			return handler(ctx, req)
		}
		t := methodInfo[2]
		pubReq, ok := req.(*pbv1.PublishRequest)
		if !ok {
			return nil, fmt.Errorf("invalid request type for Publish method")
		}
		// convert the request to cloudevent and extract the source
		evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pubReq.Event))
		if err != nil {
			return nil, fmt.Errorf("failed to convert to cloudevent: %v", err)
		}
		source := evt.Source()

		// extract operation from CloudEvent type
		operation := extractOperation(evt)

		// Update existing metrics (backwards compatible)
		grpcCalledCountMetric.WithLabelValues(t, source).Inc()
		grpcMessageReceivedCountMetric.WithLabelValues(t, source).Inc()

		// Update new operation-level metrics
		grpcCalledCountByOperationMetric.WithLabelValues(t, source, operation).Inc()
		grpcMessageReceivedCountByOperationMetric.WithLabelValues(t, source, operation).Inc()

		startTime := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(startTime).Seconds()

		// Update existing metrics
		grpcMessageSentCountMetric.WithLabelValues(t, source).Inc()

		// Update new operation-level metrics
		grpcMessageSentCountByOperationMetric.WithLabelValues(t, source, operation).Inc()

		// get status code from error
		status := statusFromError(err)
		code := status.Code()

		// Update existing metrics
		grpcProcessedCountMetric.WithLabelValues(t, source, code.String()).Inc()
		grpcProcessedDurationMetric.WithLabelValues(t, source).Observe(duration)

		// Update new operation-level metrics
		grpcProcessedCountByOperationMetric.WithLabelValues(t, source, operation, code.String()).Inc()
		grpcProcessedDurationByOperationMetric.WithLabelValues(t, source, operation).Observe(duration)

		return resp, err
	}
}

// wrappedMetricsStream wraps a grpc.ServerStream, capturing the request source
// emitting metrics for the stream interceptor.
type wrappedMetricsStream struct {
	t      string
	source *string
	grpc.ServerStream
	ctx context.Context
}

// RecvMsg wraps the RecvMsg method of the embedded grpc.ServerStream.
// It captures the source from the SubscriptionRequest and emits metrics.
func (w *wrappedMetricsStream) RecvMsg(m interface{}) error {
	err := w.ServerStream.RecvMsg(m)
	subReq, ok := m.(*pbv1.SubscriptionRequest)
	if !ok {
		return fmt.Errorf("invalid request type for Subscribe method")
	}
	*w.source = subReq.Source
	// Update existing metrics (backwards compatible)
	grpcCalledCountMetric.WithLabelValues(w.t, subReq.Source).Inc()
	grpcMessageReceivedCountMetric.WithLabelValues(w.t, subReq.Source).Inc()
	// Update new operation-level metrics
	grpcCalledCountByOperationMetric.WithLabelValues(w.t, subReq.Source, "subscribe").Inc()
	grpcMessageReceivedCountByOperationMetric.WithLabelValues(w.t, subReq.Source, "subscribe").Inc()

	return err
}

// SendMsg wraps the SendMsg method of the embedded grpc.ServerStream.
func (w *wrappedMetricsStream) SendMsg(m interface{}) error {
	err := w.ServerStream.SendMsg(m)
	// Update existing metrics (backwards compatible)
	grpcMessageSentCountMetric.WithLabelValues(w.t, *w.source).Inc()
	// Update new operation-level metrics
	grpcMessageSentCountByOperationMetric.WithLabelValues(w.t, *w.source, "subscribe").Inc()
	return err
}

// newWrappedMetricsStream creates a wrappedMetricsStream with the specified type and source reference.
func newWrappedMetricsStream(t string, source *string, ctx context.Context, ss grpc.ServerStream) grpc.ServerStream {
	return &wrappedMetricsStream{t, source, ss, ctx}
}

// newMetricsStreamInterceptor creates a stream server interceptor for server metrics.
// Currently supports the Subscribe method with SubscriptionRequest.
func newMetricsStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// extract the type from the method name
		if !info.IsServerStream || info.IsClientStream {
			return fmt.Errorf("invalid stream type for stream method: %s", info.FullMethod)
		}
		methodInfo := strings.Split(info.FullMethod, "/")
		if len(methodInfo) != 3 || methodInfo[2] != "Subscribe" {
			return handler(srv, stream)
		}
		t := methodInfo[2]
		source := ""
		// create a wrapped stream to capture the source and emit metrics
		wrappedMetricsStream := newWrappedMetricsStream(t, &source, stream.Context(), stream)
		err := handler(srv, wrappedMetricsStream)

		// get status code from error
		status := statusFromError(err)
		code := status.Code()
		// Update existing metrics (backwards compatible)
		grpcProcessedCountMetric.WithLabelValues(t, source, code.String()).Inc()
		// Update new operation-level metrics
		grpcProcessedCountByOperationMetric.WithLabelValues(t, source, "subscribe", code.String()).Inc()

		return err
	}
}

// statusFromError returns a grpc status. If the error code is neither a valid grpc status
// nor a context error, codes.Unknown will be set.
func statusFromError(err error) *status.Status {
	s, ok := status.FromError(err)
	// Mirror what the grpc server itself does, i.e. also convert context errors to status
	if !ok {
		s = status.FromContextError(err)
	}
	return s
}

// Subsystem used to define the metrics:
const grpcMetricsSubsystem = "grpc_server"

// Names of the labels added to metrics:
const (
	grpcMetricsTypeLabel      = "type"
	grpcMetricsSourceLabel    = "source"
	grpcMetricsCodeLabel      = "code"
	grpcMetricsOperationLabel = "operation"
)

// grpcMetricsLabels - Array of labels added to existing metrics:
var grpcMetricsLabels = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
}

// grpcMetricsAllLabels - Array of all labels added to existing metrics:
var grpcMetricsAllLabels = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
	grpcMetricsCodeLabel,
}

// grpcMetricsLabelsWithOperation - Array of labels for new operation-level metrics:
var grpcMetricsLabelsWithOperation = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
	grpcMetricsOperationLabel,
}

// grpcMetricsAllLabelsWithOperation - Array of all labels for new operation-level metrics:
var grpcMetricsAllLabelsWithOperation = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
	grpcMetricsOperationLabel,
	grpcMetricsCodeLabel,
}

// Names of the metrics:
const (
	calledCountMetric          = "called_total"
	processedCountMetric       = "processed_total"
	processedDurationMetric    = "processed_duration_seconds"
	messageReceivedCountMetric = "message_received_total"
	messageSentCountMetric     = "message_sent_total"
	// New operation-level metrics (non-breaking)
	calledCountByOperationMetric          = "called_total_by_operation"
	processedCountByOperationMetric       = "processed_total_by_operation"
	processedDurationByOperationMetric    = "processed_duration_seconds_by_operation"
	messageReceivedCountByOperationMetric = "message_received_total_by_operation"
	messageSentCountByOperationMetric     = "message_sent_total_by_operation"
)

// Register the metrics:
func RegisterGRPCMetrics() {
	// Register existing metrics (backwards compatible)
	prometheus.MustRegister(grpcCalledCountMetric)
	prometheus.MustRegister(grpcProcessedCountMetric)
	prometheus.MustRegister(grpcProcessedDurationMetric)
	prometheus.MustRegister(grpcMessageReceivedCountMetric)
	prometheus.MustRegister(grpcMessageSentCountMetric)
	// Register new operation-level metrics
	prometheus.MustRegister(grpcCalledCountByOperationMetric)
	prometheus.MustRegister(grpcProcessedCountByOperationMetric)
	prometheus.MustRegister(grpcProcessedDurationByOperationMetric)
	prometheus.MustRegister(grpcMessageReceivedCountByOperationMetric)
	prometheus.MustRegister(grpcMessageSentCountByOperationMetric)
}

// Unregister the metrics:
func UnregisterGRPCMetrics() {
	prometheus.Unregister(grpcCalledCountMetric)
	prometheus.Unregister(grpcProcessedCountMetric)
	prometheus.Unregister(grpcProcessedDurationMetric)
	prometheus.Unregister(grpcMessageReceivedCountMetric)
	prometheus.Unregister(grpcMessageSentCountMetric)
	prometheus.Unregister(grpcCalledCountByOperationMetric)
	prometheus.Unregister(grpcProcessedCountByOperationMetric)
	prometheus.Unregister(grpcProcessedDurationByOperationMetric)
	prometheus.Unregister(grpcMessageReceivedCountByOperationMetric)
	prometheus.Unregister(grpcMessageSentCountByOperationMetric)
}

// Reset the metrics:
func ResetGRPCMetrics() {
	grpcCalledCountMetric.Reset()
	grpcProcessedCountMetric.Reset()
	grpcProcessedDurationMetric.Reset()
	grpcMessageReceivedCountMetric.Reset()
	grpcMessageSentCountMetric.Reset()
	grpcCalledCountByOperationMetric.Reset()
	grpcProcessedCountByOperationMetric.Reset()
	grpcProcessedDurationByOperationMetric.Reset()
	grpcMessageReceivedCountByOperationMetric.Reset()
	grpcMessageSentCountByOperationMetric.Reset()
}

// Description of the gRPC called count metric:
var grpcCalledCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      calledCountMetric,
		Help:      "Total number of RPCs called on the server.",
	},
	grpcMetricsLabels,
)

// Description of the gRPC processed count metric:
var grpcProcessedCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      processedCountMetric,
		Help:      "Total number of RPCs processed on the server, regardless of success or failure.",
	},
	grpcMetricsAllLabels,
)

// Description of the gRPC processed duration metric:
var grpcProcessedDurationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      processedDurationMetric,
		Help:      "Histogram of the duration of RPCs processed on the server.",
		Buckets:   prometheus.DefBuckets,
	},
	grpcMetricsLabels,
)

// Description of the gRPC message received count metric:
var grpcMessageReceivedCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      messageReceivedCountMetric,
		Help:      "Total number of messages received on the server from agent and client.",
	},
	grpcMetricsLabels,
)

// Description of the gRPC message sent count metric:
var grpcMessageSentCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      messageSentCountMetric,
		Help:      "Total number of messages sent by the server to agent and client.",
	},
	grpcMetricsLabels,
)

// New operation-level metrics (non-breaking additions):

// Description of the gRPC called count metric by operation:
var grpcCalledCountByOperationMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      calledCountByOperationMetric,
		Help:      "Total number of RPCs called on the server, by operation type.",
	},
	grpcMetricsLabelsWithOperation,
)

// Description of the gRPC processed count metric by operation:
var grpcProcessedCountByOperationMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      processedCountByOperationMetric,
		Help:      "Total number of RPCs processed on the server by operation type, regardless of success or failure.",
	},
	grpcMetricsAllLabelsWithOperation,
)

// Description of the gRPC processed duration metric by operation:
var grpcProcessedDurationByOperationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      processedDurationByOperationMetric,
		Help:      "Histogram of the duration of RPCs processed on the server by operation type.",
		Buckets:   prometheus.DefBuckets,
	},
	grpcMetricsLabelsWithOperation,
)

// Description of the gRPC message received count metric by operation:
var grpcMessageReceivedCountByOperationMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      messageReceivedCountByOperationMetric,
		Help:      "Total number of messages received on the server from agent and client, by operation type.",
	},
	grpcMetricsLabelsWithOperation,
)

// Description of the gRPC message sent count metric by operation:
var grpcMessageSentCountByOperationMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: grpcMetricsSubsystem,
		Name:      messageSentCountByOperationMetric,
		Help:      "Total number of messages sent by the server to agent and client, by operation type.",
	},
	grpcMetricsLabelsWithOperation,
)
