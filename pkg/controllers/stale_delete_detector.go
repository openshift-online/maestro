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

func init() {
	prometheus.MustRegister(staleDeleteEventsReconciledTotal)
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
type StaleDeleteDetector struct {
	events      services.EventService
	lockFactory db.LockFactory
	threshold   time.Duration
}

func NewStaleDeleteDetector(
	events services.EventService,
	lockFactory db.LockFactory,
	thresholdSeconds int,
) *StaleDeleteDetector {
	return &StaleDeleteDetector{
		events:      events,
		lockFactory: lockFactory,
		threshold:   time.Duration(thresholdSeconds) * time.Second,
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
}
