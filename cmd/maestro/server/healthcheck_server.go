package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift-online/maestro/pkg/dao"
	"k8s.io/klog/v2"
)

var _ Server = &healthCheckServer{}

type healthCheckServer struct {
	httpServer  *http.Server
	instanceDao dao.InstanceDao
	instanceID  string
	brokerType  string
}

func NewHealthCheckServer() *healthCheckServer {
	router := mux.NewRouter()
	srv := &http.Server{
		Handler: router,
		Addr:    env().Config.HTTPServer.Hostname + ":" + env().Config.HealthCheck.BindPort,
	}

	sessionFactory := env().Database.SessionFactory
	server := &healthCheckServer{
		httpServer:  srv,
		instanceDao: dao.NewInstanceDao(&sessionFactory),
		instanceID:  env().Config.MessageBroker.ClientID,
		brokerType:  env().Config.MessageBroker.MessageBrokerType,
	}

	router.HandleFunc("/healthcheck", server.healthCheckHandler).Methods(http.MethodGet)

	return server
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

// healthCheckHandler returns a 200 OK if the instance is ready, 503 Service Unavailable otherwise.
func (s healthCheckServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// For MQTT, check if the instance is ready
	if s.brokerType == "mqtt" {
		instance, err := s.instanceDao.Get(r.Context(), s.instanceID)
		if err != nil {
			klog.Errorf("Error getting instance: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"status": "error"}`))
			if err != nil {
				klog.Errorf("Error writing healthcheck response: %v", err)
			}
			return
		}
		if instance.Ready {
			klog.Infof("Instance is ready")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"status": "ok"}`))
			if err != nil {
				klog.Errorf("Error writing healthcheck response: %v", err)
			}
			return
		}

		klog.Infof("Instance not ready")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err = w.Write([]byte(`{"status": "not ready"}`))
		if err != nil {
			klog.Errorf("Error writing healthcheck response: %v", err)
		}
		return
	}

	// For gRPC broker, return 200 OK for now
	klog.Infof("Instance is ready")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"status": "ok"}`))
	if err != nil {
		klog.Errorf("Error writing healthcheck response: %v", err)
	}
}
