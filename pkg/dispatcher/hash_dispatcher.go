package dispatcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/logger"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

var log = logger.GetLogger()

var _ Dispatcher = &HashDispatcher{}

// HashDispatcher is an implementation of Dispatcher. It uses consistent hashing to map consumers to maestro instances.
// Only the maestro instance that is mapped to a consumer will process the resource status update from that consumer.
// Need to trigger status resync for the consumer when an instance is up or down.
type HashDispatcher struct {
	instanceID     string
	sessionFactory db.SessionFactory
	instanceDao    dao.InstanceDao
	consumerDao    dao.ConsumerDao
	sourceClient   cloudevents.SourceClient
	consumerSet    mapset.Set[string]
	workQueue      workqueue.RateLimitingInterface
	consistent     *consistent.Consistent
}

func NewHashDispatcher(instanceID string, sessionFactory db.SessionFactory, sourceClient cloudevents.SourceClient, consistentHashingConfig *config.ConsistentHashConfig) *HashDispatcher {
	return &HashDispatcher{
		instanceID:     instanceID,
		sessionFactory: sessionFactory,
		instanceDao:    dao.NewInstanceDao(&sessionFactory),
		consumerDao:    dao.NewConsumerDao(&sessionFactory),
		sourceClient:   sourceClient,
		consumerSet:    mapset.NewSet[string](),
		workQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "hash-dispatcher"),
		consistent: consistent.New(nil, consistent.Config{
			PartitionCount:    consistentHashingConfig.PartitionCount,
			ReplicationFactor: consistentHashingConfig.ReplicationFactor,
			Load:              consistentHashingConfig.Load,
			Hasher:            hasher{},
		}),
	}
}

// Start initializes and runs the dispatcher, updating the hashing ring and consumer set for the current instance.
func (d *HashDispatcher) Start(ctx context.Context) {
	// start a goroutine to handle status resync requests
	go d.startStatusResyncWorkers(ctx)

	// start a goroutine to periodically check the instances and consumers.
	go wait.UntilWithContext(ctx, d.check, 5*time.Second)

	// listen for server_instance update
	log.Infof("HashDispatcher listening for server_instances updates")
	go d.sessionFactory.NewListener(ctx, "server_instances", d.onInstanceUpdate)

	// start a goroutine to resync current consumers for this source when the client is reconnected
	go d.resyncOnReconnect(ctx)

	// wait until context is canceled
	<-ctx.Done()
	d.workQueue.ShutDown()
}

func (d *HashDispatcher) onInstanceUpdate(ids string) {
	states := strings.Split(ids, ":")
	if len(states) != 2 {
		log.Infof("watched server instances updated with invalid ids: %s", ids)
		return
	}
	idList := strings.Split(states[1], ",")
	if states[0] == "ready" {
		for _, id := range idList {
			if err := d.onInstanceUp(id); err != nil {
				log.Errorf("failed to call OnInstancesUp for instance %s: %s", id, err)
			}
		}
	} else {
		for _, id := range idList {
			if err := d.onInstanceDown(id); err != nil {
				log.Errorf("failed to call OnInstancesDown for instance %s: %s", id, err)
			}
		}
	}
}

// resyncOnReconnect listens for the client reconnected signal and resyncs current consumers for this source.
func (d *HashDispatcher) resyncOnReconnect(ctx context.Context) {
	// receive client reconnect signal and resync current consumers for this source
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.sourceClient.ReconnectedChan():
			// when receiving a client reconnected signal, we resync current consumers for this source
			if err := d.sourceClient.Resync(ctx, d.consumerSet.ToSlice()); err != nil {
				log.Error(fmt.Sprintf("failed to resync resourcs status for consumers (%s), %v", d.consumerSet.ToSlice(), err))
			}
		}
	}
}

// Dispatch checks if the provided consumer ID is owned by the current maestro instance.
// It returns true if the consumer is part of the current instance's consumer set;
// otherwise, it returns false.
func (d *HashDispatcher) Dispatch(consumerName string) bool {
	return d.consumerSet.Contains(consumerName)
}

// onInstanceUp adds the new instance to the hashing ring and updates the consumer set for the current instance.
func (d *HashDispatcher) onInstanceUp(instanceID string) error {
	members := d.consistent.GetMembers()
	for _, member := range members {
		if member.String() == instanceID {
			// instance already exists, hashing ring won't be changed
			return nil
		}
	}

	// add the new instance to the hashing ring
	d.consistent.Add(&api.ServerInstance{
		Meta: api.Meta{
			ID: instanceID,
		},
	})

	return d.updateConsumerSet()
}

// onInstanceDown removes the instance from the hashing ring and updates the consumer set for the current instance.
func (d *HashDispatcher) onInstanceDown(instanceID string) error {
	members := d.consistent.GetMembers()
	deletedMember := true
	for _, member := range members {
		if member.String() == instanceID {
			// the instance is still in the hashing ring
			deletedMember = false
			break
		}
	}

	// if the instance is already deleted, the hash ring won't be changed
	if deletedMember {
		return nil
	}

	// remove the instance from the hashing ring
	d.consistent.Remove(instanceID)

	return d.updateConsumerSet()
}

// updateConsumerSet updates the consumer set for the current instance based on the hashing ring.
func (d *HashDispatcher) updateConsumerSet() error {
	// return if the hashing ring is not ready
	if d.consistent == nil || len(d.consistent.GetMembers()) == 0 {
		return nil
	}

	// get all consumers and update the consumer set for the current instance
	consumers, err := d.consumerDao.All(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to list consumers: %s", err.Error())
	}

	toAddConsumers, toRemoveConsumers := []string{}, []string{}
	for _, consumer := range consumers {
		instanceID := d.consistent.LocateKey([]byte(consumer.Name)).String()
		if instanceID == d.instanceID {
			if !d.consumerSet.Contains(consumer.Name) {
				// new consumer added to the current instance, need to resync resource status updates for this consumer
				toAddConsumers = append(toAddConsumers, consumer.Name)
				d.workQueue.Add(consumer.Name)
			}
		} else {
			// remove the consumer from the set if it is not in the current instance
			if d.consumerSet.Contains(consumer.Name) {
				toRemoveConsumers = append(toRemoveConsumers, consumer.Name)
			}
		}
	}

	_ = d.consumerSet.Append(toAddConsumers...)
	d.consumerSet.RemoveAll(toRemoveConsumers...)
	log.Infof("Consumers set for current instance: %s", d.consumerSet.String())

	return nil
}

// startStatusResyncWorkers starts the status resync workers to process status resync requests.
func (d *HashDispatcher) startStatusResyncWorkers(ctx context.Context) {
	wg := &sync.WaitGroup{}
	maxConcurrentResyncHandlers := 10
	wg.Add(maxConcurrentResyncHandlers)
	for i := 0; i < maxConcurrentResyncHandlers; i++ {
		go func() {
			defer wg.Done()
			for d.processNextResync(ctx) {
			}
		}()
	}
	wg.Wait()
}

// check checks the instances & consumers and updates the hashing ring and consumer set for the current instance.
func (d *HashDispatcher) check(ctx context.Context) {
	instances, err := d.instanceDao.All(ctx)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to get all maestro instances: %s", err.Error()))
		return
	}

	// ensure the hashing ring members are up-to-date
	members := d.consistent.GetMembers()
	for _, member := range members {
		isMemberActive := false
		for _, instance := range instances {
			if member.String() == instance.ID {
				isMemberActive = true
				break
			}
		}
		if !isMemberActive {
			d.consistent.Remove(member.String())
		}
	}

	if err := d.updateConsumerSet(); err != nil {
		log.Error(fmt.Sprintf("Unable to update consumer set: %s", err.Error()))
	}
}

// processNextResync attempts to resync resource status updates for new consumers added to the current maestro instance
// using the cloudevents source client. It returns true if the resync is successful.
func (d *HashDispatcher) processNextResync(ctx context.Context) bool {
	key, shutdown := d.workQueue.Get()
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
	defer d.workQueue.Done(key)

	consumerName, ok := key.(string)
	if !ok {
		d.workQueue.Forget(key)
		// return true to indicate that we should continue processing the next item
		return true
	}

	log.Infof("processing status resync request for consumer %s", consumerName)
	if err := d.sourceClient.Resync(ctx, []string{consumerName}); err != nil {
		log.Error(fmt.Sprintf("failed to resync resourcs status for consumer %s: %s", consumerName, err))
		// Put the item back on the workqueue to handle any transient errors.
		d.workQueue.AddRateLimited(key)
	}

	return true
}

// hasher is an implementation of consistent.Hasher (github.com/buraksezer/consistent) interface
type hasher struct{}

// Sum64 returns the 64-bit xxHash of data
func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}
