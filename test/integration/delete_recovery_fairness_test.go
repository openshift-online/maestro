package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/test"
)

// TestDeleteRecoveryContinuousBootstrapFairness checks that a busy consumer's
// fresh initialization cannot displace older due consumers, including tied ones.
func TestDeleteRecoveryContinuousBootstrapFairness(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	for i := range 8 {
		recoveryTombstone(conn, fmt.Sprintf("busy-%02d", i), "aaa-busy", 24*time.Hour)
	}
	for i := range 4 {
		id := fmt.Sprintf("waiting-%d", i)
		recoveryTombstone(conn, id, id, time.Hour)
		Expect(conn.Exec(`UPDATE resources SET delete_retry_at = '2000-01-01', delete_retry_delay = ?
			WHERE id = ?`, int64(time.Minute), id).Error).To(Succeed())
		Expect(conn.Exec(`INSERT INTO delete_recovery_consumers (consumer_name, next_at)
			VALUES (?, '2000-01-01')`, id).Error).To(Succeed())
	}
	recovery := recoveryForTest(t, h, 1)
	for i := range 4 {
		advanceRecoveryRound(conn)
		result, err := recovery.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Initialized).To(Equal(1), "every round adds another busy-consumer tombstone")
		Expect(result.Published).To(Equal(1))
		var source string
		Expect(conn.Raw("SELECT source_id FROM events ORDER BY created_at DESC, id DESC LIMIT 1").Scan(&source).Error).To(Succeed())
		Expect(source).To(Equal(fmt.Sprintf("waiting-%d", i)), "bootstrap must not jump the queue")
	}
	for range 8 {
		advanceRecoveryRound(conn)
		result, err := recovery.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Published).To(Equal(1))
	}
	var events, unscheduled int64
	Expect(conn.Raw("SELECT count(*) FROM events").Scan(&events).Error).To(Succeed())
	Expect(events).To(Equal(int64(12)), "every resource in the finite backlog gets a turn")
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE delete_retry_at IS NULL").Scan(&unscheduled).Error).To(Succeed())
	Expect(unscheduled).To(BeZero())
}

// TestDeleteRecoveryBootstrapShortensFutureDeadline prevents a queued consumer's
// capped backoff from delaying freshly initialized work that is already due.
func TestDeleteRecoveryBootstrapShortensFutureDeadline(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	conn := h.DBFactory.New(ctx)
	recoveryTombstone(conn, "backoff", "consumer", time.Hour)
	Expect(conn.Exec(`UPDATE resources SET delete_retry_at = clock_timestamp() + interval '1 hour',
		delete_retry_delay = ? WHERE id = 'backoff'`, int64(time.Hour)).Error).To(Succeed())
	Expect(conn.Exec(`INSERT INTO delete_recovery_consumers (consumer_name, next_at)
		VALUES ('consumer', clock_timestamp() + interval '1 hour')`).Error).To(Succeed())
	recoveryTombstone(conn, "fresh", "consumer", time.Hour)
	result, err := recoveryForTest(t, h, 1).Run(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Initialized).To(Equal(1))
	Expect(result.Published).To(Equal(1))
	var source string
	Expect(conn.Raw("SELECT source_id FROM events").Scan(&source).Error).To(Succeed())
	Expect(source).To(Equal("fresh"), "ON CONFLICT DO NOTHING would defer this due work for an hour")
}
