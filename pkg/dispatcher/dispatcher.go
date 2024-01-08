package dispatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	logger "github.com/openshift-online/maestro/pkg/logger"
	"k8s.io/client-go/util/workqueue"
	cegeneric "open-cluster-management.io/api/cloudevents/generic"
	cetypes "open-cluster-management.io/api/cloudevents/generic/types"
)

// Dispatcher interface outlines methods for coordinating resource status updates
// in the context of multiple active Maestro instances. Each instance subscribes
// to a shared topic for resource status updates.
//
// The dispatcher manages the mapping between maestro instances and consumers (agents),
// ensuring that only one instance processes specific resource status updates from a consumer.
type Dispatcher interface {
	// Start initiates the dispatcher with the provided context
	Start(ctx context.Context) error
	// Dispatch determines if the current maestro instance should process the resource status update based on the consumer ID.
	Dispatch(consumerID string) bool
	// ProcessResync Attempts to resync resource status updates for new consumers added to the current maestro instance
	// using the cloudevents source client. Return True if the resync is successful.
	ProcessResync(ctx context.Context, client *cegeneric.CloudEventSourceClient[*api.Resource], sourceID string) bool
}

type hasher struct{}

// Sum64 returns the 64-bit xxHash of data
func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// DispatcherImpl is an implementation of the Dispatcher interface.
// It utilizes a consistent hash ring to distribute resource status updates
// to the appropriate Maestro instance based on consumer ID.
type DispatcherImpl struct {
	instanceDao   dao.InstanceDao
	consumerDao   dao.ConsumerDao
	instanceID    string
	pulseInterval int64
	checkInterval int64
	consumerSet   mapset.Set[string]
	consistent    *consistent.Consistent
	workQueue     workqueue.RateLimitingInterface
}

// NewDispatcher creates a new instance of the Dispatcher interface with the provided parameters.
// It initializes the DispatcherImpl struct, setting up data access objects, instance identifier,
// pulse and check intervals, expiration period, consumer set, and a resync channel.
func NewDispatcher(instanceDao dao.InstanceDao, consumerDao dao.ConsumerDao, instanceID string, pulseInterval, checkInterval int64) Dispatcher {
	return &DispatcherImpl{
		instanceDao:   instanceDao,
		consumerDao:   consumerDao,
		instanceID:    instanceID,
		pulseInterval: pulseInterval,
		checkInterval: checkInterval,
		consumerSet:   mapset.NewSet[string](),
		workQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "dispatcher"),
	}
}

// Start initializes and runs the DispatcherImpl instance.
// It periodically updates the heartbeat for the current Maestro instance,
// checks Maestro instances and consumers to maintain the consistent hash ring,
// keeps track of consumers that belong to the current instance, and sends resync signals
// for consumers that are added to the current instance.
func (d *DispatcherImpl) Start(ctx context.Context) error {
	logger := logger.NewOCMLogger(ctx)
	instance := &api.Instance{
		Name: d.instanceID,
		Meta: api.Meta{
			ID:        d.instanceID,
			UpdatedAt: time.Now(),
		},
	}
	instance, err := d.instanceDao.UpSert(ctx, instance)
	if err != nil {
		return fmt.Errorf("unable to create maestro instance: %s", err.Error())
	}
	logger.Infof("Created maestro instance %s in database", instance.Name)

	instances, err := d.instanceDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list maestro instances: %s", err.Error())
	}

	// initialize the consistent hash ring
	members := []consistent.Member{}
	for _, instance := range instances {
		// Instances not pulsing within the last three check intervals are considered as dead.
		if instance.UpdatedAt.After(time.Now().Add(time.Duration(-3*d.checkInterval) * time.Second)) {
			members = append(members, instance)
		}
	}

	d.consistent = consistent.New(members, consistent.Config{
		PartitionCount:    7,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            hasher{},
	})
	logger.Infof("Initialized consistent hash ring with members: %v", members)

	consumers, err := d.consumerDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to list consumers: %s", err.Error())
	}

	toAddConsumers := []string{}
	// initialize the consumer set for current instance
	for _, consumer := range consumers {
		instanceID := d.consistent.LocateKey([]byte(consumer.ID)).String()
		if instanceID == d.instanceID {
			// new consumer added to the current instance, need to resync resource status updates for this consumer
			logger.Infof("Adding new consumer %s to consumer set for instance %s", consumer.ID, d.instanceID)
			toAddConsumers = append(toAddConsumers, consumer.ID)
			d.workQueue.Add(consumer.ID)
		}
	}
	_ = d.consumerSet.Append(toAddConsumers...)
	logger.Infof("Initialized consumers %d for current instance %s", d.consumerSet.Cardinality(), d.instanceID)

	pulseTicker := time.NewTicker(time.Duration(d.pulseInterval) * time.Second)
	checkTicker := time.NewTicker(time.Duration(d.checkInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			pulseTicker.Stop()
			checkTicker.Stop()
			d.workQueue.ShutDown()
			return nil
		case <-pulseTicker.C:
			logger.Infof("Updating heartbeat for maestro instance: %s", instance.Name)
			// update the heartbeat for current instance
			instance.UpdatedAt = time.Now()
			instance, err = d.instanceDao.Replace(ctx, instance)
			if err != nil {
				return fmt.Errorf("unable to update heartbeat for maestro instance: %s", err.Error())
			}
		case <-checkTicker.C:
			logger.Infof("Checking maestro instances and updating the hash ring")
			// Instances not pulsing within the last three check intervals are considered as dead.
			instances, err = d.instanceDao.All(ctx)
			if err != nil {
				return fmt.Errorf("unable to list maestro instances: %s", err.Error())
			}
			for _, instance := range instances {
				// Instances not pulsing within the last three check intervals are considered as dead.
				if instance.UpdatedAt.After(time.Now().Add(time.Duration(-3*d.checkInterval) * time.Second)) {
					d.consistent.Add(instance)
				} else {
					d.consistent.Remove(instance.Name)
				}
			}
			logger.Infof("Members in consistent hash ring: %v", d.consistent.GetMembers())
			// TODO: optimize the performance of consumer set update from database.
			consumers, err = d.consumerDao.All(ctx)
			if err != nil {
				return fmt.Errorf("unable to list consumers: %s", err.Error())
			}
			toAddConsumers, toRemoveConsumers := []string{}, []string{}
			for _, consumer := range consumers {
				instanceID := d.consistent.LocateKey([]byte(consumer.ID)).String()
				if instanceID == d.instanceID {
					if !d.consumerSet.Contains(consumer.ID) {
						// new consumer added to the current instance, need to resync resource status updates for this consumer
						logger.Infof("Adding new consumer %s to consumer set for instance %s", consumer.ID, d.instanceID)
						toAddConsumers = append(toAddConsumers, consumer.ID)
						d.workQueue.Add(consumer.ID)
					}
				} else {
					// remove the consumer from the set if it is not in the current instance
					if d.consumerSet.Contains(consumer.ID) {
						logger.Infof("Removing consumer %s from consumer set for instance %s", consumer.ID, d.instanceID)
						toRemoveConsumers = append(toRemoveConsumers, consumer.ID)
					}
				}
			}
			_ = d.consumerSet.Append(toAddConsumers...)
			d.consumerSet.RemoveAll(toRemoveConsumers...)
			logger.Infof("Consumers length %d for current instance %s", d.consumerSet.Cardinality(), d.instanceID)
		}
	}
}

// Dispatch checks if the provided consumer ID belongs to the current Maestro instance.
// It returns true if the consumer is part of the current instance's consumer set; otherwise, it returns false.
func (d *DispatcherImpl) Dispatch(consumerID string) bool {
	return d.consumerSet.Contains(consumerID)
}

// ProcessResync attempts to resync resource status updates for new consumers added to the current maestro instance
// using the cloudevents source client. It returns true if the resync is successful.
func (d *DispatcherImpl) ProcessResync(ctx context.Context, client *cegeneric.CloudEventSourceClient[*api.Resource], sourceID string) bool {
	consumerID, shutdown := d.workQueue.Get()
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
	defer d.workQueue.Done(consumerID)

	consumerIDStr, ok := consumerID.(string)
	if !ok {
		d.workQueue.Forget(consumerID)
		// return true to indicate that we should continue processing the next item
		return true
	}

	logger := logger.NewOCMLogger(ctx)
	logger.Infof("processing status resync request for consumer %s", consumerIDStr)
	if err := client.Resync(ctx, cetypes.ListOptions{Source: sourceID, ClusterName: consumerIDStr}); err != nil {
		logger.Error(fmt.Sprintf("failed to resync resourcs status for consumer %s: %s", consumerIDStr, err))
		// Put the item back on the workqueue to handle any transient errors.
		d.workQueue.AddRateLimited(consumerID)
	}

	return true
}
