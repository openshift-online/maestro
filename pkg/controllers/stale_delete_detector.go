package controllers

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/services"
)

var staleDeleteEventsReconciledTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Subsystem: specControllerMetricsSubsystem,
		Name:      "stale_delete_events_reconciled_total",
		Help:      "Total number of stuck delete events retired because their resource was soft-deleted with no agent to acknowledge the delete",
	},
)

var staleDeleteHardDeletesTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Subsystem: specControllerMetricsSubsystem,
		Name:      "stale_delete_hard_deletes_total",
		Help:      "Total number of resources hard-deleted, with a synthetic delete status event sent, after remaining soft-deleted past the hard-delete threshold with no agent acknowledgment",
	},
)

func init() {
	prometheus.MustRegister(staleDeleteEventsReconciledTotal)
	prometheus.MustRegister(staleDeleteHardDeletesTotal)
}

// StaleDeleteDetector periodically retires delete events that can never be reconciled:
// the resource has been soft-deleted (deletion requested) but its agent is gone and never
// acknowledged the delete, so the resource is never hard-deleted. Because the resource is
// read Unscoped, PredicateEvent keeps finding it and never takes the 404 mark-reconciled
// fast path, so the single spec-event worker skips the event on every pass and the periodic
// sync re-enqueues it forever, starving unrelated create/update events. Retiring these
// events (marking them reconciled) breaks that loop.
//
// It runs as a singleton across all Maestro instances via an advisory lock and evaluates a
// global view of the database, so it does not depend on any instance's local subscriber set.
// The threshold ensures a delete is only retired once it has been pending long enough that a
// still-connected agent (on any instance) would already have acknowledged it.
//
// If hardDeleteThreshold is greater than zero, resources that remain soft-deleted past that
// (much larger) threshold are assumed to have been successfully deleted by their agent
// despite never acknowledging it: the detector hard-deletes them and sends a synthetic
// delete status event, so watchers (e.g. Clusters Service) stop retrying. See
// services.FinalizeStaleDelete for the trade-off this involves.
type StaleDeleteDetector struct {
	events              services.EventService
	resources           services.ResourceService
	statusEvents        services.StatusEventService
	lockFactory         db.LockFactory
	threshold           time.Duration
	hardDeleteThreshold time.Duration
}

func NewStaleDeleteDetector(
	events services.EventService,
	resources services.ResourceService,
	statusEvents services.StatusEventService,
	lockFactory db.LockFactory,
	thresholdSeconds int,
	hardDeleteThresholdSeconds int,
) *StaleDeleteDetector {
	return &StaleDeleteDetector{
		events:              events,
		resources:           resources,
		statusEvents:        statusEvents,
		lockFactory:         lockFactory,
		threshold:           time.Duration(thresholdSeconds) * time.Second,
		hardDeleteThreshold: time.Duration(hardDeleteThresholdSeconds) * time.Second,
	}
}

func (d *StaleDeleteDetector) Run(ctx context.Context) {
	logger := klog.FromContext(ctx)

	lockOwnerID, acquired, err := d.lockFactory.NewNonBlockingLock(ctx, "maestro-stale-delete-check", db.Instances)
	defer d.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		logger.Error(err, "Error obtaining the stale delete event check lock")
		return
	}
	if !acquired {
		logger.V(4).Info("Another instance is checking stale delete events, skip")
		return
	}

	count, svcErr := d.events.ReconcileStaleDeleteEvents(ctx, d.threshold)
	if svcErr != nil {
		logger.Error(svcErr, "Failed to reconcile stale delete events")
		return
	}

	if count > 0 {
		staleDeleteEventsReconciledTotal.Add(float64(count))
		if count >= dao.StaleDeleteReconcileBatchSize {
			logger.Info("Retired stale delete events, batch limit reached so more likely remain (draining on subsequent ticks)",
				"count", count, "batchLimit", dao.StaleDeleteReconcileBatchSize, "threshold", d.threshold.String())
		} else {
			logger.Info("Retired stale delete events for soft-deleted resources with no agent",
				"count", count, "threshold", d.threshold.String())
		}
	}

	if d.hardDeleteThreshold <= 0 {
		return
	}

	d.hardDeleteStaleResources(ctx)
}

// hardDeleteStaleResources hard-deletes resources that remain soft-deleted past
// hardDeleteThreshold, assuming their delete succeeded on the agent even though it was
// never acknowledged. See services.FinalizeStaleDelete for the trade-off this involves.
func (d *StaleDeleteDetector) hardDeleteStaleResources(ctx context.Context) {
	logger := klog.FromContext(ctx)

	resources, svcErr := d.resources.FindStaleDeleting(ctx, d.hardDeleteThreshold)
	if svcErr != nil {
		logger.Error(svcErr, "Failed to find stale deleting resources for hard-delete")
		return
	}

	hardDeleted := 0
	for _, resource := range resources {
		if svcErr := services.FinalizeStaleDelete(ctx, resource, d.resources, d.statusEvents); svcErr != nil {
			logger.Error(svcErr, "Failed to hard-delete stale deleting resource", "resourceID", resource.ID)
			continue
		}
		hardDeleted++
	}

	if hardDeleted > 0 {
		staleDeleteHardDeletesTotal.Add(float64(hardDeleted))
		logger.Info("Hard-deleted resources stuck soft-deleted past the hard-delete threshold",
			"count", hardDeleted, "threshold", d.hardDeleteThreshold.String())
		if len(resources) >= dao.StaleHardDeleteBatchSize {
			logger.Info("Hard-delete batch limit reached, more likely remain (draining on subsequent ticks)",
				"batchLimit", dao.StaleHardDeleteBatchSize)
		}
	}
}
