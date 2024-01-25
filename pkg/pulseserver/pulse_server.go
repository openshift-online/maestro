package pulseserver

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/api/meta"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// pulseServerImpl is an implementation of the PulseServer interface.
// It periodically updates the heartbeat for the current maestro instance,
// checks all maestro instances' liveness, and triggers status resync for dead instances.
type pulseServerImpl struct {
	instanceID      string
	pulseInterval   int64
	checkInterval   int64
	instanceDao     dao.InstanceDao
	consumerDao     dao.ConsumerDao
	lockFactory     db.LockFactory
	resourceService services.ResourceService
	sourceClient    cloudevents.SourceClient
}

// NewPulseServerImpl creates and returns a new instance of PulseServerImpl.
// It requires a session factory, resource service, instance ID,
// pulse and check intervals, and a CloudEvents source client.
func NewPulseServerImpl(sessionFactory *db.SessionFactory,
	resourceService services.ResourceService,
	instanceID string, pulseInterval, checkInterval int64,
	sourceClient cloudevents.SourceClient) PulseServer {
	return &pulseServerImpl{
		instanceID:      instanceID,
		pulseInterval:   pulseInterval,
		checkInterval:   checkInterval,
		instanceDao:     dao.NewInstanceDao(sessionFactory),
		consumerDao:     dao.NewConsumerDao(sessionFactory),
		lockFactory:     db.NewAdvisoryLockFactory(*sessionFactory),
		resourceService: resourceService,
		sourceClient:    sourceClient,
	}
}

// Start initializes and runs the pulse server. It performs the following tasks:
//
// 1. Periodically updates the heartbeat for the current maestro instance.
// 2. Checks the liveness of maestro instances.
// 3. Subscribes to resource status update messages.
// 4. Triggers status resync for dead instances.
//
// The function runs until the provided context is canceled or an error occurs.
func (s *pulseServerImpl) Start(ctx context.Context) error {
	logger := logger.NewOCMLogger(ctx)
	instance := &api.ServerInstance{
		Meta: api.Meta{
			ID:        s.instanceID,
			UpdatedAt: time.Now(),
		},
	}
	_, err := s.instanceDao.UpSert(ctx, instance)
	if err != nil {
		return fmt.Errorf("unable to create maestro instance: %s", err.Error())
	}

	// start subscribing to resource status update messages.
	s.startSubscription(ctx)

	pulseTicker := time.NewTicker(time.Duration(s.pulseInterval) * time.Second)
	checkTicker := time.NewTicker(time.Duration(s.checkInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			pulseTicker.Stop()
			checkTicker.Stop()
			return nil
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

// CheckInstances checks all maestro instances' liveness and trigger statusresync for dead instances.
func (s *pulseServerImpl) CheckInstances(ctx context.Context) error {
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

		// send resync request to each consumer
		// TODO: optimize this to only resync resource status for necessary consumers
		consumerIDs := []string{}

		consumers, err := s.consumerDao.All(ctx)
		if err != nil {
			return fmt.Errorf("unable to get all consumers: %s", err.Error())
		}

		for _, c := range consumers {
			consumerIDs = append(consumerIDs, c.ID)
		}

		if err := s.sourceClient.Resync(ctx, consumerIDs); err != nil {
			return fmt.Errorf("unable to trigger statusresync for maestro instance(s): %s", err.Error())
		}

		// batch delete dead instances
		if err := s.instanceDao.DeleteByIDs(ctx, deletedInstanceIDs); err != nil {
			return fmt.Errorf("unable to delete dead maestro instances: %s", err.Error())
		}
	}

	return nil
}

// startSubscription initiates the subscription to resource status update messages.
// It runs asynchronously in the background until the provided context is canceled.
func (s *pulseServerImpl) startSubscription(ctx context.Context) {
	logger := logger.NewOCMLogger(ctx)
	s.sourceClient.Subscribe(ctx, func(action types.ResourceAction, resource *api.Resource) error {
		logger.Infof("received action %s for resource %s", action, resource.ID)
		switch action {
		case types.StatusModified:
			resourceStatus, error := api.JSONMapStausToResourceStatus(resource.Status)
			if error != nil {
				return error
			}

			// if the resource has been deleted from agent, delete it from maestro
			if resourceStatus.ReconcileStatus != nil && meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Deleted") {
				if err := s.resourceService.Delete(ctx, resource.ID); err != nil {
					return err
				}
			} else {
				// update the resource status
				if _, err := s.resourceService.UpdateStatus(ctx, resource); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported action %s", action)
		}

		return nil
	})
}
