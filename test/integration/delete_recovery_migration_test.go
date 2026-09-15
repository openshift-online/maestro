package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	. "github.com/onsi/gomega"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/db/migrations"
	"github.com/openshift-online/maestro/test"
)

// recoveryMigrationDB isolates session settings and detects nested pool acquisition.
func recoveryMigrationDB(t *testing.T, h *test.Helper) *gorm.DB {
	t.Helper()
	cfg := h.AppConfig.Database
	pgConfig, err := pgx.ParseConfig(cfg.ConnectionString(cfg.SSLMode != "disable"))
	Expect(err).NotTo(HaveOccurred())
	pgConfig.Password = cfg.Password
	pool := stdlib.OpenDB(*pgConfig)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { Expect(pool.Close()).To(Succeed()) })
	conn, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{
		Logger: h.DBFactory.New(h.Ctx).Logger,
	})
	Expect(err).NotTo(HaveOccurred())
	return conn
}

// runConcurrentRecoveryMigrations proves both production migrators wait before DDL.
func runConcurrentRecoveryMigrations(t *testing.T, h *test.Helper) {
	t.Helper()
	ctx, cancel := context.WithTimeout(h.Ctx, 30*time.Second)
	defer cancel()
	observer := h.DBFactory.New(ctx)
	pool, err := observer.DB()
	Expect(err).NotTo(HaveOccurred())
	holder, err := pool.Conn(ctx)
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, err := holder.ExecContext(cleanupCtx, "SELECT pg_advisory_unlock(hashtext('maestro'), hashtext('migrations'))")
		Expect(err).NotTo(HaveOccurred())
		Expect(holder.Close()).To(Succeed())
	}()
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_lock(hashtext('maestro'), hashtext('migrations'))")
	Expect(err).NotTo(HaveOccurred())
	results := make(chan error, 2)
	for range 2 {
		migrator := recoveryMigrationDB(t, h).WithContext(ctx)
		var pid int
		Expect(migrator.Raw("SELECT pg_backend_pid()").Scan(&pid).Error).To(Succeed())
		go func() { results <- db.Migrate(migrator) }()
		Eventually(func() (bool, error) {
			var waiting bool
			err := observer.Raw(`SELECT query LIKE '%pg_try_advisory_lock%' FROM pg_stat_activity
				WHERE pid = ?`, pid).Scan(&waiting).Error
			return waiting, err
		}, "5s", "10ms").Should(BeTrue())
	}
	var history int64
	Expect(observer.Raw("SELECT count(*) FROM migrations WHERE id = '202609141000'").Scan(&history).Error).To(Succeed())
	Expect(history).To(BeZero())
	var columns int64
	Expect(observer.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'resources' AND column_name = 'delete_retry_at'`).Scan(&columns).Error).To(Succeed())
	Expect(columns).To(BeZero(), "neither migrator may run DDL before acquiring the advisory lock")
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext('maestro'), hashtext('migrations'))")
	Expect(err).NotTo(HaveOccurred())
	for range 2 {
		var migrationErr error
		Eventually(results, "20s").Should(Receive(&migrationErr))
		Expect(migrationErr).NotTo(HaveOccurred())
	}
	Expect(observer.Raw("SELECT count(*) FROM migrations WHERE id = '202609141000'").Scan(&history).Error).To(Succeed())
	Expect(history).To(Equal(int64(1)))
}

// TestDeleteRecoveryMigrationLockTimeout exercises real concurrent DDL waits and cleanup.
func TestDeleteRecoveryMigrationLockTimeout(t *testing.T) {
	for _, scenario := range []string{"success", "drop", "create", "old-snapshot", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			h, _ := test.RegisterIntegration(t)
			ctx, cancel := context.WithTimeout(h.Ctx, 30*time.Second)
			defer cancel()
			observer := h.DBFactory.New(ctx)
			Expect(gormigrate.New(observer, gormigrate.DefaultOptions, migrations.MigrationList).RollbackLast()).To(Succeed())
			if scenario == "drop" {
				Expect(observer.Exec("CREATE INDEX idx_events_pending_resource_delete ON events (source_id)").Error).To(Succeed())
			}
			migrator := recoveryMigrationDB(t, h).WithContext(ctx)
			Expect(migrator.Exec("SELECT set_config('lock_timeout', ?, false)", "17s").Error).To(Succeed())
			var originalPID int
			Expect(migrator.Raw("SELECT pg_backend_pid()").Scan(&originalPID).Error).To(Succeed())

			var blocker *sql.Tx
			if scenario != "success" {
				pool, err := observer.DB()
				Expect(err).NotTo(HaveOccurred())
				blocker, err = pool.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					if err := blocker.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
						t.Errorf("release migration blocker: %v", err)
					}
				}()
				if scenario == "old-snapshot" {
					_, err = blocker.ExecContext(ctx, "SELECT count(*) FROM pg_class")
				} else {
					_, err = blocker.ExecContext(ctx, "LOCK TABLE events IN SHARE UPDATE EXCLUSIVE MODE")
				}
				Expect(err).NotTo(HaveOccurred())
			}
			migrationCtx, migrationCancel := context.WithCancel(ctx)
			defer migrationCancel()
			results := make(chan error, 1)
			started := time.Now()
			go func() { results <- db.Migrate(migrator.WithContext(migrationCtx)) }()
			if scenario != "success" {
				query := "CREATE INDEX CONCURRENTLY idx_events_pending_resource_delete%"
				if scenario == "drop" {
					query = "DROP INDEX CONCURRENTLY IF EXISTS idx_events_pending_resource_delete%"
				} else if scenario == "old-snapshot" {
					query = "CREATE INDEX CONCURRENTLY idx_resources_delete_recovery_bootstrap%"
				}
				Eventually(func() (bool, error) {
					var waiting bool
					err := observer.Raw(`SELECT wait_event_type = 'Lock' AND query LIKE ?
						FROM pg_stat_activity WHERE pid = ?`, query, originalPID).Scan(&waiting).Error
					return waiting, err
				}, "4s", "10ms").Should(BeTrue())
				if scenario == "old-snapshot" {
					var phase string
					Expect(observer.Raw("SELECT phase FROM pg_stat_progress_create_index WHERE pid = ?", originalPID).Scan(&phase).Error).To(Succeed())
					Expect(phase).To(Equal("waiting for old snapshots"))
				}
				if scenario == "cancel" {
					migrationCancel()
				}
			}
			var migrationErr error
			Eventually(results, "12s").Should(Receive(&migrationErr))
			t.Logf("%s migration finished in %s: %v", scenario, time.Since(started), migrationErr)
			if scenario == "success" {
				Expect(migrationErr).NotTo(HaveOccurred())
			} else {
				Expect(migrationErr).To(HaveOccurred())
				if scenario != "cancel" {
					var pgErr *pgconn.PgError
					Expect(errors.As(migrationErr, &pgErr)).To(BeTrue())
					Expect(pgErr.Code).To(Equal("55P03"), "lock_timeout, not a total statement deadline")
					Expect(time.Since(started)).To(BeNumerically(">=", 5*time.Second))
				} else {
					Expect(errors.Is(migrationErr, context.Canceled)).To(BeTrue())
				}
				var history int64
				Expect(observer.Raw("SELECT count(*) FROM migrations WHERE id = '202609141000'").Scan(&history).Error).To(Succeed())
				Expect(history).To(BeZero(), "failed migrations must not be recorded")
				if scenario == "old-snapshot" {
					var valid bool
					Expect(observer.Raw(`SELECT indisvalid FROM pg_index
						WHERE indexrelid = 'idx_resources_delete_recovery_bootstrap'::regclass`).Scan(&valid).Error).To(Succeed())
					Expect(valid).To(BeFalse(), "an interrupted build must be repaired on retry")
				}
				Expect(blocker.Rollback()).To(Succeed())
			}

			var setting string
			var currentPID int
			Expect(migrator.Raw("SHOW lock_timeout").Scan(&setting).Error).To(Succeed())
			Expect(migrator.Raw("SELECT pg_backend_pid()").Scan(&currentPID).Error).To(Succeed())
			if currentPID == originalPID {
				Expect(setting).To(Equal("17s"), "restore the exact original session setting")
			} else {
				Expect(scenario).To(Equal("cancel"), "only cancellation may discard this session")
				Expect(setting).To(Equal("0"), "replacement sessions must not inherit the migration timeout")
				Eventually(func() (int64, error) {
					var alive int64
					err := observer.Raw("SELECT count(*) FROM pg_stat_activity WHERE pid = ?", originalPID).Scan(&alive).Error
					return alive, err
				}, "5s").Should(BeZero())
			}
			var locks int64
			Expect(observer.Raw("SELECT count(*) FROM pg_locks WHERE pid = ? AND locktype = 'advisory'", currentPID).Scan(&locks).Error).To(Succeed())
			Expect(locks).To(BeZero())
			Expect(db.Migrate(migrator)).To(Succeed(), "retry must repair interrupted concurrent indexes")
			var valid int64
			Expect(observer.Raw(`SELECT count(*) FROM pg_index JOIN pg_class ON indexrelid = pg_class.oid
				WHERE relname IN ('idx_resources_delete_recovery_bootstrap', 'idx_resources_delete_recovery_due',
					'idx_events_pending_resource_delete') AND indisvalid`).Scan(&valid).Error).To(Succeed())
			Expect(valid).To(Equal(int64(3)))
		})
	}
}

// TestDeleteRecoveryMigrationDirectSession covers callers without the production wrapper.
func TestDeleteRecoveryMigrationDirectSession(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(h.Ctx, 30*time.Second)
	defer cancel()
	conn := recoveryMigrationDB(t, h).WithContext(ctx)
	migrator := gormigrate.New(conn, gormigrate.DefaultOptions, migrations.MigrationList)
	Expect(migrator.RollbackLast()).To(Succeed())
	Expect(conn.Exec("SELECT set_config('lock_timeout', ?, false)", "23s").Error).To(Succeed())
	Expect(migrator.Migrate()).To(Succeed())
	var setting string
	Expect(conn.Raw("SHOW lock_timeout").Scan(&setting).Error).To(Succeed())
	Expect(setting).To(Equal("23s"))
}
