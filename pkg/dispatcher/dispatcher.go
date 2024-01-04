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
	// Dispatch sends resource status updates to the appropriate maestro instance based on the consumer ID
	Dispatch(consumerID string) bool
	// Resync returns a channel signaling the need to resync resource status updates for the provided consumer
	Resync() <-chan string
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
	resyncChan    chan string
	consistent    *consistent.Consistent
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
		resyncChan:    make(chan string, 100),
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

	// initialize the consumer set for current instance
	for _, consumer := range consumers {
		instanceID := d.consistent.LocateKey([]byte(consumer.ID)).String()
		if instanceID == d.instanceID {
			// new consumer added to the current instance, need to resync resource status updates for this consumer
			d.consumerSet.Add(consumer.ID)
			logger.Infof("Added new consumer %s to consumer set for instance %s", consumer.ID, d.instanceID)
			d.resyncChan <- consumer.ID
		}
	}
	logger.Infof("Initialized consumers %s for current instance %s", d.consumerSet.String(), d.instanceID)

	pulseTicker := time.NewTicker(time.Duration(d.pulseInterval) * time.Second)
	checkTicker := time.NewTicker(time.Duration(d.checkInterval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			pulseTicker.Stop()
			checkTicker.Stop()
			close(d.resyncChan)
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
			for _, consumer := range consumers {
				instanceID := d.consistent.LocateKey([]byte(consumer.ID)).String()
				if instanceID == d.instanceID {
					if added := d.consumerSet.Add(consumer.ID); added {
						// new consumer added to the current instance, need to resync resource status updates for this consumer
						logger.Infof("Added new consumer %s to consumer set for instance %s", consumer.ID, d.instanceID)
						d.resyncChan <- consumer.ID
					}
				} else {
					// consistent hashing minimize this case but it is still possible
					// remove the consumer from the set if it is not in the current instance
					if d.consumerSet.Contains(consumer.ID) {
						logger.Infof("Removed consumer %s from consumer set for instance %s", consumer.ID, d.instanceID)
						d.consumerSet.Remove(consumer.ID)
					}
				}
			}
			logger.Infof("Consumers %s for current instance %s", d.consumerSet.String(), d.instanceID)
		}
	}
}

// Dispatch checks if the provided consumer ID belongs to the current Maestro instance.
// It returns true if the consumer is part of the current instance's consumer set; otherwise, it returns false.
func (d *DispatcherImpl) Dispatch(consumerID string) bool {
	return d.consumerSet.Contains(consumerID)
}

// Resync returns the channel used for signaling the need to resync resource status updates for consumers.
func (d *DispatcherImpl) Resync() <-chan string {
	return d.resyncChan
}

// IsInstanceExists checks if the provided instance ID exists in the consistent hash ring.
func (d *DispatcherImpl) IsInstanceExists(instanceID string) bool {
	for _, member := range d.consistent.GetMembers() {
		if member.String() == instanceID {
			return true
		}
	}
	return false
}
