package controllers

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/dao"
)

// DeleteRecoveryRunner executes one durable, fleet-budgeted recovery round.
type DeleteRecoveryRunner interface {
	Run(context.Context) (dao.DeleteRecoveryResult, error)
}

// DeleteRecoverySnapshotReader reads aggregate durable recovery state for metrics.
type DeleteRecoverySnapshotReader interface {
	Snapshot(context.Context) (dao.DeleteRecoverySnapshot, error)
}

// DeleteRecoverySnapshotReportInterval limits database-backed metric snapshots.
const DeleteRecoverySnapshotReportInterval = time.Minute

// DeleteRecoveryController bounds recovery execution time and records round outcomes.
type DeleteRecoveryController struct {
	recovery  DeleteRecoveryRunner
	snapshots DeleteRecoverySnapshotReader
	metrics   *deleteRecoveryMetrics
}

// NewDeleteRecoveryController wraps a recovery runner with shared scheduler metrics.
func NewDeleteRecoveryController(recovery DeleteRecoveryRunner) *DeleteRecoveryController {
	controller := &DeleteRecoveryController{recovery: recovery, metrics: recoveryMetrics}
	if snapshots, ok := recovery.(DeleteRecoverySnapshotReader); ok {
		controller.snapshots = snapshots
	}
	return controller
}

// Run executes one recovery round with a 30-second deadline and records its outcome.
func (c *DeleteRecoveryController) Run(ctx context.Context) {
	// Bound transaction lifetime, including row locks, independently of caller
	// traffic. Failed rounds roll back their events, schedule and fleet deadline.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	result, err := c.recovery.Run(ctx)
	c.metrics.observe(result, err, time.Since(start).Seconds())
	logger := klog.FromContext(ctx)
	if err != nil {
		// Database errors can contain customer identifiers from constraint details.
		logger.Error(nil, "Delete recovery round failed")
		return
	}
	if result.Initialized != 0 || result.Consumers != 0 {
		logger.V(2).Info("Delete recovery round complete", "initialized", result.Initialized,
			"consumers", result.Consumers, "published", result.Published)
	}
}

// Report snapshots durable recovery state independently of Prometheus collection.
func (c *DeleteRecoveryController) Report(ctx context.Context) {
	if c.snapshots == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snapshot, err := c.snapshots.Snapshot(ctx)
	if err != nil {
		c.metrics.snapshotErrors.Inc()
		// Database errors can contain customer identifiers from constraint details.
		klog.FromContext(ctx).Error(nil, "Delete recovery metrics snapshot failed")
		return
	}
	c.metrics.observeSnapshot(snapshot)
}
