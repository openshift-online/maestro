package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openshift-online/maestro/pkg/dao"
)

type recoveryRunnerFunc func(context.Context) (dao.DeleteRecoveryResult, error)

func (f recoveryRunnerFunc) Run(ctx context.Context) (dao.DeleteRecoveryResult, error) {
	return f(ctx)
}

func TestDeleteRecoveryController(t *testing.T) {
	calls := 0
	c := NewDeleteRecoveryController(recoveryRunnerFunc(func(ctx context.Context) (dao.DeleteRecoveryResult, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Fatal("round must have a bounded transaction deadline")
		}
		return dao.DeleteRecoveryResult{}, errors.New("database unavailable")
	}))
	c.Run(context.Background())
	c.Run(context.Background())
	if calls != 2 {
		t.Fatal("a failed round must not stop subsequent recovery")
	}
}
