package server

import (
	"context"
	e "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/cmd/maestro/server/logging"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
)

type HealthCheckServer struct {
	httpServer        *http.Server
	lockFactory       db.LockFactory
	instanceDao       dao.InstanceDao
	instanceID        string
	heartbeatInterval int
	brokerType        string
}

func NewHealthCheckServer(ctx context.Context) *HealthCheckServer {
	router := mux.NewRouter()
	srv := &http.Server{
		Handler: router,
		Addr:    env().Config.HTTPServer.Hostname + ":" + env().Config.HealthCheck.BindPort,
	}

	logging.RegisterLoggerMiddleware(ctx, router)
	sessionFactory := env().Database.SessionFactory
	server := &HealthCheckServer{
		httpServer:        srv,
		lockFactory:       db.NewAdvisoryLockFactory(sessionFactory),
		instanceDao:       dao.NewInstanceDao(&sessionFactory),
		instanceID:        env().Config.MessageBroker.ClientID,
		heartbeatInterval: env().Config.HealthCheck.HeartbeartInterval,
		brokerType:        env().Config.MessageBroker.MessageBrokerType,
	}

	router.HandleFunc("/healthcheck", server.healthCheckHandler).Methods(http.MethodGet)

	return server
}

func (s *HealthCheckServer) Start(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("instanceID", s.instanceID)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting HealthCheck server")

	// start a goroutine to periodically update heartbeat for the current maestro instance
	go wait.UntilWithContext(ctx, s.pulse, time.Duration(s.heartbeatInterval*int(time.Second)))

	// start a goroutine to periodically check the liveness of maestro instances
	go wait.UntilWithContext(ctx, s.checkInstances, time.Duration(s.heartbeatInterval/3*int(time.Second)))

	var err error
	if env().Config.HealthCheck.EnableHTTPS {
		if env().Config.HTTPServer.HTTPSCertFile == "" || env().Config.HTTPServer.HTTPSKeyFile == "" {
			check(ctx,
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
			)
		}

		// Serve with TLS
		logger.Info("Serving HealthCheck with TLS", "port", env().Config.HealthCheck.BindPort)
		err = s.httpServer.ListenAndServeTLS(env().Config.HTTPServer.HTTPSCertFile, env().Config.HTTPServer.HTTPSKeyFile)
	} else {
		logger.Info("Serving HealthCheck without TLS", "port", env().Config.HealthCheck.BindPort)
		err = s.httpServer.ListenAndServe()
	}
	check(ctx, err, "HealthCheck server terminated with errors")
	logger.Info("HealthCheck server terminated")

	// wait until context is done
	<-ctx.Done()

	logger.Info("Shutting down HealthCheck server")
	s.httpServer.Shutdown(context.Background())
}

func (s *HealthCheckServer) pulse(ctx context.Context) {
	logger := klog.FromContext(ctx)
	// If there are multiple requests at the same time, it will cause the race conditions among these
	// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, s.instanceID, db.Instances)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		logger.Error(err, "Error obtaining the instance lock")
		return
	}
	found, err := s.instanceDao.Get(ctx, s.instanceID)
	if err != nil {
		if e.Is(err, gorm.ErrRecordNotFound) {
			// create a new instance if not found
			logger.V(4).Info("Creating new maestro instance")
			instance := &api.ServerInstance{
				Meta: api.Meta{
					ID: s.instanceID,
				},
				LastHeartbeat: time.Now(),
			}
			_, err := s.instanceDao.Create(ctx, instance)
			if err != nil {
				logger.Error(err, "Unable to create maestro instance")
			}
			return
		}
		logger.Error(err, "Unable to get maestro instance")
		return
	}
	found.LastHeartbeat = time.Now()
	_, err = s.instanceDao.Replace(ctx, found)
	if err != nil {
		logger.Error(err, "Unable to update heartbeat for maestro instance")
	}
}

func (s *HealthCheckServer) checkInstances(ctx context.Context) {
	// lock the Instance with a fail-fast advisory lock context.
	// this allows concurrent processing of many instances by one or more maestro instances exclusively.
	lockOwnerID, acquired, err := s.lockFactory.NewNonBlockingLock(ctx, "maestro-instances-liveness-check", db.Instances)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	logger := klog.FromContext(ctx)
	if err != nil {
		logger.Error(err, "Error obtaining the instance lock")
		return
	}
	// skip if the lock is not acquired
	if !acquired {
		logger.V(4).Info("failed to acquire the lock as another maestro instance is checking instances, skip")
		return
	}

	instances, err := s.instanceDao.All(ctx)
	if err != nil {
		logger.Error(err, "Unable to get all maestro instances")
		return
	}

	activeInstanceIDs := []string{}
	inactiveInstanceIDs := []string{}
	for _, instance := range instances {
		// Instances pulsing within the last three check intervals are considered as active.
		if instance.LastHeartbeat.After(time.Now().Add(time.Duration(int(-3*time.Second)*s.heartbeatInterval))) && !instance.Ready {
			activeInstanceIDs = append(activeInstanceIDs, instance.ID)
		} else if instance.LastHeartbeat.Before(time.Now().Add(time.Duration(int(-3*time.Second)*s.heartbeatInterval))) && instance.Ready {
			inactiveInstanceIDs = append(inactiveInstanceIDs, instance.ID)
		}
	}

	if len(activeInstanceIDs) > 0 {
		// batch mark active instances.
		if err := s.instanceDao.MarkReadyByIDs(ctx, activeInstanceIDs); err != nil {
			logger.Error(err, "Unable to mark active maestro instances", "activeInstanceIDs", activeInstanceIDs)
		}
	}

	if len(inactiveInstanceIDs) > 0 {
		// batch mark inactive instances.
		if err := s.instanceDao.MarkUnreadyByIDs(ctx, inactiveInstanceIDs); err != nil {
			logger.Error(err, "Unable to mark inactive maestro instances", "inactiveInstanceIDs", inactiveInstanceIDs)
		}
	}
}

// healthCheckHandler returns a 200 OK if the instance is ready, 503 Service Unavailable otherwise.
func (s *HealthCheckServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("instanceID", s.instanceID)
	instance, err := s.instanceDao.Get(r.Context(), s.instanceID)
	if err != nil {
		logger.Error(err, "Error getting instance")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"status": "error"}`))
		if err != nil {
			logger.Error(err, "Error writing healthcheck response")
		}
		return
	}
	if instance.Ready {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		if err != nil {
			logger.Error(err, "Error writing healthcheck response")
		}
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	_, err = w.Write([]byte(`{"status": "not ready"}`))
	if err != nil {
		logger.Error(err, "Error writing healthcheck response")
	}
}
