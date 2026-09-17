package dao

import (
	"math"
	"strings"
	"testing"
	"time"
)

// TestDeleteRecoveryBackoff checks exponential caps, overflow safety and equal-jitter bounds.
func TestDeleteRecoveryBackoff(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		previous, initial, cap, want time.Duration
	}{
		{"first", time.Minute, time.Minute, time.Hour, 2 * time.Minute},
		{"progress", 2 * time.Minute, time.Minute, time.Hour, 4 * time.Minute},
		{"cap", 40 * time.Minute, time.Minute, time.Hour, time.Hour},
		{"changed config", time.Hour, time.Minute, 10 * time.Minute, 10 * time.Minute},
		{"null bootstrap", 0, time.Minute, time.Hour, 2 * time.Minute},
		{"overflow", math.MaxInt64 - 1, time.Second, math.MaxInt64, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := nextDeleteRetryDelay(tc.previous, tc.initial, tc.cap)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for range 100 {
				delay := jitterDeleteRetryDelay(got)
				if delay < got/2 || delay >= got {
					t.Fatalf("jitter %v outside [%v, %v)", delay, got/2, got)
				}
			}
		})
	}
}

// TestDeleteRecoverySnapshotQuery checks the reporter reads aggregate durable state only.
func TestDeleteRecoverySnapshotQuery(t *testing.T) {
	for _, fragment := range []string{
		"clock_timestamp()",
		"unscheduled_tombstones",
		"due_tombstones",
		"delayed_tombstones",
		"due_consumers",
		"scheduled_consumers",
		"pending_delete_events",
		"oldest_pending_event_age_seconds",
	} {
		if !strings.Contains(deleteRecoverySnapshotSQL, fragment) {
			t.Fatalf("snapshot query is missing %q", fragment)
		}
	}
	for _, identifier := range []string{"SELECT id", "SELECT consumer_name", "SELECT source_id"} {
		if strings.Contains(deleteRecoverySnapshotSQL, identifier) {
			t.Fatalf("snapshot query must not return %s", identifier)
		}
	}
}
