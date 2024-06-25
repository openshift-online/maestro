package dispatcher

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/logger"
)

var _ Dispatcher = &NoopDispatcher{}

// NoopDispatcher is a no-op implementation of Dispatcher. It will always dispatch the resource status update
// to the current maestro instance. This is the default implementation when shared subscription is enabled.
// Need to trigger status resync from all consumers when an instance is down.
type NoopDispatcher struct {
	consumerDao  dao.ConsumerDao
	sourceClient cloudevents.SourceClient
}

// NewNoopDispatcher creates a new NoopDispatcher instance.
func NewNoopDispatcher(consumerDao dao.ConsumerDao, sourceClient cloudevents.SourceClient) *NoopDispatcher {
	return &NoopDispatcher{
		consumerDao:  consumerDao,
		sourceClient: sourceClient,
	}
}

// Start is a no-op implementation.
func (d *NoopDispatcher) Start(ctx context.Context) {
	// handle client reconnected signal and resync status from consumers for this source
	d.resyncOnReconnect(ctx)
}

// resyncOnReconnect listens for client reconnected signal and resyncs all consumers for this source.
func (d *NoopDispatcher) resyncOnReconnect(ctx context.Context) {
	log := logger.NewOCMLogger(ctx)
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
				log.Error(fmt.Sprintf("failed to get all consumers: %v", err))
				continue
			}

			for _, c := range consumers {
				consumerIDs = append(consumerIDs, c.ID)
			}
			if err := d.sourceClient.Resync(ctx, consumerIDs); err != nil {
				log.Error(fmt.Sprintf("failed to resync resourcs status for consumers (%s), %v", consumerIDs, err))
			}
		}
	}
}

// Dispatch always returns true, indicating that the current maestro instance should process the resource status update.
func (d *NoopDispatcher) Dispatch(consumerID string) bool {
	return true
}

// OnInstanceUp is a no-op implementation.
func (d *NoopDispatcher) OnInstanceUp(instanceID string) error {
	return nil
}

// OnInstanceDown triggers status resync from all consumers.
func (d *NoopDispatcher) OnInstanceDown(instanceID string) error {
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
