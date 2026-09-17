package controllers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/dao"
)

type recoveryRunnerFunc func(context.Context) (dao.DeleteRecoveryResult, error)

// Run delegates to the test callback so controller outcomes can be injected.
func (f recoveryRunnerFunc) Run(ctx context.Context) (dao.DeleteRecoveryResult, error) {
	return f(ctx)
}

type recoverySnapshotFunc func(context.Context) (dao.DeleteRecoverySnapshot, error)

// Snapshot delegates to the test callback so durable metric snapshots can be injected.
func (f recoverySnapshotFunc) Snapshot(ctx context.Context) (dao.DeleteRecoverySnapshot, error) {
	return f(ctx)
}

type recoveryWithSnapshot struct {
	recoveryRunnerFunc
	recoverySnapshotFunc
}

// TestDeleteRecoveryMetrics checks bounded outcome labels, elapsed time and committed-only work counts.
func TestDeleteRecoveryMetrics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		result  dao.DeleteRecoveryResult
		err     error
		outcome string
	}{
		{"published", dao.DeleteRecoveryResult{Claimed: true, Initialized: 2, Consumers: 3, Published: 1}, nil, "committed"},
		{"pending", dao.DeleteRecoveryResult{Claimed: true, Consumers: 3}, nil, "committed"},
		{"bootstrap", dao.DeleteRecoveryResult{Claimed: true, Initialized: 2}, nil, "committed"},
		{"empty claimed round", dao.DeleteRecoveryResult{Claimed: true}, nil, "empty"},
		{"cooldown handshake", dao.DeleteRecoveryResult{Cooldown: true}, nil, "cooldown"},
		{"failed cooldown", dao.DeleteRecoveryResult{Cooldown: true}, errors.New("commit failed"), "error"},
		{"unclaimed poll", dao.DeleteRecoveryResult{}, nil, "noop"},
		{"database error", dao.DeleteRecoveryResult{}, errors.New("database unavailable"), "error"},
		{"rolled back work", dao.DeleteRecoveryResult{Claimed: true, Initialized: 2, Consumers: 3, Published: 1}, errors.New("commit failed"), "error"},
		{"cancelled", dao.DeleteRecoveryResult{}, context.Canceled, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newDeleteRecoveryMetrics()
			registry := prometheus.NewRegistry()
			registry.MustRegister(m.duration, m.work)
			var elapsed time.Duration
			c := NewDeleteRecoveryController(recoveryRunnerFunc(func(context.Context) (dao.DeleteRecoveryResult, error) {
				start := time.Now()
				time.Sleep(time.Millisecond)
				elapsed = time.Since(start)
				return tc.result, tc.err
			}))
			c.metrics = m
			c.Run(context.Background())
			families, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			if len(families) != 2 {
				t.Fatalf("metric families = %d, want 2", len(families))
			}
			for _, family := range families {
				switch family.GetName() {
				case "delete_recovery_round_duration_seconds":
					if len(family.Metric) != 5 {
						t.Fatal("outcomes must have exactly five bounded label values")
					}
					for _, metric := range family.Metric {
						if len(metric.Label) != 1 || metric.Label[0].GetName() != "outcome" {
							t.Fatal("only the outcome label is allowed")
						}
						want := uint64(0)
						if metric.Label[0].GetValue() == tc.outcome {
							want = 1
							if metric.Histogram.GetSampleSum() < elapsed.Seconds() {
								t.Fatal("duration must include the entire runner call")
							}
						}
						if metric.Histogram.GetSampleCount() != want {
							t.Fatalf("outcome %s count = %d, want %d", metric.Label[0].GetValue(), metric.Histogram.GetSampleCount(), want)
						}
					}
				case "delete_recovery_work_total":
					if len(family.Metric) != 3 {
						t.Fatal("work must have exactly three bounded label values")
					}
					want := map[string]int{"initialized": 0, "consumers": 0, "published": 0}
					if tc.err == nil && tc.result.Claimed {
						want = map[string]int{"initialized": tc.result.Initialized, "consumers": tc.result.Consumers, "published": tc.result.Published}
					}
					for _, metric := range family.Metric {
						if len(metric.Label) != 1 || metric.Label[0].GetName() != "kind" {
							t.Fatal("only the kind label is allowed")
						}
						if metric.Counter.GetValue() != float64(want[metric.Label[0].GetValue()]) {
							t.Fatal("work counters must include committed work only")
						}
					}
				default:
					t.Fatalf("unexpected metric %s", family.GetName())
				}
			}
		})
	}
}

// TestDeleteRecoveryController verifies the runner receives a bounded, cancellable context.
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

// TestDeleteRecoveryErrorLogging checks database error details never reach controller logs.
func TestDeleteRecoveryErrorLogging(t *testing.T) {
	var output strings.Builder
	logger := funcr.New(func(prefix, args string) {
		output.WriteString(prefix)
		output.WriteString(args)
	}, funcr.Options{})
	c := NewDeleteRecoveryController(recoveryWithSnapshot{
		recoveryRunnerFunc: func(context.Context) (dao.DeleteRecoveryResult, error) {
			return dao.DeleteRecoveryResult{}, errors.New("constraint failed: consumer_name=customer-value")
		},
		recoverySnapshotFunc: func(context.Context) (dao.DeleteRecoverySnapshot, error) {
			return dao.DeleteRecoverySnapshot{}, errors.New("constraint failed: consumer_name=customer-value")
		},
	})
	ctx := klog.NewContext(context.Background(), logger)
	c.Run(ctx)
	c.Report(ctx)
	if !strings.Contains(output.String(), "Delete recovery round failed") {
		t.Fatal("a failed round must emit a generic failure log")
	}
	if !strings.Contains(output.String(), "Delete recovery metrics snapshot failed") {
		t.Fatal("a failed snapshot must emit a generic failure log")
	}
	if strings.Contains(output.String(), "constraint failed") || strings.Contains(output.String(), "customer-value") {
		t.Fatal("recovery logs must not contain database error details")
	}
}

// TestDeleteRecoverySnapshotMetrics checks every fixed state is refreshed, including drained states.
func TestDeleteRecoverySnapshotMetrics(t *testing.T) {
	snapshot := dao.DeleteRecoverySnapshot{
		UnscheduledTombstones:        3,
		DueTombstones:                2,
		DelayedTombstones:            1,
		OldestUnscheduledAgeSeconds:  300,
		OldestDueAgeSeconds:          200,
		OldestDelayedAgeSeconds:      100,
		DueConsumers:                 2,
		ScheduledConsumers:           1,
		PendingDeleteEvents:          4,
		OldestPendingEventAgeSeconds: 400,
	}
	reporter := recoveryWithSnapshot{
		recoveryRunnerFunc: func(context.Context) (dao.DeleteRecoveryResult, error) {
			return dao.DeleteRecoveryResult{}, nil
		},
		recoverySnapshotFunc: func(context.Context) (dao.DeleteRecoverySnapshot, error) {
			return snapshot, nil
		},
	}
	m := newDeleteRecoveryMetrics()
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		m.tombstoneBacklog,
		m.tombstoneOldestAge,
		m.consumerQueue,
		m.pendingDeleteEvents,
		m.pendingDeleteEventAge,
		m.snapshotErrors,
	)
	c := NewDeleteRecoveryController(reporter)
	c.metrics = m
	c.Report(context.Background())

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 6 {
		t.Fatalf("snapshot metric families = %d, want 6", len(families))
	}
	want := map[string]map[string]float64{
		"delete_recovery_tombstone_backlog": {
			"state=unscheduled": 3, "state=due": 2, "state=delayed": 1,
		},
		"delete_recovery_tombstone_oldest_age_seconds": {
			"state=unscheduled": 300, "state=due": 200, "state=delayed": 100,
		},
		"delete_recovery_consumer_queue": {
			"state=due": 2, "state=scheduled": 1,
		},
		"delete_recovery_pending_delete_events": {
			"": 4,
		},
		"delete_recovery_pending_delete_event_oldest_age_seconds": {
			"": 400,
		},
		"delete_recovery_snapshot_errors_total": {
			"": 0,
		},
	}
	for _, family := range families {
		values, ok := want[family.GetName()]
		if !ok {
			t.Fatalf("unexpected metric family %s", family.GetName())
		}
		if len(family.Metric) != len(values) {
			t.Fatalf("%s cardinality = %d, want %d", family.GetName(), len(family.Metric), len(values))
		}
		for _, metric := range family.Metric {
			labels := ""
			for _, label := range metric.Label {
				labels += label.GetName() + "=" + label.GetValue()
			}
			value := metric.GetGauge().GetValue()
			if metric.Counter != nil {
				value = metric.GetCounter().GetValue()
			}
			wantValue, ok := values[labels]
			if !ok {
				t.Fatalf("unexpected %s{%s}", family.GetName(), labels)
			}
			if value != wantValue {
				t.Fatalf("%s{%s} = %v, want %v", family.GetName(), labels, value, wantValue)
			}
		}
	}

	snapshot = dao.DeleteRecoverySnapshot{}
	c.Report(context.Background())
	families, err = registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "delete_recovery_tombstone_backlog" {
			for _, metric := range family.Metric {
				if metric.GetGauge().GetValue() != 0 {
					t.Fatal("a drained scheduler state must be reset to zero")
				}
			}
		}
	}
}

// TestDeleteRecoverySnapshotErrorRetainsMetrics checks a failed snapshot is safe and observable.
func TestDeleteRecoverySnapshotErrorRetainsMetrics(t *testing.T) {
	var fail bool
	reporter := recoveryWithSnapshot{
		recoveryRunnerFunc: func(context.Context) (dao.DeleteRecoveryResult, error) {
			return dao.DeleteRecoveryResult{}, nil
		},
		recoverySnapshotFunc: func(context.Context) (dao.DeleteRecoverySnapshot, error) {
			if fail {
				return dao.DeleteRecoverySnapshot{}, errors.New("database failed: consumer_name=customer-value")
			}
			return dao.DeleteRecoverySnapshot{DueTombstones: 7}, nil
		},
	}
	m := newDeleteRecoveryMetrics()
	registry := prometheus.NewRegistry()
	registry.MustRegister(m.tombstoneBacklog, m.tombstoneOldestAge, m.consumerQueue, m.pendingDeleteEvents, m.pendingDeleteEventAge, m.snapshotErrors)
	c := NewDeleteRecoveryController(reporter)
	c.metrics = m
	c.Report(context.Background())
	fail = true
	c.Report(context.Background())

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		switch family.GetName() {
		case "delete_recovery_tombstone_backlog":
			for _, metric := range family.Metric {
				if metric.Label[0].GetValue() == "due" && metric.GetGauge().GetValue() != 7 {
					t.Fatal("a failed snapshot must retain the last good values")
				}
			}
		case "delete_recovery_snapshot_errors_total":
			if family.Metric[0].GetCounter().GetValue() != 1 {
				t.Fatal("a failed snapshot must increment the bounded error counter")
			}
		}
	}
}
