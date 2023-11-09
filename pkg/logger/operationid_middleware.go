package logger

import (
	"context"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/segmentio/ksuid"
)

type OperationIDKey string

const OpIDKey OperationIDKey = "opID"
const OpIDHeader OperationIDKey = "X-Operation-ID"

// Middleware wraps the given HTTP handler so that the details of the request are sent to the log.
func OperationIDMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithOpID(r.Context())

		opID, ok := ctx.Value(OpIDKey).(string)
		if ok && len(opID) > 0 {
			w.Header().Set(string(OpIDHeader), opID)
		}

		// Add operation ID to sentry context
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("operation_id", opID)
			})
		}

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
