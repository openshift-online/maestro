package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/openshift-online/maestro/test"
)

func TestNotifyChannelMetrics(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to collect notification IDs
	received := make(chan string, 100)

	// Start listener with callback that collects notifications
	listener := h.Env().Database.SessionFactory.NewListener(ctx, "metrics_test_channel", func(id string) {
		received <- id
		// Introduce a small delay to allow buffer to fill up
		time.Sleep(10 * time.Millisecond)
	})
	defer listener.Close()

	// Wait for listener to be ready
	time.Sleep(100 * time.Millisecond)

	g2 := h.Env().Database.SessionFactory.New(ctx)

	// Test 1: Send notifications that should NOT exceed threshold
	t.Run("LowDepthDoesNotTriggerAlert", func(t *testing.T) {
		// Get initial counter value
		initialNearFull := getCounterValue(t, "db_notify_channel_near_full_total", map[string]string{"channel": "metrics_test_channel"})

		// Send a few notifications (well below threshold of 25)
		for i := 0; i < 5; i++ {
			if err := g2.Exec("SELECT pg_notify('metrics_test_channel', $1)", i).Error; err != nil {
				t.Fatalf("Failed to send notification: %v", err)
			}
		}

		// Wait for processing
		time.Sleep(500 * time.Millisecond)

		// Verify depth gauge is recorded (should be low)
		depth := getGaugeValue(t, "db_notify_channel_depth", map[string]string{"channel": "metrics_test_channel"})
		if depth > 5 {
			t.Errorf("Expected depth <= 5, got %f", depth)
		}

		// Verify near_full counter did NOT increment
		finalNearFull := getCounterValue(t, "db_notify_channel_near_full_total", map[string]string{"channel": "metrics_test_channel"})
		if finalNearFull != initialNearFull {
			t.Errorf("Expected near_full counter to remain at %f, but got %f", initialNearFull, finalNearFull)
		}
	})

	// Test 2: Send many notifications rapidly to exceed threshold
	t.Run("HighDepthTriggersAlert", func(t *testing.T) {
		// Get initial counter value
		initialNearFull := getCounterValue(t, "db_notify_channel_near_full_total", map[string]string{"channel": "metrics_test_channel"})

		// Send 30 notifications rapidly (more than threshold of 25)
		// The slow callback should cause buffer to fill up
		for i := 0; i < 30; i++ {
			if err := g2.Exec("SELECT pg_notify('metrics_test_channel', $1)", i).Error; err != nil {
				t.Fatalf("Failed to send notification: %v", err)
			}
		}

		// Give time for at least one notification to be processed (which records metrics)
		time.Sleep(100 * time.Millisecond)

		// Verify depth gauge shows high value
		depth := getGaugeValue(t, "db_notify_channel_depth", map[string]string{"channel": "metrics_test_channel"})
		t.Logf("Buffer depth: %f", depth)

		// Verify near_full counter incremented (at least once)
		Eventually(func() bool {
			finalNearFull := getCounterValue(t, "db_notify_channel_near_full_total", map[string]string{"channel": "metrics_test_channel"})
			return finalNearFull > initialNearFull
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue(), "Expected near_full counter to increment when buffer is saturated")
	})

	// Clean up: drain remaining notifications
drainLoop:
	for {
		select {
		case <-received:
		case <-time.After(100 * time.Millisecond):
			break drainLoop
		}
	}
}

// getGaugeValue retrieves the current value of a gauge metric
func getGaugeValue(t *testing.T, metricName string, labels map[string]string) float64 {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if labelsMatch(m.GetLabel(), labels) {
					return m.GetGauge().GetValue()
				}
			}
		}
	}

	// Return 0 if metric not found (hasn't been recorded yet)
	return 0
}

// getCounterValue retrieves the current value of a counter metric
func getCounterValue(t *testing.T, metricName string, labels map[string]string) float64 {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if labelsMatch(m.GetLabel(), labels) {
					return m.GetCounter().GetValue()
				}
			}
		}
	}

	// Return 0 if metric not found (hasn't been recorded yet)
	return 0
}

// labelsMatch checks if metric labels match the expected labels
func labelsMatch(metricLabels []*dto.LabelPair, expectedLabels map[string]string) bool {
	if len(metricLabels) != len(expectedLabels) {
		return false
	}

	for _, label := range metricLabels {
		expectedValue, exists := expectedLabels[label.GetName()]
		if !exists || expectedValue != label.GetValue() {
			return false
		}
	}

	return true
}
