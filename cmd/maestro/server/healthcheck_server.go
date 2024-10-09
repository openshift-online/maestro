package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	health "github.com/docker/go-healthcheck"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

var (
	updater = health.NewStatusUpdater()
)

var _ Server = &healthCheckServer{}

type healthCheckServer struct {
	httpServer *http.Server
}

func NewHealthCheckServer() *healthCheckServer {
	router := mux.NewRouter()
	health.DefaultRegistry = health.NewRegistry()
	health.Register("maintenance_status", updater)
	router.HandleFunc("/healthcheck", health.StatusHandler).Methods(http.MethodGet)
	router.HandleFunc("/healthcheck/down", downHandler).Methods(http.MethodPost)
	router.HandleFunc("/healthcheck/up", upHandler).Methods(http.MethodPost)

	srv := &http.Server{
		Handler: router,
		Addr:    env().Config.HTTPServer.Hostname + ":" + env().Config.HealthCheck.BindPort,
	}

	return &healthCheckServer{
		httpServer: srv,
	}
}

func (s healthCheckServer) Start() {
	var err error
	if env().Config.HealthCheck.EnableHTTPS {
		if env().Config.HTTPServer.HTTPSCertFile == "" || env().Config.HTTPServer.HTTPSKeyFile == "" {
			check(
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
			)
		}

		// Serve with TLS
		klog.Infof("Serving HealthCheck with TLS at %s", env().Config.HealthCheck.BindPort)
		err = s.httpServer.ListenAndServeTLS(env().Config.HTTPServer.HTTPSCertFile, env().Config.HTTPServer.HTTPSKeyFile)
	} else {
		klog.Infof("Serving HealthCheck without TLS at %s", env().Config.HealthCheck.BindPort)
		err = s.httpServer.ListenAndServe()
	}
	check(err, "HealthCheck server terminated with errors")
	klog.Infof("HealthCheck server terminated")
}

func (s healthCheckServer) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}

// Unimplemented
func (s healthCheckServer) Listen() (listener net.Listener, err error) {
	return nil, nil
}

// Unimplemented
func (s healthCheckServer) Serve(listener net.Listener) {
}

func upHandler(w http.ResponseWriter, r *http.Request) {
	updater.Update(nil)
}

func downHandler(w http.ResponseWriter, r *http.Request) {
	updater.Update(fmt.Errorf("maintenance mode"))
}
