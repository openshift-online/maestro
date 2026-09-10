package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/db/migrations"
	"github.com/openshift-online/maestro/test"
)

func TestResourceLabelsIndexMigration(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	conn := h.DBFactory.New(h.Ctx)
	migrator := gormigrate.New(conn, gormigrate.DefaultOptions, migrations.MigrationList)
	Expect(migrator.RollbackTo("202412181141")).To(Succeed())
	_, err := h.CreateConsumer("consumer")
	Expect(err).NotTo(HaveOccurred())

	Expect(conn.Exec(`INSERT INTO resources
		(id, name, version, source, consumer_name, deleted_at, payload)
		SELECT i::text, 'resource-' || i, 1, 'source', 'consumer',
			CASE WHEN i = 1 THEN now() END,
			jsonb_build_object('metadata', jsonb_build_object('labels',
				jsonb_build_object('cluster', 'cluster-' || (i % 256), 'type', 'bundle')),
				'spec', repeat('x', 1024))
		FROM generate_series(1, 1024) AS i`).Error).To(Succeed())

	const query = `SELECT id FROM resources
		WHERE source = 'source' AND consumer_name = 'consumer'
			AND payload -> 'metadata' -> 'labels' @> '{"cluster":"cluster-1","type":"bundle"}'
		ORDER BY id LIMIT 400`
	var before []string
	Expect(conn.Raw(query).Scan(&before).Error).To(Succeed())
	Expect(before).To(Equal([]string{"1", "257", "513", "769"}))

	// A failed concurrent build leaves an invalid index with the reserved name.
	Expect(conn.Exec(`CREATE INDEX CONCURRENTLY idx_resources_payload_labels ON resources
		(((payload -> 'metadata' -> 'labels' ->> 'cluster')::integer))`).Error).To(HaveOccurred())
	var valid bool
	Expect(conn.Raw(`SELECT indisvalid FROM pg_index
		WHERE indexrelid = 'idx_resources_payload_labels'::regclass`).Scan(&valid).Error).To(Succeed())
	Expect(valid).To(BeFalse())

	// Replica init containers must not race the index DDL or migration history.
	//
	// Bound each attempt: CREATE INDEX CONCURRENTLY can take much longer than usual under
	// a slow or contended CI runner. Without a deadline, a slow build here leaks a
	// goroutine that keeps running db.Migrate to completion long after Eventually gives up
	// on this test, still holding the migrations advisory lock, and blocks every later
	// test in the suite that calls db.Migrate as part of its own setup. Cancelling cleanly
	// releases the lock (see TestResourceLabelsMigrationCancelledWhileWaiting), containing
	// a slow environment to a failure of this test alone.
	buildCtx, cancelBuild := context.WithTimeout(h.Ctx, 90*time.Second)
	defer cancelBuild()
	results := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- db.Migrate(h.DBFactory.New(buildCtx))
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		var migrationErr error
		Eventually(results, "95s").Should(Receive(&migrationErr))
		Expect(migrationErr).NotTo(HaveOccurred())
	}
	Expect(db.Migrate(conn)).To(Succeed())

	Expect(conn.Raw(`SELECT indisvalid FROM pg_index
		WHERE indexrelid = 'idx_resources_payload_labels'::regclass`).Scan(&valid).Error).To(Succeed())
	Expect(valid).To(BeTrue())
	var definition string
	Expect(conn.Raw(`SELECT pg_get_indexdef('idx_resources_payload_labels'::regclass)`).
		Scan(&definition).Error).To(Succeed())
	Expect(definition).To(ContainSubstring("USING gin"))
	Expect(definition).To(ContainSubstring("jsonb_path_ops"))

	Expect(conn.Exec("ANALYZE resources").Error).To(Succeed())
	var plan []string
	Expect(conn.Raw("EXPLAIN " + query).Scan(&plan).Error).To(Succeed())
	Expect(strings.Join(plan, "\n")).To(ContainSubstring("Bitmap Index Scan on idx_resources_payload_labels"))

	var after []string
	Expect(conn.Raw(query).Scan(&after).Error).To(Succeed())
	Expect(after).To(Equal(before))

	Expect(gormigrate.New(conn, gormigrate.DefaultOptions, migrations.MigrationList).
		RollbackTo("202412181141")).To(Succeed())
	var indexCount int64
	Expect(conn.Raw(`SELECT count(*) FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = 'idx_resources_payload_labels'`).
		Scan(&indexCount).Error).To(Succeed())
	Expect(indexCount).To(BeZero())
	Expect(conn.Raw(query).Scan(&after).Error).To(Succeed())
	Expect(after).To(Equal(before))

	// Rebuilds the index (CREATE INDEX CONCURRENTLY again); bound it for the same reason
	// as the first build above.
	rebuildCtx, cancelRebuild := context.WithTimeout(h.Ctx, 90*time.Second)
	defer cancelRebuild()
	Expect(db.Migrate(conn.WithContext(rebuildCtx))).To(Succeed())
}

func TestResourceLabelsMigrationCancelledWhileWaiting(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	conn := h.DBFactory.New(h.Ctx)
	sqlDB, err := conn.DB()
	Expect(err).NotTo(HaveOccurred())
	holder, err := sqlDB.Conn(h.Ctx)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext('maestro'), hashtext('migrations'))"); err != nil {
			t.Errorf("release migration lock: %v", err)
		}
		if err := holder.Close(); err != nil {
			t.Errorf("close migration lock connection: %v", err)
		}
	})

	_, err = holder.ExecContext(h.Ctx, "SELECT pg_advisory_lock(hashtext('maestro'), hashtext('migrations'))")
	Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithTimeout(h.Ctx, 100*time.Millisecond)
	defer cancel()
	Expect(errors.Is(db.Migrate(conn.WithContext(ctx)), context.DeadlineExceeded)).To(BeTrue())

	_, err = holder.ExecContext(h.Ctx, "SELECT pg_advisory_unlock(hashtext('maestro'), hashtext('migrations'))")
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Migrate(conn)).To(Succeed())
}
