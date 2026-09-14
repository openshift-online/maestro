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
					if len(family.Metric) != 4 {
						t.Fatal("outcomes must have exactly four bounded label values")
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
	c := NewDeleteRecoveryController(recoveryRunnerFunc(func(context.Context) (dao.DeleteRecoveryResult, error) {
		return dao.DeleteRecoveryResult{}, errors.New("constraint failed: consumer_name=customer-value")
	}))
	c.Run(klog.NewContext(context.Background(), logger))
	if !strings.Contains(output.String(), "Delete recovery round failed") {
		t.Fatal("a failed round must emit a generic failure log")
	}
	if strings.Contains(output.String(), "constraint failed") || strings.Contains(output.String(), "customer-value") {
		t.Fatal("recovery logs must not contain database error details")
	}
}
