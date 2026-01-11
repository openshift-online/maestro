package logger

import (
	"context"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/klog/v2"
	sdkgologging "open-cluster-management.io/sdk-go/pkg/logging"
)

const OpIDKeyHeader string = "X-Operation-ID-Key"
const OpIDHeader string = "X-Operation-ID"

// Middleware wraps the given HTTP handler so that the details of the request are sent to the log.
func OperationIDMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		contextOPIDKey := sdkgologging.ContextTracingOPIDKey
		// Get operation ID Key from request header if existed
		opIDKey := r.Header.Get(OpIDKeyHeader)
		// Restrict to a safe format ([a-zA-Z0-9_.-]{1,64}) and fall back to the default key if invalid.
		if opIDKey != "" && isSafeOpIDKey(opIDKey) {
			contextOPIDKey = sdkgologging.ContextTracingKey(opIDKey)
		}

		// Get operation ID from request header if existed
		opID := r.Header.Get(OpIDHeader)
		if opID != "" {
			// Add operationID to context (override if existed)
			ctx = context.WithValue(ctx, contextOPIDKey, opID)
		} else {
			// If no operationID from header, get it from context or generate a new one
			ctx = withOpID(r.Context(), contextOPIDKey)
			opID, _ := ctx.Value(contextOPIDKey).(string)
			// Set the generated operation ID in the request header for consistency
			r.Header.Set(OpIDHeader, opID)
		}

		// Add operationID attribute to span
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(operationIDAttribute(strings.ToLower(string(contextOPIDKey)), opID))

		// Add operationID to sentry context
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("operation_id", opID)
			})
		}

		// Add operationID to logger so it appears in all log messages
		logger := klog.FromContext(ctx).WithValues(contextOPIDKey, opID)
		ctx = klog.NewContext(ctx, logger)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withOpID(ctx context.Context, opIDKey sdkgologging.ContextTracingKey) context.Context {
	if v := ctx.Value(opIDKey); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return ctx
		}
	}

	opID := ksuid.New().String()
	return context.WithValue(ctx, opIDKey, opID)
}

// operationIDAttribute returns an otel attribute with operationID
func operationIDAttribute(key, id string) attribute.KeyValue {
	return attribute.String(key, id)
}

func isSafeOpIDKey(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}
