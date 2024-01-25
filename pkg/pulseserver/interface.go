package pulseserver

import (
	"context"
)

// PulseServer represents a server responsible for periodic heartbeat updates and
// checking the liveness of Maestro instances, triggering status resync based on
// instances' status and other conditions.
type PulseServer interface {
	// Start initializes and runs the pulse server, updating and checking Maestro instances' liveness,
	// start subscribing to status update messages and
	// triggering status resync based on different implementations.
	Start(ctx context.Context) error
}

// StatusDispatcher defines methods for coordinating resource status updates
// in the context of multiple active maestro instances. Each instance subscribes
// to the same topic for resource status updates.
//
// The dispatcher manages the mapping between Maestro instances and consumers (agents),
// ensuring that only one instance processes specific resource status updates from a consumer.
// Note: Enable this interface only when shared subscription is disabled.
type StatusDispatcher interface {
	// Dispatch determines if the current Maestro instance should process the resource status update based on the consumer ID.
	Dispatch(consumerID string) bool
}
