package db_session

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNotifyChannelMetrics(t *testing.T) {
	// Reset metrics before test
	notifyChannelDepthGauge.Reset()
	notifyChannelNearFullCounter.Reset()

	testChannel := "test_channel"

	t.Run("DepthGaugeRecordsValue", func(t *testing.T) {
		// Set depth to 10
		notifyChannelDepthGauge.WithLabelValues(testChannel).Set(10)

		// Verify the gauge value
		value := getGaugeValue(t, "db_notify_channel_depth", testChannel)
		if value != 10 {
			t.Errorf("Expected depth gauge value 10, got %f", value)
		}
	})

	t.Run("NearFullCounterIncrementsAboveThreshold", func(t *testing.T) {
		// Reset counter
		notifyChannelNearFullCounter.Reset()

		// Simulate exceeding threshold (>25)
		notifyChannelNearFullCounter.WithLabelValues(testChannel).Inc()

		// Verify counter incremented
		value := getCounterValue(t, "db_notify_channel_near_full_total", testChannel)
		if value != 1 {
			t.Errorf("Expected counter value 1, got %f", value)
		}

		// Increment again
		notifyChannelNearFullCounter.WithLabelValues(testChannel).Inc()

		// Verify counter is now 2
		value = getCounterValue(t, "db_notify_channel_near_full_total", testChannel)
		if value != 2 {
			t.Errorf("Expected counter value 2, got %f", value)
		}
	})

	t.Run("ThresholdConstants", func(t *testing.T) {
		// Verify the constants are set correctly
		if notifyChannelCapacity != 32 {
			t.Errorf("Expected capacity 32, got %d", notifyChannelCapacity)
		}

		if notifyChannelThreshold != 25 {
			t.Errorf("Expected threshold 25 (78%% of 32), got %d", notifyChannelThreshold)
		}

		// Verify threshold is reasonable (between 50% and 90% of capacity)
		thresholdPercent := float64(notifyChannelThreshold) / float64(notifyChannelCapacity) * 100
		if thresholdPercent < 50 || thresholdPercent > 90 {
			t.Errorf("Threshold should be 50-90%% of capacity, got %.1f%%", thresholdPercent)
		}
	})

	t.Run("MultipleChannels", func(t *testing.T) {
		// Reset metrics
		notifyChannelDepthGauge.Reset()
		notifyChannelNearFullCounter.Reset()

		// Set values for different channels
		notifyChannelDepthGauge.WithLabelValues("events").Set(5)
		notifyChannelDepthGauge.WithLabelValues("status_events").Set(28)

		notifyChannelNearFullCounter.WithLabelValues("status_events").Inc()

		// Verify each channel has independent values
		eventsDepth := getGaugeValue(t, "db_notify_channel_depth", "events")
		statusDepth := getGaugeValue(t, "db_notify_channel_depth", "status_events")

		if eventsDepth != 5 {
			t.Errorf("Expected events depth 5, got %f", eventsDepth)
		}

		if statusDepth != 28 {
			t.Errorf("Expected status_events depth 28, got %f", statusDepth)
		}

		// Verify counter only incremented for status_events
		eventsCounter := getCounterValue(t, "db_notify_channel_near_full_total", "events")
		statusCounter := getCounterValue(t, "db_notify_channel_near_full_total", "status_events")

		if eventsCounter != 0 {
			t.Errorf("Expected events counter 0, got %f", eventsCounter)
		}

		if statusCounter != 1 {
			t.Errorf("Expected status_events counter 1, got %f", statusCounter)
		}
	})
}

func getGaugeValue(t *testing.T, metricName, channel string) float64 {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if matchesChannel(m.GetLabel(), channel) {
					return m.GetGauge().GetValue()
				}
			}
		}
	}

	return 0
}

func getCounterValue(t *testing.T, metricName, channel string) float64 {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if matchesChannel(m.GetLabel(), channel) {
					return m.GetCounter().GetValue()
				}
			}
		}
	}

	return 0
}

func matchesChannel(labels []*dto.LabelPair, channel string) bool {
	for _, label := range labels {
		if label.GetName() == "channel" && label.GetValue() == channel {
			return true
		}
	}
	return false
}
