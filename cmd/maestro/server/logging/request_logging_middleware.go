package logging

import (
	"net/http"
	"strings"
	"time"
)

func RequestLoggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.TrimSuffix(request.URL.Path, "/")
		doLog := true

		// these contribute greatly to log spam but are not useful or meaningful.
		// consider a list/map of URLs should this grow in the future.
		if path == "/api/maestro" {
			doLog = false
		}

		loggingWriter := NewLoggingWriter(writer, request, NewJSONLogFormatter())

		if doLog {
			loggingWriter.log(loggingWriter.prepareRequestLog())
		}

		before := time.Now()
		handler.ServeHTTP(loggingWriter, request)
		elapsed := time.Since(before).String()

		if doLog {
			loggingWriter.log(loggingWriter.prepareResponseLog(elapsed))
		}
	})
}
