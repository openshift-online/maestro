package common

import (
	"context"
	"os"
	"time"

	errors "github.com/zgalor/weberr"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/constants"
)

// Without a specific configuration, a noop tracer is used by default.
// At least two environment variables must be configured to enable trace export:
//   - name: OTEL_EXPORTER_OTLP_ENDPOINT
//     value: http(s)://<service>.<namespace>:4318
//   - name: OTEL_TRACES_EXPORTER
//     value: otlp
func InstallOpenTelemetryTracer(ctx context.Context, logger klog.Logger) (func(context.Context) error, error) {
	logger.Info("initializing OpenTelemetry tracer")

	exp, err := autoexport.NewSpanExporter(ctx, autoexport.WithFallbackSpanExporter(newNoopFactory))
	if err != nil {
		return nil, errors.Errorf("failed to create OTEL exporter: %s", err)
	}

	resources, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(constants.DefaultSourceID),
		),
		resource.WithHost(),
	)
	if err != nil {
		return nil, errors.Errorf("failed to initialize trace resources: %s", err)
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resources),
	)
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}

	propagator := propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})
	otel.SetTextMapPropagator(propagator)

	otel.SetErrorHandler(otelErrorHandlerFunc(func(err error) {
		logger.Error(err, "OpenTelemetry.ErrorHandler")
	}))

	return shutdown, nil
}

// TracingEnabled returns true if the environment variable OTEL_TRACES_EXPORTER
// to configure the OpenTelemetry Exporter is defined.
func TracingEnabled() bool {
	_, ok := os.LookupEnv("OTEL_TRACES_EXPORTER")
	return ok
}

type otelErrorHandlerFunc func(error)

// Handle implements otel.ErrorHandler
func (f otelErrorHandlerFunc) Handle(err error) {
	f(err)
}

func newNoopFactory(_ context.Context) (tracesdk.SpanExporter, error) {
	return &noopSpanExporter{}, nil
}

var _ tracesdk.SpanExporter = noopSpanExporter{}

// noopSpanExporter is an implementation of trace.SpanExporter that performs no operations.
type noopSpanExporter struct{}

// ExportSpans is part of trace.SpanExporter interface.
func (e noopSpanExporter) ExportSpans(ctx context.Context, spans []tracesdk.ReadOnlySpan) error {
	return nil
}

// Shutdown is part of trace.SpanExporter interface.
func (e noopSpanExporter) Shutdown(ctx context.Context) error {
	return nil
}
