package dispatcher

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
)

var _ Dispatcher = &NoopDispatcher{}

// NoopDispatcher is a no-op implementation of Dispatcher. It will always dispatch the resource status update
// to the current maestro instance. This is the default implementation when shared subscription is enabled.
// Need to trigger status resync from all consumers when an instance is down.
type NoopDispatcher struct {
	sessionFactory db.SessionFactory
	consumerDao    dao.ConsumerDao
	sourceClient   cloudevents.SourceClient
}

// NewNoopDispatcher creates a new NoopDispatcher instance.
func NewNoopDispatcher(sessionFactory db.SessionFactory, sourceClient cloudevents.SourceClient) *NoopDispatcher {
	return &NoopDispatcher{
		sessionFactory: sessionFactory,
		consumerDao:    dao.NewConsumerDao(&sessionFactory),
		sourceClient:   sourceClient,
	}
}

// Start is a no-op implementation.
func (d *NoopDispatcher) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	// handle client reconnected signal and resync status from consumers for this source
	go d.resyncOnReconnect(ctx)

	// listen for server_instance update
	logger.Info("NoopDispatcher listening for server_instances updates")
	go d.sessionFactory.NewListener(ctx, "server_instances", func(ids string) {
		d.onInstanceUpdate(logger, ids)
	})

	// wait until context is canceled
	<-ctx.Done()

}

// resyncOnReconnect listens for client reconnected signal and resyncs all consumers for this source.
func (d *NoopDispatcher) resyncOnReconnect(ctx context.Context) {
	logger := klog.FromContext(ctx)
	// receive client reconnect signal and resync current consumers for this source
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.sourceClient.ReconnectedChan():
			// when receiving a client reconnected signal, we resync all consumers for this source
			// TODO: optimize this to only resync resource status for necessary consumers
			consumerIDs := []string{}
			consumers, err := d.consumerDao.All(ctx)
			if err != nil {
				logger.Error(err, "failed to get all consumers")
				continue
			}

			for _, c := range consumers {
				consumerIDs = append(consumerIDs, c.ID)
			}
			if err := d.sourceClient.Resync(ctx, consumerIDs); err != nil {
				logger.Error(err, "failed to resync resources status for consumers", "consumers", consumerIDs)
			}
		}
	}
}

func (d *NoopDispatcher) onInstanceUpdate(logger klog.Logger, ids string) {
	states := strings.Split(ids, ":")
	if len(states) != 2 {
		logger.Info("watched server instances updated with invalid ids", "IDs", ids)
		return
	}
	idList := strings.Split(states[1], ",")
	if states[0] == "unready" && len(idList) > 0 {
		// only call onInstanceDown once with empty instance id to reduce the number of status resync requests
		if err := d.onInstanceDown(); err != nil {
			logger.Error(err, "failed to call OnInstancesDown")
		}
	}
}

// Dispatch always returns true, indicating that the current maestro instance should process the resource status update.
func (d *NoopDispatcher) Dispatch(consumerID string) bool {
	return true
}

// onInstanceDown calls status resync when there is down instance watched.
func (d *NoopDispatcher) onInstanceDown() error {
	// send resync request to each consumer
	// TODO: optimize this to only resync resource status for necessary consumers
	consumerIDs := []string{}
	ctx := context.TODO()
	consumers, err := d.consumerDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to get all consumers: %s", err.Error())
	}

	for _, c := range consumers {
		consumerIDs = append(consumerIDs, c.ID)
	}

	if err := d.sourceClient.Resync(ctx, consumerIDs); err != nil {
		return fmt.Errorf("unable to trigger statusresync: %s", err.Error())
	}

	return nil
}
