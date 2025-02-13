package server

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"github.com/openshift-online/maestro/pkg/logger"
)

// traceAttributeMiddleware is currently only relevant for the correlation of
// requests by the ARO-HCP resource provider frontend.
//
// The middleware extracts correlation data transferred in the baggage and sets
// it as an attribute in the currently active span.
// This middleware has no effect if tracing is deactivated or if there is no
// data in the transferred baggage.
func traceAttributeMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logger.WithOpID(r.Context())
		b := baggage.FromContext(ctx)
		const (
			// TODO: May need to change the names of these IDs if maestro is used in ROSA
			correlationID   = "azure.correlation.id"
			requestID       = "azure.request.id"
			clientRequestID = "azure.client.request.id"
			operationID     = "operation.id"
		)
		attrs := []attribute.KeyValue{}
		bvalues := []string{correlationID, requestID, clientRequestID, operationID}
		for _, k := range bvalues {
			if v := b.Member(k).Value(); v != "" {
				attrs = append(attrs, attribute.String(k, b.Member(k).Value()))
			}
		}
		if v := b.Member(operationID).Value(); v == "" {
			attrs = append(attrs, attribute.String(operationID, logger.GetOperationID(ctx)))
		}
		if len(attrs) > 0 {
			trace.SpanFromContext(ctx).SetAttributes(attrs...)
		}
		h.ServeHTTP(w, r)
	})
}
