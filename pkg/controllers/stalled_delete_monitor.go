package controllers

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/services"
)

// stalledDeleteLogLimit bounds how many stalled resources are sampled per tick, so a large
// backlog cannot flood the log. The gauge below always reports the true total count.
const stalledDeleteLogLimit = 20

var stalledDeleteResourcesGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Subsystem: specControllerMetricsSubsystem,
		Name:      "stalled_delete_resources",
		Help: "Number of resources soft-deleted longer than the stalled-delete warning threshold " +
			"without the agent confirming deletion",
	},
)

func init() {
	prometheus.MustRegister(stalledDeleteResourcesGauge)
}

// StalledDeleteMonitor periodically reports resources whose deletion has been requested
// (soft-deleted) but not yet completed (hard-deleted) within a warning threshold, i.e. the
// agent has not confirmed the ManifestWork was removed from the managed cluster (see
// event_server.go's handling of the ResourceDeleted status condition).
//
// MarkAsDeleting keeps caller retries idempotent; DeleteRecovery independently republishes
// unacknowledged deletes with persistent, bounded backoff. This monitor is diagnostic only: its
// threshold identifies old tombstones but neither changes recovery behavior nor authorizes
// hard deletion. The backlog gauge is refreshed by every replica; an advisory lock only
// deduplicates the per-resource diagnostic logs.
type StalledDeleteMonitor struct {
	resourceService services.ResourceService
	lockFactory     db.LockFactory
	threshold       time.Duration
}

func NewStalledDeleteMonitor(
	resourceService services.ResourceService,
	lockFactory db.LockFactory,
	thresholdSeconds int,
) *StalledDeleteMonitor {
	return &StalledDeleteMonitor{
		resourceService: resourceService,
		lockFactory:     lockFactory,
		threshold:       time.Duration(thresholdSeconds) * time.Second,
	}
}

func (m *StalledDeleteMonitor) Run(ctx context.Context) {
	logger := klog.FromContext(ctx)

	count, svcErr := m.resourceService.CountStalledDeletes(ctx, m.threshold)
	if svcErr != nil {
		logger.Error(svcErr, "Failed to count stalled delete resources")
		return
	}

	// Refresh on every replica so an instance that did not acquire the logging lock
	// cannot keep exporting a stale gauge after leadership moves or the backlog drains.
	stalledDeleteResourcesGauge.Set(float64(count))
	if count == 0 {
		return
	}

	lockOwnerID, acquired, err := m.lockFactory.NewNonBlockingLock(ctx, "maestro-stalled-delete-monitor", db.Instances)
	defer m.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		logger.Error(err, "Error obtaining the stalled delete monitor lock")
		return
	}
	if !acquired {
		logger.V(4).Info("Another instance is monitoring stalled deletes, skip")
		return
	}

	stalled, svcErr := m.resourceService.FindStalledDeletes(ctx, m.threshold, stalledDeleteLogLimit)
	if svcErr != nil {
		logger.Error(svcErr, "Failed to list stalled delete resources for diagnostic logging")
		return
	}

	for _, resource := range stalled {
		logger.Info("resource has been soft-deleted longer than the stalled-delete threshold; agent has not confirmed deletion",
			"deletedAt", resource.DeletedAt.Time,
			"age", time.Since(resource.DeletedAt.Time).String())
	}

	logger.Info("Found resources stalled in deletion",
		"count", count, "loggedSample", len(stalled), "threshold", m.threshold.String())
}
