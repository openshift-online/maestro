package server

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"

	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/pkg/logger"
)

var log = logger.GetLogger()

type Server interface {
	Start()
	Stop() error
	Listen() (net.Listener, error)
	Serve(net.Listener)
}

func removeTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

// Exit on error
func check(err error, msg string) {
	if err != nil && err != http.ErrServerClosed {
		log.Errorf("%s: %s", msg, err)
		sentry.CaptureException(err)
		sentry.Flush(environments.Environment().Config.Sentry.Timeout)
		os.Exit(1)
	}
}
