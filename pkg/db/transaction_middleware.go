package db

import (
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/errors"
	loggertracing "github.com/openshift-online/maestro/pkg/logger"
)

// TransactionMiddleware creates a new HTTP middleware that begins a database transaction
// and stores it in the request context.
func TransactionMiddleware(next http.Handler, connection SessionFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new Context with the transaction stored in it.
		ctx, err := NewContext(r.Context(), connection)
		logger := klog.FromContext(ctx)
		if err != nil {
			logger.Error(err, "Could not create transaction")
			// use default error to avoid exposing internals to users
			err := errors.GeneralError("")
			operationID := loggertracing.GetOperationID(ctx)
			writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(operationID))
			return
		}

		// Set the value of the request pointer to the value of a new copy of the request with the new context key,vale
		// stored in it
		*r = *r.WithContext(ctx)

		// Returned from handlers and resolve transactions.
		defer func() { Resolve(r.Context()) }()

		// Continue handling requests.
		next.ServeHTTP(w, r)
	})
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
