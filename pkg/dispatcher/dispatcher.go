package dispatcher

import (
	"context"
)

// Dispatcher defines methods for coordinating resource status updates in the context of multiple active maestro instances.
//
// The dispatcher ensures only one instance processes specific resource status updates from a consumer.
// It needs to handle status resync based on the instances' status and different implementations.
type Dispatcher interface {
	// Start initializes and runs the dispatcher based on different implementations.
	Start(ctx context.Context)
	// Dispatch determines if the current Maestro instance should process the resource status update based on the consumer ID.
	Dispatch(consumerID string) bool
	// OnInstanceUp is called when a new maestro instance is up.
	OnInstanceUp(instanceID string) error
	// OnInstanceDown is called when a maestro instance is inactive.
	OnInstanceDown(instanceID string) error
}
