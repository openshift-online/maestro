package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

// TestDeleteRecoveryStuckTombstones counts deletion age independently of event age,
// consumer connectivity and recovery scheduling state, without reading live rows.
func TestDeleteRecoveryStuckTombstones(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	for _, fixture := range []struct {
		id  string
		age time.Duration
	}{
		{"old-unscheduled", 2 * time.Hour},
		{"old-due", 3 * time.Hour},
		{"old-delayed", 4 * time.Hour},
		{"fresh-unscheduled", 5 * time.Minute},
		{"fresh-due", 5 * time.Minute},
		{"fresh-delayed", 5 * time.Minute},
		{"future-deletion", -time.Hour},
	} {
		recoveryTombstone(conn, fixture.id, "stuck-metrics-consumer", fixture.age)
	}
	Expect(conn.Exec(`UPDATE resources SET delete_retry_at = clock_timestamp() - interval '1 minute'
		WHERE id IN ('old-due', 'fresh-due')`).Error).To(Succeed())
	Expect(conn.Exec(`UPDATE resources SET delete_retry_at = clock_timestamp() + interval '1 hour'
		WHERE id IN ('old-delayed', 'fresh-delayed')`).Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO resources (id, name, version, consumer_name, created_at)
		VALUES ('live', 'live', 1, 'stuck-metrics-consumer', clock_timestamp() - interval '1 day')`).Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO events (id, source, source_id, event_type, created_at)
		VALUES ('fresh-event-old-resource', 'Resources', 'old-unscheduled', 'Delete', clock_timestamp()),
		       ('old-event-fresh-resource', 'Resources', 'fresh-due', 'Delete', clock_timestamp() - interval '1 day'),
		       ('old-event-live-resource', 'Resources', 'live', 'Delete', clock_timestamp() - interval '1 day')`).Error).To(Succeed())

	for _, tc := range []struct {
		name             string
		thresholdSeconds int
		disableRecovery  bool
		want             int64
	}{
		{"default threshold", 3600, false, 3},
		{"custom threshold", 9000, false, 2},
		{"no old tombstones", 18000, false, 0},
		{"short threshold", 60, false, 6},
		{"threshold disabled", 0, false, 0},
		{"recovery disabled", 3600, true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			settings := config.NewEventServerConfig()
			settings.StaleDeleteEventThreshold = tc.thresholdSeconds
			if tc.disableRecovery {
				settings.DeleteEventRepublishInterval = 0
			}
			recovery, err := dao.NewDeleteRecovery(h.DBFactory, settings)
			g.Expect(err).NotTo(HaveOccurred())
			snapshot, err := recovery.Snapshot(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(snapshot.StuckTombstones).To(Equal(tc.want))
			g.Expect(snapshot.UnscheduledTombstones).To(Equal(int64(3)))
			g.Expect(snapshot.DueTombstones).To(Equal(int64(2)))
			g.Expect(snapshot.DelayedTombstones).To(Equal(int64(2)))
			g.Expect(snapshot.OldestDelayedAgeSeconds).To(BeNumerically(">=", 4*time.Hour.Seconds()))

			// Exercise the process-wide registration as exposed by /metrics, not
			// just the collector in a private unit-test registry.
			controllers.NewDeleteRecoveryController(recovery).Report(ctx)
			families, err := prometheus.DefaultGatherer.Gather()
			g.Expect(err).NotTo(HaveOccurred())
			found := false
			for _, family := range families {
				if family.GetName() != "delete_recovery_stuck_tombstones" {
					continue
				}
				found = true
				g.Expect(family.Metric).To(HaveLen(1))
				g.Expect(family.Metric[0].Label).To(BeEmpty())
				g.Expect(family.Metric[0].GetGauge().GetValue()).To(Equal(float64(tc.want)))
			}
			g.Expect(found).To(BeTrue(), "stuck-tombstone gauge must be registered")
		})
	}

	// Republishing changes event age, not the resource's original deletion age.
	Expect(conn.Exec("UPDATE events SET created_at = clock_timestamp()").Error).To(Succeed())
	snapshot, err := recoveryForTest(t, h, 1).Snapshot(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshot.StuckTombstones).To(Equal(int64(3)))
}
