package server

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"github.com/openshift-online/maestro/pkg/constants"
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
		attrs := []attribute.KeyValue{}
		bvalues := []string{constants.ClusterServiceClusterID, constants.AROCorrelationID, constants.AROClientRequestID, constants.ARORequestID, string(logger.OpIDKey)}
		for _, k := range bvalues {
			if v := b.Member(k).Value(); v != "" {
				attrs = append(attrs, attribute.String(k, b.Member(k).Value()))
			}
		}

		if len(attrs) > 0 {
			trace.SpanFromContext(ctx).SetAttributes(attrs...)
		}
		h.ServeHTTP(w, r)
	})
}
