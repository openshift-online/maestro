package server

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/logger"
)

// PulseServer is a server that periodically updates the heartbeat for the current Maestro instance,
// checks all Maestro instances' liveness and trigger statusresync for dead instances.
type PulseServer struct {
	instanceID    string
	pulseInterval int64
	checkInterval int64
	instanceDao   dao.InstanceDao
	lockFactory   db.LockFactory
	sourceClient  cloudevents.SourceClient
}

func NewPulseServer() *PulseServer {
	sessionFactory := env().Database.SessionFactory
	return &PulseServer{
		instanceID:    env().Config.MessageBroker.ClientID,
		pulseInterval: env().Config.PulseServer.PulseInterval,
		checkInterval: env().Config.PulseServer.CheckInterval,
		instanceDao:   dao.NewInstanceDao(&sessionFactory),
		lockFactory:   db.NewAdvisoryLockFactory(sessionFactory),
		sourceClient:  env().Clients.CloudEventsSource,
	}
}

// Start initializes and runs the pulse server.
// It periodically updates the heartbeat for the current Maestro instance,
// checks Maestro instances' liveness and trigger statusresync for dead instances.
func (s *PulseServer) Start(ctx context.Context) {
	logger := logger.NewOCMLogger(ctx)
	instance := &api.Instance{
		Meta: api.Meta{
			ID:        s.instanceID,
			UpdatedAt: time.Now(),
		},
	}
	instance, err := s.instanceDao.UpSert(ctx, instance)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to create maestro instance: %s", err.Error()))
		return
	}

	pulseTicker := time.NewTicker(time.Duration(s.pulseInterval) * time.Second)
	checkTicker := time.NewTicker(time.Duration(s.checkInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			pulseTicker.Stop()
			checkTicker.Stop()
			return
		case <-pulseTicker.C:
			logger.Infof("Updating heartbeat for maestro instance: %s", instance.ID)
			instance.UpdatedAt = time.Now()
			instance, err = s.instanceDao.Replace(ctx, instance)
			if err != nil {
				// ignore the error and continue to tolerate the intermittent db connection issue
				logger.Error(fmt.Sprintf("Unable to update heartbeat for maestro instance: %s", err.Error()))
				continue
			}
		case <-checkTicker.C:
			logger.Infof("Checking maestro instances liveness and trigger statusresync for dead instances")
			// Instances not pulsing within the last three check intervals are considered as dead.
			instances, err := s.instanceDao.All(ctx)
			if err != nil {
				// ignore the error and continue to tolerate the intermittent db connection issue
				logger.Error(fmt.Sprintf("Unable to get all maestro instances: %s", err.Error()))
				continue
			}
			for _, instance := range instances {
				// Instances not pulsing within the last three check intervals are considered as dead.
				if !instance.UpdatedAt.After(time.Now().Add(time.Duration(-3*s.checkInterval) * time.Second)) {
					// lock the Instance with a fail-fast advisory lock context.
					// this allows concurrent processing of many instances by one or more maestro instances exclusively.
					lockOwnerID, acquired, err := s.lockFactory.NewNonBlockingLock(ctx, instance.ID, db.Instances)
					// Ensure that the transaction related to this lock always end.
					defer s.lockFactory.Unlock(ctx, lockOwnerID)
					if err != nil {
						logger.Error(fmt.Sprintf("Unable to obtain the event lock: %s", err.Error()))
						continue
					}
					// skip if the lock is not acquired
					if !acquired {
						logger.Infof("Instance %s is processed by another maestro instance, skip...", instance.ID)
						continue
					}

					// trigger statusresync for dead instances
					if err := s.sourceClient.Resync(ctx); err != nil {
						logger.Error(fmt.Sprintf("Unable to trigger statusresync for maestro instance (%s): %s", instance.ID, err.Error()))
						continue
					}
					// delete dead instance to avoid statusresync triggered again
					if err := s.instanceDao.Delete(ctx, instance.ID); err != nil {
						logger.Error(fmt.Sprintf("Unable to delete maestro instance (%s): %s", instance.ID, err.Error()))
						continue
					}
				}
			}
		}
	}
}
