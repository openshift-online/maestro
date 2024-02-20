package event

import (
	"context"
	"sync"

	"github.com/openshift-online/maestro/pkg/api"
)

// EventClient is a client that can receive resource status change events.
type EventClient struct {
	clusterName string
	resChan     chan *api.Resource
}

// NewEventClient creates a new event client.
func NewEventClient(clusterName string) *EventClient {
	return &EventClient{
		clusterName: clusterName,
		resChan:     make(chan *api.Resource),
	}
}

// Receive returns a channel that can be used to receive resource status change events.
func (c *EventClient) Receive() <-chan *api.Resource {
	return c.resChan
}

// EventHub is a hub that can broadcast resource status change events to registered clients.
type EventHub struct {
	mu sync.RWMutex

	// Registered clients.
	clients map[*EventClient]struct{}

	// Inbound messages from the clients.
	broadcast chan *api.Resource
}

// NewEventHub creates a new event hub.
func NewEventHub() *EventHub {
	return &EventHub{
		clients:   make(map[*EventClient]struct{}),
		broadcast: make(chan *api.Resource),
	}
}

// Register registers a client.
func (h *EventHub) Register(client *EventClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}
}

// Unregister unregisters a client.
func (h *EventHub) Unregister(client *EventClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, client)
	close(client.resChan)
}

// Broadcast broadcasts a resource status change event to all registered clients.
func (h *EventHub) Broadcast(res *api.Resource) {
	h.broadcast <- res
}

// Start starts the event hub and waits for events to broadcast.
func (h *EventHub) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case res := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if client.clusterName == res.ConsumerID || client.clusterName == "+" {
					client.resChan <- res
				}
			}
			h.mu.RUnlock()
		}
	}
}
