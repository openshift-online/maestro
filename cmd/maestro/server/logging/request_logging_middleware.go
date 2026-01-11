package logging

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

func RegisterLoggerMiddleware(ctx context.Context, router *mux.Router) {
	router.Use(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimSuffix(r.URL.Path, "/")
				doLog := true
				logLevel := 2

				// these contribute greatly to log spam but are not useful or meaningful.
				// consider a list/map of URLs should this grow in the future.
				if path == "/api/maestro" {
					doLog = false
				}

				if path == "/healthcheck" {
					logLevel = 4
				}

				reqCtx := r.Context()
				logger := klog.FromContext(reqCtx)
				loggingWriter := NewLoggingWriter(logger, w, r, NewJSONLogFormatter())

				if doLog {
					msg, err := loggingWriter.prepareRequestLog()
					loggingWriter.log(logLevel, msg, err)
				}

				before := time.Now()
				next.ServeHTTP(loggingWriter, r)
				elapsed := time.Since(before).String()

				if doLog {
					msg, err := loggingWriter.prepareResponseLog(elapsed)
					loggingWriter.log(logLevel, msg, err)
				}
			})
		})
}
