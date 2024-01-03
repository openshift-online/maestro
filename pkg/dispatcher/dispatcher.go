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

type Dispatcher interface {
	Start(ctx context.Context) error
	Dispatch(key string) bool
	Resync() <-chan string
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

type DispatcherImpl struct {
	instanceDao              dao.InstanceDao
	consumerDao              dao.ConsumerDao
	instanceID               string
	pulseInterval            int64
	checkInterval            int64
	instanceExpirationPeriod int64
	consumerSet              mapset.Set[string]
	resyncChan               chan string
	consistent               *consistent.Consistent
}

func NewDispatcher(instanceDao dao.InstanceDao, consumerDao dao.ConsumerDao, instanceID string, pulseInterval, checkInterval, instanceExpirationPeriod int64) Dispatcher {
	return &DispatcherImpl{
		instanceDao:              instanceDao,
		consumerDao:              consumerDao,
		instanceID:               instanceID,
		pulseInterval:            pulseInterval,
		checkInterval:            checkInterval,
		instanceExpirationPeriod: instanceExpirationPeriod,
		consumerSet:              mapset.NewSet[string](),
		resyncChan:               make(chan string, 100),
	}
}

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
		if instance.UpdatedAt.After(time.Now().Add(time.Duration(-1*d.instanceExpirationPeriod) * time.Second)) {
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
			// check for instances that are not updated in the last instanceExpirationPeriod seconds
			instances, err = d.instanceDao.All(ctx)
			if err != nil {
				return fmt.Errorf("unable to list maestro instances: %s", err.Error())
			}
			for _, instance := range instances {
				if instance.UpdatedAt.After(time.Now().Add(time.Duration(-1*d.instanceExpirationPeriod) * time.Second)) {
					d.consistent.Add(instance)
				} else {
					d.consistent.Remove(instance.Name)
				}
			}
			logger.Infof("members in consistent hash ring: %v", d.consistent.GetMembers())
			consumers, err = d.consumerDao.All(ctx)
			if err != nil {
				return fmt.Errorf("unable to list consumers: %s", err.Error())
			}
			for _, consumer := range consumers {
				instanceID := d.consistent.LocateKey([]byte(consumer.ID)).String()
				if instanceID == d.instanceID {
					if added := d.consumerSet.Add(consumer.ID); added {
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

func (d *DispatcherImpl) Dispatch(consumerID string) bool {
	return d.consumerSet.Contains(consumerID)
}

func (d *DispatcherImpl) Resync() <-chan string {
	return d.resyncChan
}

func (d *DispatcherImpl) IsInstanceExists(instanceID string) bool {
	for _, member := range d.consistent.GetMembers() {
		if member.String() == instanceID {
			return true
		}
	}
	return false
}
