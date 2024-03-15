package dispatcher

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/client/cloudevents"
	"github.com/openshift-online/maestro/pkg/dao"
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
	consumerNames := []string{}
	ctx := context.TODO()
	consumers, err := d.consumerDao.All(ctx)
	if err != nil {
		return fmt.Errorf("unable to get all consumers: %s", err.Error())
	}

	for _, c := range consumers {
		consumerNames = append(consumerNames, c.Name)
	}

	if err := d.sourceClient.Resync(ctx, consumerNames); err != nil {
		return fmt.Errorf("unable to trigger statusresync: %s", err.Error())
	}

	return nil
}
