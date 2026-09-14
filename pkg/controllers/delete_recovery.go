package controllers

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/dao"
)

type DeleteRecoveryRunner interface {
	Run(context.Context) (dao.DeleteRecoveryResult, error)
}

type DeleteRecoveryController struct {
	recovery DeleteRecoveryRunner
}

func NewDeleteRecoveryController(recovery DeleteRecoveryRunner) *DeleteRecoveryController {
	return &DeleteRecoveryController{recovery: recovery}
}

func (c *DeleteRecoveryController) Run(ctx context.Context) {
	// Bound transaction lifetime, including row locks, independently of caller
	// traffic. Failed rounds roll back their events, schedule and fleet deadline.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := c.recovery.Run(ctx)
	logger := klog.FromContext(ctx)
	if err != nil {
		logger.Error(err, "Delete recovery round failed")
		return
	}
	if result.Initialized != 0 || result.Consumers != 0 {
		logger.V(2).Info("Delete recovery round complete", "initialized", result.Initialized,
			"consumers", result.Consumers, "published", result.Published)
	}
}
