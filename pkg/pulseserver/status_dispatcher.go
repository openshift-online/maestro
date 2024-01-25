package pulseserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/util/workqueue"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

const (
	maxConcurrentResyncHandlers = 10
)

// hasher is an implementation of consistent.Hasher (github.com/buraksezer/consistent) interface
type hasher struct{}

// Sum64 returns the 64-bit xxHash of data
func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// pulseServerWithStatusDispatcher is an implementation of the PulseServer and StatusDispatcher interfaces.
// It manages periodic heartbeat updates, checks maestro instances' liveness,
// and triggers status resynchronization based on consumers and instances mapping in a consistent hash ring.
// The implementation utilizes the consistent hash ring to distribute resource status updates to the appropriate
// maestro instance based on consumer ID.
type pulseServerWithStatusDispatcher struct {
	instanceID      string
	pulseInterval   int64
	checkInterval   int64
	instanceDao     dao.InstanceDao
	consumerDao     dao.ConsumerDao
	lockFactory     db.LockFactory
	resourceService services.ResourceService
	sourceClient    cloudevents.SourceClient
	consumerSet     mapset.Set[string]
	consistent      *consistent.Consistent
	workQueue       workqueue.RateLimitingInterface
}

// NewPulseServerWithStatusDispatcher creates and returns a new instance of PulseServerWithStatusDispatcher.
// It requires a session factory, resource service, instance ID,
// pulse and check intervals, and a CloudEvents source client.
func NewPulseServerWithStatusDispatcher(sessionFactory *db.SessionFactory,
	resourceService services.ResourceService,
	instanceID string, pulseInterval, checkInterval int64,
	sourceClient cloudevents.SourceClient) PulseServer {
	return &pulseServerWithStatusDispatcher{
		instanceID:      instanceID,
		pulseInterval:   pulseInterval,
		checkInterval:   checkInterval,
		instanceDao:     dao.NewInstanceDao(sessionFactory),
		consumerDao:     dao.NewConsumerDao(sessionFactory),
		lockFactory:     db.NewAdvisoryLockFactory(*sessionFactory),
		resourceService: resourceService,
		sourceClient:    sourceClient,
		consumerSet:     mapset.NewSet[string](),
		workQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "status-dispatcher"),
	}
}

// Dispatch checks if the provided consumer ID is owned by the current maestro instance.
// It returns true if the consumer is part of the current instance's consumer set; otherwise, it returns false.
func (d *pulseServerWithStatusDispatcher) Dispatch(consumerID string) bool {
	return d.consumerSet.Contains(consumerID)
}

// Start initializes and runs the pulseServerWithStatusDispatcher instance.
// It performs the following tasks:
//
// 1. Periodically updates the heartbeat for the current maestro instance.
// 2. Periodically checks the active maestro instances and consumers to maintain the consistent hash ring.
// 3. Signals resync for newly added consumers for the current instance.
// 4. Starts the status resync workers to process status resync requests.
// 5. Subscribes to resource status update messages.
//
// The function runs until the provided context is canceled or an error occurs.
func (s *pulseServerWithStatusDispatcher) Start(ctx context.Context) error {
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

	instances, err := s.instanceDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list maestro instances: %s", err.Error())
	}

	// initialize the consistent hash ring
	members := []consistent.Member{}
	for _, instance := range instances {
		// Instances not pulsing within the last three check intervals are considered as inactive.
		if instance.UpdatedAt.After(time.Now().Add(time.Duration(int64(-3*time.Second) * s.checkInterval))) {
			members = append(members, instance)
		}
	}

	s.consistent = consistent.New(members, consistent.Config{
		PartitionCount:    30,   // consumer IDs are distributed among partitions, select a big PartitionCount for more consumers.
		ReplicationFactor: 20,   // the numbers for maestro instances to be replicated on consistent hash ring.
		Load:              1.25, // Load is used to calculate average load, 1.25 is reasonable for most cases.
		Hasher:            hasher{},
	})
	logger.V(4).Infof("Initialized consistent hash ring with members: %v", members)

	consumers, err := s.consumerDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list consumers: %s", err.Error())
	}

	toAddConsumers := []string{}
	// initialize the consumer set for current instance
	for _, consumer := range consumers {
		instanceID := s.consistent.LocateKey([]byte(consumer.ID)).String()
		if instanceID == s.instanceID {
			// new consumer added to the current instance, need to resync resource status updates for this consumer
			logger.V(4).Infof("Adding new consumer %s to consumer set for instance %s", consumer.ID, s.instanceID)
			toAddConsumers = append(toAddConsumers, consumer.ID)
			s.workQueue.Add(consumer.ID)
		}
	}
	_ = s.consumerSet.Append(toAddConsumers...)
	logger.V(4).Infof("Initialized consumers %s for current instance %s", s.consumerSet.String(), s.instanceID)

	// start the status resync workers
	go s.startStatusResyncWorkers(ctx)

	// start subscribing to resource status update messages.
	s.startSubscription(ctx)

	pulseTicker := time.NewTicker(time.Duration(s.pulseInterval) * time.Second)
	checkTicker := time.NewTicker(time.Duration(s.checkInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			pulseTicker.Stop()
			checkTicker.Stop()
			s.workQueue.ShutDown()
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
			logger.V(4).Infof("Checking maestro instances liveness and updating the consistent hash ring")
			if err := s.UpdateConsistent(ctx); err != nil {
				// log and ignore the error and continue to tolerate the intermittent issue
				logger.Error(fmt.Sprintf("Unable to update consistent hash ring: %s", err.Error()))
			}
		}
	}
}

// UpdateConsistent updates the consistent hash ring based on the active maestro instances and consumers.
// It keeps track of consumers belonging to the current instance and signals resync for added consumers.
func (s *pulseServerWithStatusDispatcher) UpdateConsistent(ctx context.Context) error {
	logger := logger.NewOCMLogger(ctx)
	instances, err := s.instanceDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list maestro instances: %s", err.Error())
	}
	for _, instance := range instances {
		// Instances not pulsing within the last three check intervals are considered as active.
		if instance.UpdatedAt.After(time.Now().Add(time.Duration(int64(-3*time.Second) * s.checkInterval))) {
			s.consistent.Add(instance)
		} else {
			s.consistent.Remove(instance.ID)
		}
	}
	logger.V(4).Infof("Members in consistent hash ring: %v", s.consistent.GetMembers())

	// TODO: optimize the performance of consumer set update from database.
	consumers, err := s.consumerDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list consumers: %s", err.Error())
	}
	toAddConsumers, toRemoveConsumers := []string{}, []string{}
	for _, consumer := range consumers {
		instanceID := s.consistent.LocateKey([]byte(consumer.ID)).String()
		if instanceID == s.instanceID {
			if !s.consumerSet.Contains(consumer.ID) {
				// new consumer added to the current instance, need to resync resource status updates for this consumer
				logger.V(4).Infof("Adding new consumer %s to consumer set for instance %s", consumer.ID, s.instanceID)
				toAddConsumers = append(toAddConsumers, consumer.ID)
				s.workQueue.Add(consumer.ID)
			}
		} else {
			// remove the consumer from the set if it is not in the current instance
			if s.consumerSet.Contains(consumer.ID) {
				logger.V(4).Infof("Removing consumer %s from consumer set for instance %s", consumer.ID, s.instanceID)
				toRemoveConsumers = append(toRemoveConsumers, consumer.ID)
			}
		}
	}
	_ = s.consumerSet.Append(toAddConsumers...)
	s.consumerSet.RemoveAll(toRemoveConsumers...)
	logger.V(4).Infof("Consumers %s for current instance %s", s.consumerSet.String(), s.instanceID)

	return nil
}

// processNextResync attempts to resync resource status updates for new consumers added to the current maestro instance
// using the cloudevents source client. It returns true if the resync is successful.
func (s *pulseServerWithStatusDispatcher) processNextResync(ctx context.Context) bool {
	consumerID, shutdown := s.workQueue.Get()
	if shutdown {
		// workqueue has been shutdown, return false
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer s.workQueue.Done(consumerID)

	consumerIDStr, ok := consumerID.(string)
	if !ok {
		s.workQueue.Forget(consumerID)
		// return true to indicate that we should continue processing the next item
		return true
	}

	logger := logger.NewOCMLogger(ctx)
	logger.Infof("processing status resync request for consumer %s", consumerIDStr)
	if err := s.sourceClient.Resync(ctx, []string{consumerIDStr}); err != nil {
		logger.Error(fmt.Sprintf("failed to resync resourcs status for consumer %s: %s", consumerIDStr, err))
		// Put the item back on the workqueue to handle any transient errors.
		s.workQueue.AddRateLimited(consumerID)
	}

	return true
}

// startStatusResyncWorkers starts the status resync workers to process status resync requests.
func (s *pulseServerWithStatusDispatcher) startStatusResyncWorkers(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(maxConcurrentResyncHandlers)
	for i := 0; i < maxConcurrentResyncHandlers; i++ {
		go func() {
			defer wg.Done()
			for s.processNextResync(ctx) {
			}
		}()
	}
	wg.Wait()
}

// startSubscription initiates the subscription to resource status update messages.
// It runs asynchronously in the background until the provided context is canceled.
func (s *pulseServerWithStatusDispatcher) startSubscription(ctx context.Context) {
	logger := logger.NewOCMLogger(ctx)
	s.sourceClient.Subscribe(ctx, func(action types.ResourceAction, resource *api.Resource) error {
		logger.Infof("received action %s for resource %s", action, resource.ID)
		switch action {
		case types.StatusModified:
			if !s.Dispatch(resource.ConsumerID) {
				// the resource is not owned by the current instance, skip
				return nil
			}

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
