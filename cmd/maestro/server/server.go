package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/environments"
)

type Server interface {
	Start(ctx context.Context)
	Stop() error
	Listen() (net.Listener, error)
	Serve(ctx context.Context, listener net.Listener)
}

func removeTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

// Exit on error
func check(ctx context.Context, err error, msg string) {
	if err != nil && err != http.ErrServerClosed {
		logger := klog.FromContext(ctx)
		logger.Error(err, msg)
		sentry.CaptureException(err)
		sentry.Flush(environments.Environment().Config.Sentry.Timeout)
		os.Exit(1)
	}
}
