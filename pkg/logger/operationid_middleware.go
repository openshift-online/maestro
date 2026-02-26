package logger

import (
	"context"
	"net/http"
	"strings"

	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/klog/v2"
	sdkgologging "open-cluster-management.io/sdk-go/pkg/logging"
)

const OpIDKey = sdkgologging.ContextTracingOPIDKey
const OpIDHeader string = "X-Operation-ID"

// Middleware wraps the given HTTP handler so that the details of the request are sent to the log.
func OperationIDMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get operation ID from request header if existed
		opID := r.Header.Get(OpIDHeader)
		if opID != "" {
			// Add operationID to context (override if existed)
			ctx = context.WithValue(ctx, OpIDKey, opID)
		} else {
			// If no operationID from header, get it from context or generate a new one
			ctx = WithOpID(r.Context())
			opID, _ = ctx.Value(OpIDKey).(string)
			// Set the generated operation ID in the request header for consistency
			r.Header.Set(OpIDHeader, opID)
		}

		// Add operationID attribute to span
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(operationIDAttribute(opID))

		// Add operationID to logger so it appears in all log messages
		logger := klog.FromContext(ctx).WithValues(OpIDKey, opID)
		ctx = klog.NewContext(ctx, logger)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithOpID(ctx context.Context) context.Context {
	if ctx.Value(OpIDKey) != nil {
		return ctx
	}
	opID := ksuid.New().String()
	return context.WithValue(ctx, OpIDKey, opID)
}

// GetOperationID get operationID of the context
func GetOperationID(ctx context.Context) string {
	if opID, ok := ctx.Value(OpIDKey).(string); ok {
		return opID
	}
	return ""
}

// operationIDAttribute returns an otel attribute with operationID
func operationIDAttribute(id string) attribute.KeyValue {
	return attribute.String(strings.ToLower(string(OpIDKey)), id)
}
