package logging

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	sdkgologging "open-cluster-management.io/sdk-go/pkg/logging"

	maestrologger "github.com/openshift-online/maestro/pkg/logger"
)

func RegisterLoggerMiddleware(ctx context.Context, router *mux.Router) {
	router.Use(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimSuffix(r.URL.Path, "/")
				doLog := true

				// these contribute greatly to log spam but are not useful or meaningful.
				// consider a list/map of URLs should this grow in the future.
				if path == "/api/maestro" {
					doLog = false
				}

				// TODO set opid of logger from req
				// Get operation ID from request header if existed
				opID := r.Header.Get(string(maestrologger.OpIDHeader))
				logger := klog.FromContext(ctx).WithValues(sdkgologging.ContextTracingOPIDKey, opID)
				loggingWriter := NewLoggingWriter(logger, w, r, NewJSONLogFormatter())

				reqCtx := r.Context()
				newReqCtx := klog.NewContext(reqCtx, logger)

				if doLog {
					loggingWriter.log(loggingWriter.prepareRequestLog())
				}

				before := time.Now()
				next.ServeHTTP(loggingWriter, r.WithContext(newReqCtx))
				elapsed := time.Since(before).String()

				if doLog {
					loggingWriter.log(loggingWriter.prepareResponseLog(elapsed))
				}
			})
		})
}
