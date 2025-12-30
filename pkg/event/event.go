package event

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api"
)

// resourceHandler is a function that can handle resource status change events.
type resourceHandler func(res *api.Resource) error

// eventClient is a client that can receive and handle resource status change events.
type eventClient struct {
	source  string
	handler resourceHandler
}

// EventBroadcaster is a component that can broadcast resource status change events to registered clients.
type EventBroadcaster struct {
	mu sync.RWMutex

	// registered clients.
	clients map[string]*eventClient

	// inbound messages from the clients.
	broadcast chan *api.Resource
}

// NewEventBroadcaster creates a new event broadcaster.
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		clients:   make(map[string]*eventClient),
		broadcast: make(chan *api.Resource),
	}
}

// Register registers a client and return client id and error channel.
func (h *EventBroadcaster) Register(ctx context.Context, source string, handler resourceHandler) string {
	logger := klog.FromContext(ctx)

	h.mu.Lock()
	defer h.mu.Unlock()

	id := uuid.NewString()
	h.clients[id] = &eventClient{
		source:  source,
		handler: handler,
	}

	logger.Info("registered a broadcaster client", "id", id, "source", source)
	grpcRegisteredSourceClientsGaugeMetric.WithLabelValues(source).Inc()

	return id
}

// Unregister unregisters a client by id
func (h *EventBroadcaster) Unregister(ctx context.Context, id string) {
	logger := klog.FromContext(ctx).WithValues("id", id)
	h.mu.Lock()
	defer h.mu.Unlock()

	client, exists := h.clients[id]
	if !exists {
		logger.Info("attempted to unregister non-existent broadcaster client")
		return
	}

	delete(h.clients, id)
	logger.Info("unregistered broadcaster client", "source", client.source)
	grpcRegisteredSourceClientsGaugeMetric.WithLabelValues(client.source).Dec()
}

// Broadcast broadcasts a resource status change event to all registered clients.
func (h *EventBroadcaster) Broadcast(res *api.Resource) {
	h.broadcast <- res
}

// Start starts the event broadcaster and waits for events to broadcast.
func (h *EventBroadcaster) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)

	logger.Info("Starting event broadcaster")
	for {
		select {
		case <-ctx.Done():
			return
		case res := <-h.broadcast:
			h.mu.RLock()

			if len(h.clients) == 0 {
				logger.Info("no clients registered on this instance")
			}

			for _, client := range h.clients {
				if client.source == res.Source {
					if err := client.handler(res); err != nil {
						logger.Error(err, "failed to handle resource", "resourceID", res.ID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}
