package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"k8s.io/klog/v2"
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
		os.Exit(1)
	}
}
