package controllers

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/services"
)

var undeliveredResourcesRepublishedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Subsystem: specControllerMetricsSubsystem,
		Name:      "undelivered_resources_republished_total",
		Help:      "Total number of resources re-published because they had no status feedback within the threshold",
	},
)

func init() {
	prometheus.MustRegister(undeliveredResourcesRepublishedTotal)
}

type UndeliveredDetector struct {
	resourceService services.ResourceService
	events          services.EventService
	lockFactory     db.LockFactory
	threshold       time.Duration
}

func NewUndeliveredDetector(
	resourceService services.ResourceService,
	events services.EventService,
	lockFactory db.LockFactory,
	thresholdSeconds int,
) *UndeliveredDetector {
	return &UndeliveredDetector{
		resourceService: resourceService,
		events:          events,
		lockFactory:     lockFactory,
		threshold:       time.Duration(thresholdSeconds) * time.Second,
	}
}

func (d *UndeliveredDetector) Run(ctx context.Context) {
	logger := klog.FromContext(ctx)

	lockOwnerID, acquired, err := d.lockFactory.NewNonBlockingLock(ctx, "maestro-undelivered-check", db.Instances)
	defer d.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		logger.Error(err, "Error obtaining the undelivered resource check lock")
		return
	}
	if !acquired {
		logger.V(4).Info("Another instance is checking undelivered resources, skip")
		return
	}

	resources, svcErr := d.resourceService.FindUndelivered(ctx, d.threshold)
	if svcErr != nil {
		logger.Error(svcErr, "Failed to find undelivered resources")
		return
	}

	if len(resources) == 0 {
		return
	}

	logger.Info("Found resources with no status feedback, re-publishing", "count", len(resources))

	for _, resource := range resources {
		age := time.Since(resource.CreatedAt)
		eventType := api.CreateEventType
		if resource.Version > 1 {
			eventType = api.UpdateEventType
		}

		if _, err := d.events.Create(ctx, &api.Event{
			Source:    "Resources",
			SourceID:  resource.ID,
			EventType: eventType,
		}); err != nil {
			logger.Error(err, "Failed to create re-publish event", "resourceID", resource.ID)
			continue
		}

		undeliveredResourcesRepublishedTotal.Inc()
		logger.Info("Re-published undelivered resource",
			"resourceID", resource.ID,
			"consumerName", resource.ConsumerName,
			"source", resource.Source,
			"age", age.String())
	}
}
