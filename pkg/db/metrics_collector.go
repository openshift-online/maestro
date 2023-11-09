package db

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollector interface {
}

// Subsystem used to define the metrics:
const metricsSubsystem = "advisory_lock"

// Names of the labels added to metrics:
const (
	metricsTypeLabel   = "type"
	metricsStatusLabel = "status"
)

// MetricsLabels - Array of labels added to metrics:
var MetricsLabels = []string{
	metricsTypeLabel,
	metricsStatusLabel,
}

// Names of the metrics:
const (
	countMetric    = "count"
	durationMetric = "duration"
)

// MetricsNames - Array of Names of the metrics:
var MetricsNames = []string{
	countMetric,
	durationMetric,
}

// Description of the requests count metric:
var advisoryLockCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: metricsSubsystem,
		Name:      countMetric,
		Help:      "Number of advisory lock requests.",
	},
	MetricsLabels,
)

// Description of the request duration metric:
var advisoryLockDurationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: metricsSubsystem,
		Name:      durationMetric,
		Help:      "Advisory Lock durations in seconds.",
		Buckets: []float64{
			0.1,
			0.2,
			0.5,
			1.0,
			2.0,
			10.0,
		},
	},
	MetricsLabels,
)

// Register the metrics:
func RegisterAdvisoryLockMetrics() {
	prometheus.MustRegister(advisoryLockCountMetric)
	prometheus.MustRegister(advisoryLockDurationMetric)
}

// Unregister the metrics:
func UnregisterAdvisoryLockMetrics() {
	prometheus.Unregister(advisoryLockCountMetric)
	prometheus.Unregister(advisoryLockDurationMetric)
}

// ResetMetricCollectors resets all collectors
func ResetAdvisoryLockMetricsCollectors() {
	advisoryLockCountMetric.Reset()
	advisoryLockDurationMetric.Reset()
}

func UpdateAdvisoryLockCountMetric(lockType LockType, status string) {
	labels := prometheus.Labels{
		metricsTypeLabel:   string(lockType),
		metricsStatusLabel: status,
	}
	advisoryLockCountMetric.With(labels).Inc()
}

func UpdateAdvisoryLockDurationMetric(lockType LockType, status string, startTime time.Time) {
	labels := prometheus.Labels{
		metricsTypeLabel:   string(lockType),
		metricsStatusLabel: status,
	}
	duration := time.Since(startTime)
	advisoryLockDurationMetric.With(labels).Observe(duration.Seconds())
}
