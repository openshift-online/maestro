package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/db/migrations"
	"github.com/openshift-online/maestro/test"
)

func recoveryForTest(t *testing.T, h *test.Helper, batch int) *dao.DeleteRecovery {
	t.Helper()
	c := config.NewEventServerConfig()
	c.DeleteEventRepublishBatchSize = batch
	recovery, err := dao.NewDeleteRecovery(h.DBFactory, c)
	Expect(err).NotTo(HaveOccurred())
	return recovery
}

func recoveryTombstone(conn *gorm.DB, id, consumer string, age time.Duration) {
	Expect(conn.Exec("INSERT INTO consumers (id, name) VALUES (?, ?) ON CONFLICT (name) DO NOTHING",
		consumer, consumer).Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO resources (id, name, version, consumer_name, deleted_at)
		VALUES (?, ?, 1, ?, ?)`, id, id, consumer, time.Now().Add(-age)).Error).To(Succeed())
}

func advanceRecoveryRound(conn *gorm.DB) {
	Expect(conn.Exec("UPDATE delete_recovery_schedule SET next_at = clock_timestamp() - interval '1 second'").Error).To(Succeed())
}

func makeRecoveryDue(conn *gorm.DB) {
	Expect(conn.Exec(`UPDATE resources SET delete_retry_at = clock_timestamp() - interval '1 second'
		WHERE delete_retry_at IS NOT NULL`).Error).To(Succeed())
	Expect(conn.Exec("UPDATE delete_recovery_consumers SET next_at = clock_timestamp() - interval '1 second'").Error).To(Succeed())
	advanceRecoveryRound(conn)
}

func TestDeleteRecoveryMigrationAndBoundedBootstrap(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	migrator := gormigrate.New(conn, gormigrate.DefaultOptions, migrations.MigrationList)
	t.Cleanup(func() { Expect(db.Migrate(conn)).To(Succeed()) })
	Expect(migrator.RollbackLast()).To(Succeed())
	for i := range 5 {
		recoveryTombstone(conn, fmt.Sprintf("legacy-%d", i), fmt.Sprintf("consumer-%d", i), time.Duration(10-i)*time.Hour)
	}
	// Legacy duplicate events must not prevent migration or cause a table-wide
	// deduplication. They are retired by the existing stale-event detector.
	Expect(conn.Exec(`INSERT INTO events (id, source, source_id, event_type, created_at)
		VALUES ('legacy-event-1', 'Resources', 'legacy-0', 'Delete', now() - interval '10 hours'),
		       ('legacy-event-2', 'Resources', 'legacy-0', 'Delete', now() - interval '10 hours')`).Error).To(Succeed())
	Expect(conn.Exec(`CREATE INDEX CONCURRENTLY idx_events_pending_resource_delete
		ON events (((id)::integer))`).Error).To(HaveOccurred())

	migrationResults := make(chan error, 2)
	for range 2 {
		go func() { migrationResults <- db.Migrate(h.DBFactory.New(ctx)) }()
	}
	for range 2 {
		Expect(<-migrationResults).To(Succeed())
	}
	var initialized int64
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE delete_retry_at IS NOT NULL").Scan(&initialized).Error).To(Succeed())
	Expect(initialized).To(BeZero())
	var indexes int64
	Expect(conn.Raw(`SELECT count(*) FROM pg_index JOIN pg_class ON indexrelid = pg_class.oid
		WHERE relname IN ('idx_resources_delete_recovery_bootstrap', 'idx_resources_delete_recovery_due',
			'idx_events_pending_resource_delete') AND indisvalid`).Scan(&indexes).Error).To(Succeed())
	Expect(indexes).To(Equal(int64(3)))

	recovery := recoveryForTest(t, h, 2)
	result, err := recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(Equal(2))
	Expect(result.Claimed).To(BeTrue())
	Expect(result.Published).To(Equal(1), "legacy outstanding events are not duplicated")
	var ids []string
	Expect(conn.Raw("SELECT id FROM resources WHERE delete_retry_at IS NOT NULL ORDER BY id").Scan(&ids).Error).To(Succeed())
	Expect(ids).To(Equal([]string{"legacy-0", "legacy-1"}), "bootstrap starts with the oldest tombstones")
	result, err = recoveryForTest(t, h, 2).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result).To(Equal(dao.DeleteRecoveryResult{}), "a second replica cannot take another immediate burst")
	advanceRecoveryRound(conn)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(Equal(2))

	Expect(gormigrate.New(conn, gormigrate.DefaultOptions, migrations.MigrationList).RollbackLast()).To(Succeed())
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE deleted_at IS NOT NULL").Scan(&initialized).Error).To(Succeed())
	Expect(initialized).To(Equal(int64(5)), "rollback must preserve tombstones")
	Expect(db.Migrate(conn)).To(Succeed())
}

func TestDeleteRecoveryEmptyAndBatchSize(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	for _, batch := range []int{config.NewEventServerConfig().DeleteEventRepublishBatchSize, 100} {
		recovery := recoveryForTest(t, h, batch)
		advanceRecoveryRound(conn)
		result, err := recovery.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(dao.DeleteRecoveryResult{Claimed: true}), "empty claimed rounds differ from unclaimed polls")
		for i := range batch + 1 {
			id := fmt.Sprintf("batch-%d-%03d", batch, i)
			recoveryTombstone(conn, id, id, time.Hour)
		}
		advanceRecoveryRound(conn)
		result, err = recovery.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(dao.DeleteRecoveryResult{Claimed: true, Initialized: batch, Consumers: batch, Published: batch}))
		Expect(conn.Exec("TRUNCATE events, resources, delete_recovery_consumers, consumers CASCADE").Error).To(Succeed())
	}
}

func TestDeleteRecoveryCommitFailure(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "commit-failure", "consumer", time.Hour)
	Expect(conn.Exec(`CREATE FUNCTION fail_recovery_commit_test() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected commit failure'; END $$;
		CREATE CONSTRAINT TRIGGER fail_recovery_commit_test AFTER INSERT ON events
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_recovery_commit_test()`).Error).To(Succeed())
	t.Cleanup(func() {
		if err := conn.Exec("DROP TRIGGER IF EXISTS fail_recovery_commit_test ON events; DROP FUNCTION IF EXISTS fail_recovery_commit_test()").Error; err != nil {
			t.Errorf("clean up recovery commit-failure trigger: %v", err)
		}
	})
	recovery := recoveryForTest(t, h, 25)
	result, err := recovery.Run(ctx)
	Expect(err).To(HaveOccurred())
	Expect(result).To(Equal(dao.DeleteRecoveryResult{}), "commit failure must not report committed work")
	var count int64
	Expect(conn.Raw("SELECT count(*) FROM events").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero())
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE delete_retry_at IS NOT NULL").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero())
	Expect(conn.Exec("DROP TRIGGER fail_recovery_commit_test ON events; DROP FUNCTION fail_recovery_commit_test()").Error).To(Succeed())
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result).To(Equal(dao.DeleteRecoveryResult{Claimed: true, Initialized: 1, Consumers: 1, Published: 1}),
		"commit failure must also roll back the fleet deadline")
}

func TestDeleteRecoveryConsumerFairnessAndOldestDue(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	for i := range 4 {
		recoveryTombstone(conn, fmt.Sprintf("busy-%d", i), "busy", time.Duration(10-i)*time.Hour)
	}
	recoveryTombstone(conn, "quiet-0", "quiet", time.Hour)
	recovery := recoveryForTest(t, h, 10)
	result, err := recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(Equal(5))
	Expect(result.Consumers).To(Equal(2))
	Expect(result.Published).To(Equal(2), "one resource per consumer, not one consumer taking the whole batch")
	var ids []string
	Expect(conn.Raw("SELECT source_id FROM events ORDER BY source_id").Scan(&ids).Error).To(Succeed())
	Expect(ids).To(Equal([]string{"busy-0", "quiet-0"}))

	advanceRecoveryRound(conn)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1))
	Expect(conn.Raw("SELECT source_id FROM events ORDER BY source_id").Scan(&ids).Error).To(Succeed())
	Expect(ids).To(Equal([]string{"busy-0", "busy-1", "quiet-0"}))

	// With a one-consumer budget, continuously due consumers rotate behind
	// consumers already waiting. Neither resource count nor lexical name wins.
	makeRecoveryDue(conn)
	Expect(conn.Exec("UPDATE delete_recovery_consumers SET next_at = '2000-01-01'").Error).To(Succeed())
	recovery = recoveryForTest(t, h, 1)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Consumers).To(Equal(1))
	var next string
	Expect(conn.Raw("SELECT consumer_name FROM delete_recovery_consumers ORDER BY next_at, consumer_name LIMIT 1").
		Scan(&next).Error).To(Succeed())
	Expect(next).To(Equal("quiet"))

	recoveryTombstone(conn, "newcomer", "aaa-newcomer", 365*24*time.Hour)
	advanceRecoveryRound(conn)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(Equal(1))
	Expect(result.Consumers).To(Equal(1))
	var count int64
	Expect(conn.Raw("SELECT count(*) FROM events WHERE source_id = 'newcomer'").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero(), "a newly initialized old tombstone cannot jump ahead of waiting consumers")
}

func TestDeleteRecoveryPersistentBackoffPurgeAndStaleRetirement(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "late", "connected-agent", 30*24*time.Hour)
	recovery := recoveryForTest(t, h, 10)
	events := dao.NewEventDao(&h.DBFactory)
	var previous time.Duration = time.Minute
	for range 8 {
		start := time.Now()
		result, err := recovery.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Published).To(Equal(1))
		var state struct {
			At    time.Time
			Delay time.Duration
		}
		Expect(conn.Raw("SELECT delete_retry_at AS at, delete_retry_delay AS delay FROM resources WHERE id = 'late'").
			Scan(&state).Error).To(Succeed())
		want := min(previous*2, time.Hour)
		Expect(state.Delay).To(Equal(want))
		Expect(state.At).To(BeTemporally(">=", start.Add(want/2-time.Second)))
		Expect(state.At).To(BeTemporally("<=", time.Now().Add(want)))
		previous = want

		pending, err := events.FindAllUnreconciledEvents(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(pending).To(HaveLen(1))
		now := time.Now()
		pending[0].ReconciledDate = &now
		_, err = events.Replace(ctx, pending[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(events.DeleteAllReconciledEvents(ctx)).To(Succeed())
		// An asynchronous controller finishing after purge must not recreate it.
		_, err = events.Replace(ctx, pending[0])
		Expect(err).NotTo(HaveOccurred())
		all, err := events.All(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(all).To(BeEmpty())
		advanceRecoveryRound(conn)
		result, err = recoveryForTest(t, h, 10).Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Published).To(BeZero(), "purge and restart do not reset resource backoff")
		makeRecoveryDue(conn)
	}
	result, err := recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1))
	makeRecoveryDue(conn)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(BeZero(), "outstanding events coalesce even when the resource is due")
	Expect(conn.Exec("UPDATE events SET created_at = now() - interval '2 hours'").Error).To(Succeed())
	retired, err := events.ReconcileStaleDeleteEvents(ctx, time.Now().Add(-time.Hour))
	Expect(err).NotTo(HaveOccurred())
	Expect(retired).To(Equal(int64(1)))
	makeRecoveryDue(conn)
	result, err = recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1), "retiring the only pending event must not stall recovery")
}

func TestDeleteRecoveryRollbackAndDisabled(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "rollback", "consumer", time.Hour)
	c := config.NewEventServerConfig()
	c.DeleteEventRepublishInterval = 0
	disabled, err := dao.NewDeleteRecovery(h.DBFactory, c)
	Expect(err).NotTo(HaveOccurred())
	result, err := disabled.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result).To(Equal(dao.DeleteRecoveryResult{}))
	Expect(conn.Exec(`INSERT INTO resources (id, name, version, consumer_name)
		VALUES ('initial-delete', 'initial-delete', 1, 'consumer')`).Error).To(Succeed())
	Expect(h.Env().Services.Resources().MarkAsDeleting(ctx, "initial-delete")).To(Succeed())
	var initialEvents int64
	Expect(conn.Raw("SELECT count(*) FROM events WHERE source_id = 'initial-delete'").
		Scan(&initialEvents).Error).To(Succeed())
	Expect(initialEvents).To(Equal(int64(1)), "disabling recovery does not gate initial deletion")

	Expect(conn.Exec(`CREATE FUNCTION fail_delete_recovery_test() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected event failure'; END $$;
		CREATE TRIGGER fail_delete_recovery_test BEFORE INSERT ON events
		FOR EACH ROW EXECUTE FUNCTION fail_delete_recovery_test()`).Error).To(Succeed())
	t.Cleanup(func() {
		conn.Exec("DROP TRIGGER IF EXISTS fail_delete_recovery_test ON events; DROP FUNCTION IF EXISTS fail_delete_recovery_test()")
	})
	result, err = recoveryForTest(t, h, 10).Run(ctx)
	Expect(err).To(HaveOccurred())
	Expect(result).To(Equal(dao.DeleteRecoveryResult{}))
	var count int64
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE delete_retry_at IS NOT NULL").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero(), "event failure must roll back bootstrap as well as backoff")
	Expect(conn.Raw("SELECT count(*) FROM delete_recovery_consumers").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero())
	Expect(conn.Exec("DROP TRIGGER fail_delete_recovery_test ON events; DROP FUNCTION fail_delete_recovery_test()").Error).To(Succeed())
	result, err = recoveryForTest(t, h, 10).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(Equal(1), "failed round does not consume the durable fleet deadline")
}

func TestDeleteRecoveryConcurrentReplicasAndDeleteCallers(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	for i := range 12 {
		recoveryTombstone(conn, fmt.Sprintf("concurrent-%d", i), fmt.Sprintf("consumer-%d", i), time.Hour)
	}
	var wg sync.WaitGroup
	results := make(chan dao.DeleteRecoveryResult, 12)
	errs := make(chan error, 36)
	start := make(chan struct{})
	for range 12 {
		wg.Go(func() {
			<-start
			result, err := recoveryForTest(t, h, 2).Run(ctx)
			results <- result
			errs <- err
		})
		wg.Go(func() {
			<-start
			// Exercise the same event DAO used for initial deletion alongside
			// the scheduler's coalescing transaction.
			_, err := dao.NewEventDao(&h.DBFactory).Create(ctx, &api.Event{
				Source: "Resources", SourceID: "concurrent-0", EventType: api.DeleteEventType,
			})
			errs <- err
		})
		wg.Go(func() {
			<-start
			svcErr := h.Env().Services.Resources().MarkAsDeleting(ctx, "concurrent-0")
			if svcErr != nil {
				errs <- svcErr
			} else {
				errs <- nil
			}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		Expect(err).NotTo(HaveOccurred())
	}
	total := dao.DeleteRecoveryResult{}
	for result := range results {
		total.Initialized += result.Initialized
		total.Published += result.Published
	}
	Expect(total.Initialized).To(BeNumerically(">=", 1))
	Expect(total.Initialized).To(BeNumerically("<=", 2), "locked candidates are skipped, not replaced")
	Expect(total.Published).To(BeNumerically("<=", 2))
	var count int64
	Expect(conn.Raw(`SELECT count(*) FROM events WHERE source_id = 'concurrent-0'
		AND event_type = 'Delete' AND reconciled_date IS NULL`).Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(1)))
}

func TestDeleteRecoveryAcknowledgementRaceAndCleanup(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "ack", "consumer", time.Hour)
	recovery := recoveryForTest(t, h, 10)
	_, err := recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	makeRecoveryDue(conn)

	// Hard deletion is the DAO operation used only after agent acknowledgement.
	// Concurrent publication and stale status writes must not resurrect the row.
	errs := make(chan error, 3)
	go func() { _, err := recovery.Run(ctx); errs <- err }()
	go func() { errs <- dao.NewResourceDao(&h.DBFactory).Delete(ctx, "ack", true) }()
	go func() {
		_, err := dao.NewResourceDao(&h.DBFactory).UpdateStatus(ctx, &api.Resource{Meta: api.Meta{ID: "ack"}})
		errs <- err
	}()
	for range 3 {
		Expect(<-errs).NotTo(HaveOccurred())
	}

	Expect(conn.Exec("DELETE FROM events").Error).To(Succeed())
	_, err = dao.NewEventDao(&h.DBFactory).Create(ctx, &api.Event{
		Source: "Resources", SourceID: "ack", EventType: api.DeleteEventType,
	})
	Expect(err).NotTo(HaveOccurred())
	makeRecoveryDue(conn)
	result, err := recovery.Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Published).To(BeZero())
	var count int64
	for _, table := range []string{"resources", "events", "delete_recovery_consumers"} {
		Expect(conn.Raw("SELECT count(*) FROM " + table).Scan(&count).Error).To(Succeed())
		Expect(count).To(BeZero(), table)
	}
}

func TestDeleteRecoveryConnectedAgentWithoutCallerRetries(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(time.Second)
	}()
	consumer, err := h.CreateConsumer("recovery-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, "nginx", "default", 1)
	Expect(err).NotTo(HaveOccurred())
	h.StartWorkAgent(ctx, consumer.Name)
	workClient := h.WorkAgentHolder.ManifestWorks(consumer.Name)
	Eventually(func() error {
		_, err := workClient.Get(ctx, resource.ID, metav1.GetOptions{})
		return err
	}, 20*time.Second, 100*time.Millisecond).Should(Succeed())

	Expect(h.Env().Services.Resources().MarkAsDeleting(ctx, resource.ID)).To(Succeed())
	conn := h.DBFactory.New(ctx)
	Expect(conn.Exec("UPDATE resources SET deleted_at = ? WHERE id = ?",
		time.Now().Add(-30*24*time.Hour), resource.ID).Error).To(Succeed())
	// Model deletion state absent on a continuously connected agent, with no
	// surviving event. No caller retry or reconnect follows this point.
	Expect(conn.Exec("UPDATE events SET reconciled_date = now() WHERE source_id = ?", resource.ID).Error).To(Succeed())
	Expect(h.Env().Services.Events().DeleteAllReconciledEvents(ctx)).To(Succeed())
	work, err := workClient.Get(ctx, resource.ID, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(work.DeletionTimestamp.IsZero()).To(BeTrue())

	controllerServer := server.NewControllersServer(ctx, h.EventServer, h.EventFilter)
	Expect(controllerServer.DeleteRecovery).NotTo(BeNil())
	go controllerServer.Start(ctx)
	Eventually(func() bool {
		_, svcErr := h.Env().Services.Resources().Get(ctx, resource.ID)
		return svcErr != nil && svcErr.Is404()
	}, 30*time.Second, 100*time.Millisecond).Should(BeTrue(),
		"scheduled recovery must reach the connected agent and receive its delete acknowledgement")
	var count int64
	Expect(conn.Raw("SELECT count(*) FROM status_events WHERE resource_id = ? AND status_event_type = ?",
		resource.ID, api.StatusDeleteEventType).Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(1)))
}

func TestDeleteRecoveryLocksDoNotGateInitialDeletion(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "locked-oldest", "consumer", 2*time.Hour)
	recoveryTombstone(conn, "next-candidate", "consumer", time.Hour)
	holder := conn.Begin()
	Expect(holder.Error).NotTo(HaveOccurred())
	defer holder.Rollback()
	Expect(holder.Exec("SELECT id FROM resources WHERE id = 'locked-oldest' FOR UPDATE").Error).To(Succeed())
	result, err := recoveryForTest(t, h, 1).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(BeZero(), "the locked candidate does not cause an unbounded refill scan")
	Expect(holder.Rollback().Error).To(Succeed())

	holder = conn.Begin()
	Expect(holder.Error).NotTo(HaveOccurred())
	defer holder.Rollback()
	Expect(holder.Exec("SELECT id FROM delete_recovery_schedule WHERE id = 1 FOR UPDATE").Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO resources (id, name, version, consumer_name)
		VALUES ('initial', 'initial', 1, 'consumer')`).Error).To(Succeed())
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := h.Env().Services.Resources().MarkAsDeleting(ctx, "initial"); err != nil {
			done <- err
			return
		}
		done <- nil
	}()
	var deleteErr error
	Eventually(done, 2*time.Second).Should(Receive(&deleteErr))
	Expect(deleteErr).NotTo(HaveOccurred(), "initial deletion must not take the fleet lock")
}

func TestDeleteRecoverySchedulerIndexes(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	conn := h.DBFactory.New(context.Background())
	recoveryTombstone(conn, "index-seed", "index-consumer", time.Hour)
	Expect(conn.Exec(`INSERT INTO resources (id, name, version, consumer_name, deleted_at, delete_retry_at)
		SELECT 'plan-' || i, 'plan-' || i, 1, 'index-consumer', now() - interval '2 hours',
			CASE WHEN i % 2 = 0 THEN now() - interval '1 hour' END
		FROM generate_series(1, 2000) i`).Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO events (id, source, source_id, event_type, reconciled_date)
		SELECT 'plan-event-' || i, 'Resources', 'index-seed', 'Delete',
			CASE WHEN i > 1 THEN now() END FROM generate_series(1, 2000) i`).Error).To(Succeed())
	Expect(conn.Exec("ANALYZE resources; ANALYZE events").Error).To(Succeed())
	for _, query := range []struct{ sql, index string }{
		{`SELECT id FROM resources WHERE deleted_at IS NOT NULL AND delete_retry_at IS NULL
			ORDER BY deleted_at, id LIMIT 2`, "idx_resources_delete_recovery_bootstrap"},
		{`SELECT id FROM resources WHERE consumer_name = 'index-consumer'
			AND deleted_at IS NOT NULL AND delete_retry_at IS NOT NULL AND delete_retry_at <= now()
			ORDER BY delete_retry_at, id LIMIT 1`, "idx_resources_delete_recovery_due"},
		{`SELECT 1 FROM events WHERE source = 'Resources' AND event_type = 'Delete'
			AND reconciled_date IS NULL AND source_id = 'index-seed'`, "idx_events_pending_resource_delete"},
	} {
		var plan []string
		Expect(conn.Raw("EXPLAIN " + query.sql).Scan(&plan).Error).To(Succeed())
		Expect(strings.Join(plan, "\n")).To(ContainSubstring(query.index))
	}
}
