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
	instance := &api.ServerInstance{
		Meta: api.Meta{
			ID:        s.instanceID,
			UpdatedAt: time.Now(),
		},
	}
	_, err := s.instanceDao.UpSert(ctx, instance)
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
			logger.V(4).Infof("Updating heartbeat for maestro instance: %s", instance.ID)
			instance.UpdatedAt = time.Now()
			_, err = s.instanceDao.UpSert(ctx, instance)
			if err != nil {
				// log and ignore the error and continue to tolerate the intermittent issue
				logger.Error(fmt.Sprintf("Unable to update heartbeat for maestro instance: %s", err.Error()))
			}
		case <-checkTicker.C:
			logger.V(4).Infof("Checking maestro instances liveness and trigger statusresync for dead instances")
			if err := s.CheckInstances(ctx); err != nil {
				// log and ignore the error and continue to tolerate the intermittent issue
				logger.Error(fmt.Sprintf("Unable to check maestro instances: %s", err.Error()))
			}
		}
	}
}

func (s *PulseServer) CheckInstances(ctx context.Context) error {
	// lock the Instance with a fail-fast advisory lock context.
	// this allows concurrent processing of many instances by one or more maestro instances exclusively.
	lockOwnerID, acquired, err := s.lockFactory.NewNonBlockingLock(ctx, "maestro-instances-pulse-check", db.Instances)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		return fmt.Errorf("error obtaining the event lock: %v", err)
	}
	// skip if the lock is not acquired
	if !acquired {
		return fmt.Errorf("failed to acquire the lock as another maestro instance is checking instances")
	}
	// Instances not pulsing within the last three check intervals are considered as dead.
	instances, err := s.instanceDao.FindByUpdatedTime(ctx, time.Now().Add(time.Duration(int64(-3*time.Second)*s.checkInterval)))
	if err != nil {
		return fmt.Errorf("unable to get outdated maestro instances: %s", err.Error())
	}
	deletedInstanceIDs := []string{}
	for _, i := range instances {
		deletedInstanceIDs = append(deletedInstanceIDs, i.ID)
	}

	if len(deletedInstanceIDs) > 0 {
		// trigger statusresync for dead instances only once even if there are multiple dead instances
		// will retry in next check if the statusresync fails
		if err := s.sourceClient.Resync(ctx); err != nil {
			return fmt.Errorf("unable to trigger statusresync for maestro instance(s): %s", err.Error())
		}

		// batch delete dead instances
		if err := s.instanceDao.DeleteByIDs(ctx, deletedInstanceIDs); err != nil {
			return fmt.Errorf("unable to delete dead maestro instances: %s", err.Error())
		}
	}

	return nil
}
