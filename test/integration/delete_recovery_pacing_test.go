package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

// TestDeleteRecoverySlowCommitPacing exercises actual deferred COMMIT work,
// not a sleep inside the transaction callback or an adjusted scheduler clock.
func TestDeleteRecoverySlowCommitPacing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "slow-commit-1", "consumer-1", time.Hour)
	recoveryTombstone(conn, "slow-commit-2", "consumer-2", time.Hour)
	Expect(conn.Exec(`CREATE FUNCTION slow_recovery_commit_test() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_sleep(1.2); RETURN NEW; END $$;
		CREATE CONSTRAINT TRIGGER slow_recovery_commit_test AFTER INSERT ON events
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION slow_recovery_commit_test()`).Error).To(Succeed())
	t.Cleanup(func() {
		Expect(conn.Exec("DROP TRIGGER IF EXISTS slow_recovery_commit_test ON events; DROP FUNCTION IF EXISTS slow_recovery_commit_test()").Error).To(Succeed())
	})
	start := time.Now()
	result, err := recoveryForTest(t, h, 1).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1))
	Expect(time.Since(start)).To(BeNumerically(">=", 1200*time.Millisecond))
	committed := time.Now()
	result, err = recoveryForTest(t, h, 1).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(BeZero(), "a deferred COMMIT longer than the interval must not admit an immediate successor")
	Expect(result).To(Equal(dao.DeleteRecoveryResult{Cooldown: true}))
	var deadline time.Time
	Expect(conn.Raw("SELECT next_at FROM delete_recovery_schedule").Scan(&deadline).Error).To(Succeed())
	Expect(deadline.Sub(committed)).To(BeNumerically(">=", time.Second))
	Expect(conn.Exec("DROP TRIGGER slow_recovery_commit_test ON events; DROP FUNCTION slow_recovery_commit_test()").Error).To(Succeed())

	// Acknowledgement does not wait for the scheduler's deliberate cooldown.
	ackStart := time.Now()
	ackCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	Expect(dao.NewResourceDao(&h.DBFactory).Delete(ackCtx, "slow-commit-1", true)).To(Succeed())
	Expect(time.Now().Before(deadline)).To(BeTrue())
	t.Logf("work commit=%s cooldown deadline=%s gap=%s acknowledgement=%s",
		committed.Format(time.RFC3339Nano), deadline.Format(time.RFC3339Nano),
		deadline.Sub(committed), time.Since(ackStart))

	var wg sync.WaitGroup
	results := make(chan dao.DeleteRecoveryResult, 12)
	errs := make(chan error, 12)
	for range 12 {
		replica := recoveryForTest(t, h, 1)
		wg.Go(func() {
			result, err := replica.Run(ctx)
			results <- result
			errs <- err
		})
	}
	wg.Wait()
	for range 12 {
		Expect(<-errs).NotTo(HaveOccurred())
		Expect(<-results).To(Equal(dao.DeleteRecoveryResult{}))
	}
	time.Sleep(time.Until(deadline) + 10*time.Millisecond)
	result, err = recoveryForTest(t, h, 1).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1), "cooldown expires without manual repair or catch-up credits")
	var events int64
	Expect(conn.Raw("SELECT count(*) FROM events").Scan(&events).Error).To(Succeed())
	Expect(events).To(Equal(int64(2)), "reported publications match committed events")
}

// TestDeleteRecoveryCooldownRecovery verifies the durable marker survives
// failed/cancelled/killed handshake transactions and abandoned scheduler instances.
func TestDeleteRecoveryCooldownRecovery(t *testing.T) {
	for _, failure := range []string{"commit-error", "cancel", "timeout", "backend-death", "slow-commit"} {
		t.Run(failure, func(t *testing.T) {
			h, _ := test.RegisterIntegration(t)
			ctx := context.Background()
			conn := h.DBFactory.New(ctx)
			recoveryTombstone(conn, "cooldown-work", "consumer", time.Hour)
			result, err := recoveryForTest(t, h, 1).Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Published).To(Equal(1))
			committed := time.Now()

			// No finalization call on the original runner: any new replica must
			// recover the committed marker, including after downtime.
			var pending bool
			Expect(conn.Raw("SELECT cooldown_pending FROM delete_recovery_schedule").Scan(&pending).Error).To(Succeed())
			Expect(pending).To(BeTrue())
			if failure == "commit-error" {
				Expect(conn.Exec(`CREATE FUNCTION cooldown_commit_test() RETURNS trigger LANGUAGE plpgsql AS $$
					BEGIN RAISE EXCEPTION 'injected cooldown commit error'; END $$`).Error).To(Succeed())
			} else {
				Expect(conn.Exec(`CREATE FUNCTION cooldown_commit_test() RETURNS trigger LANGUAGE plpgsql AS $$
					BEGIN PERFORM pg_advisory_xact_lock(597, 987); RETURN NEW; END $$`).Error).To(Succeed())
			}
			Expect(conn.Exec(`CREATE CONSTRAINT TRIGGER cooldown_commit_test AFTER UPDATE ON delete_recovery_schedule
				DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
				WHEN (OLD.cooldown_pending AND NOT NEW.cooldown_pending)
				EXECUTE FUNCTION cooldown_commit_test()`).Error).To(Succeed())
			t.Cleanup(func() {
				Expect(conn.Exec("DROP TRIGGER IF EXISTS cooldown_commit_test ON delete_recovery_schedule; DROP FUNCTION IF EXISTS cooldown_commit_test()").Error).To(Succeed())
			})
			if failure == "commit-error" {
				result, err = recoveryForTest(t, h, 1).Run(ctx)
				Expect(err).To(HaveOccurred())
			} else {
				holder := conn.Begin()
				Expect(holder.Error).NotTo(HaveOccurred())
				defer holder.Rollback()
				Expect(holder.Exec("SELECT pg_advisory_xact_lock(597, 987)").Error).To(Succeed())
				var holderPID int
				Expect(holder.Raw("SELECT pg_backend_pid()").Scan(&holderPID).Error).To(Succeed())
				runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if failure == "timeout" {
					cancel()
					runCtx, cancel = context.WithTimeout(ctx, time.Second)
				}
				defer cancel()
				done := make(chan struct{})
				replica := recoveryForTest(t, h, 1)
				go func() {
					result, err = replica.Run(runCtx)
					close(done)
				}()
				var waiterPID int
				Eventually(func() int {
					Expect(conn.Raw(`SELECT COALESCE(max(pid), 0) FROM pg_stat_activity
						WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))`,
						holderPID).Scan(&waiterPID).Error).To(Succeed())
					return waiterPID
				}, 3*time.Second, 10*time.Millisecond).Should(BeNumerically(">", 0))
				// Competing schedulers return without waiting for this COMMIT.
				probeCtx, probeCancel := context.WithTimeout(ctx, 500*time.Millisecond)
				other, probeErr := recoveryForTest(t, h, 1).Run(probeCtx)
				probeCancel()
				Expect(probeErr).NotTo(HaveOccurred())
				Expect(other).To(Equal(dao.DeleteRecoveryResult{}))
				// The cooldown transaction owns no resource locks.
				ackCtx, ackCancel := context.WithTimeout(ctx, 500*time.Millisecond)
				Expect(dao.NewResourceDao(&h.DBFactory).Delete(ackCtx, "cooldown-work", true)).To(Succeed())
				ackCancel()
				switch failure {
				case "cancel":
					cancel()
				case "backend-death":
					Expect(conn.Exec("SELECT pg_terminate_backend(?)", waiterPID).Error).To(Succeed())
				case "slow-commit":
					time.Sleep(1200 * time.Millisecond)
					Expect(holder.Rollback().Error).To(Succeed())
				}
				Eventually(done, 6*time.Second).Should(BeClosed())
				if failure != "slow-commit" {
					// A cancelled client return is not proof the backend has
					// finished aborting. Keep COMMIT blocked until it is gone.
					Eventually(func() int64 {
						var waiting int64
						Expect(conn.Raw(`SELECT count(*) FROM pg_stat_activity
							WHERE pid = ? AND ? = ANY(pg_blocking_pids(pid))`,
							waiterPID, holderPID).Scan(&waiting).Error).To(Succeed())
						return waiting
					}, 5*time.Second, 10*time.Millisecond).Should(BeZero())
					Expect(holder.Rollback().Error).To(Succeed())
				}
				if failure == "slow-commit" {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(dao.DeleteRecoveryResult{Cooldown: true}))
				} else {
					Expect(err).To(HaveOccurred())
				}
			}
			if failure != "slow-commit" {
				Expect(result).To(Equal(dao.DeleteRecoveryResult{}), "handshake errors report no work")
			}
			var events int64
			Expect(conn.Raw("SELECT count(*) FROM events").Scan(&events).Error).To(Succeed())
			Expect(events).To(Equal(int64(1)), "handshake errors cannot roll back the already reported publication")
			Expect(conn.Raw("SELECT cooldown_pending FROM delete_recovery_schedule").Scan(&pending).Error).To(Succeed())
			Expect(pending).To(Equal(failure != "slow-commit"))
			Expect(conn.Exec("DROP TRIGGER cooldown_commit_test ON delete_recovery_schedule; DROP FUNCTION cooldown_commit_test()").Error).To(Succeed())
			if pending {
				result, err = recoveryForTest(t, h, 1).Run(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(dao.DeleteRecoveryResult{Cooldown: true}))
			}
			var deadline time.Time
			Expect(conn.Raw("SELECT next_at FROM delete_recovery_schedule").Scan(&deadline).Error).To(Succeed())
			Expect(deadline.Sub(committed)).To(BeNumerically(">=", time.Second))
			time.Sleep(max(0, time.Until(deadline)) + 10*time.Millisecond)
			result, err = recoveryForTest(t, h, 1).Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Claimed).To(BeTrue())
			Expect(result.Published).To(BeZero(), "recovery must not duplicate committed work")
			t.Logf("failure=%s recovered; work commit to deadline=%s", failure, deadline.Sub(committed))
		})
	}
}
