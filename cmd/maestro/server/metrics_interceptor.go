package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
)

func init() {
	// Register the metrics:
	RegisterGRPCMetrics()
}

// NewMetricsUnaryInterceptor creates a unary server interceptor for server metrics.
// Currently supports the Publish method with PublishRequest.
func newMetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// extract the type from the method name
		methodInfo := strings.Split(info.FullMethod, "/")
		if len(methodInfo) != 3 || methodInfo[2] != "Publish" {
			// only record publish metrics
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
		grpcCalledCountMetric.WithLabelValues(t, source).Inc()

		grpcMessageReceivedCountMetric.WithLabelValues(t, source).Inc()
		startTime := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(startTime).Seconds()
		grpcMessageSentCountMetric.WithLabelValues(t, source).Inc()

		// get status code from error
		status := statusFromError(err)
		code := status.Code()
		grpcProcessedCountMetric.WithLabelValues(t, source, code.String()).Inc()
		grpcProcessedDurationMetric.WithLabelValues(t, source).Observe(duration)

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
	grpcCalledCountMetric.WithLabelValues(w.t, subReq.Source).Inc()
	grpcMessageReceivedCountMetric.WithLabelValues(w.t, subReq.Source).Inc()

	return err
}

// SendMsg wraps the SendMsg method of the embedded grpc.ServerStream.
func (w *wrappedMetricsStream) SendMsg(m interface{}) error {
	err := w.ServerStream.SendMsg(m)
	grpcMessageSentCountMetric.WithLabelValues(w.t, *w.source).Inc()
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
			// only record metrics for subscribe method
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
		grpcProcessedCountMetric.WithLabelValues(t, source, code.String()).Inc()

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
	grpcMetricsTypeLabel   = "type"
	grpcMetricsSourceLabel = "source"
	grpcMetricsCodeLabel   = "code"
)

// grpcMetricsLabels - Array of labels added to metrics:
var grpcMetricsLabels = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
}

// grpcMetricsAllLabels - Array of all labels added to metrics:
var grpcMetricsAllLabels = []string{
	grpcMetricsTypeLabel,
	grpcMetricsSourceLabel,
	grpcMetricsCodeLabel,
}

// Names of the metrics:
const (
	calledCountMetric          = "called_total"
	processedCountMetric       = "processed_total"
	processedDurationMetric    = "processed_duration_seconds"
	messageReceivedCountMetric = "message_received_total"
	messageSentCountMetric     = "message_sent_total"
)

// Register the metrics:
func RegisterGRPCMetrics() {
	prometheus.MustRegister(grpcCalledCountMetric)
	prometheus.MustRegister(grpcProcessedCountMetric)
	prometheus.MustRegister(grpcProcessedDurationMetric)
	prometheus.MustRegister(grpcMessageReceivedCountMetric)
	prometheus.MustRegister(grpcMessageSentCountMetric)
}

// Unregister the metrics:
func UnregisterGRPCMetrics() {
	prometheus.Unregister(grpcCalledCountMetric)
	prometheus.Unregister(grpcProcessedCountMetric)
	prometheus.Unregister(grpcProcessedDurationMetric)
	prometheus.Unregister(grpcMessageReceivedCountMetric)
	prometheus.Unregister(grpcMessageSentCountMetric)
}

// Reset the metrics:
func ResetGRPCMetrics() {
	grpcCalledCountMetric.Reset()
	grpcProcessedCountMetric.Reset()
	grpcProcessedDurationMetric.Reset()
	grpcMessageReceivedCountMetric.Reset()
	grpcMessageSentCountMetric.Reset()
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
