package db

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	// Register the metrics for advisory locks
	RegisterAdvisoryLockMetrics()
}

type MetricsCollector interface {
}

// Subsystem used to define the metrics:
const lockMetricsSubsystem = "advisory_lock"
const unlockMetricsSubsystem = "advisory_unlock"

// Names of the labels added to metrics:
const (
	metricsTypeLabel   = "type"
	metricsStatusLabel = "status"
)

// metricsLabels - Array of labels added to metrics:
var metricsLabels = []string{
	metricsTypeLabel,
	metricsStatusLabel,
}

// Names of the metrics:
const (
	countMetric    = "count"
	durationMetric = "duration"
)

// Description of the lock requests count metric:
var advisoryLockCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: lockMetricsSubsystem,
		Name:      countMetric,
		Help:      "Number of advisory lock requests.",
	},
	metricsLabels,
)

// Description of the unlock requests count metric:
var advisoryUnlockCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: unlockMetricsSubsystem,
		Name:      countMetric,
		Help:      "Number of advisory unlock requests.",
	},
	metricsLabels,
)

// Description of the request duration metric:
var advisoryLockDurationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: lockMetricsSubsystem,
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
	metricsLabels,
)

// Register the metrics:
func RegisterAdvisoryLockMetrics() {
	prometheus.MustRegister(advisoryLockCountMetric)
	prometheus.MustRegister(advisoryUnlockCountMetric)
	prometheus.MustRegister(advisoryLockDurationMetric)
}

func UpdateAdvisoryLockCountMetric(lockType LockType, status string) {
	labels := prometheus.Labels{
		metricsTypeLabel:   string(lockType),
		metricsStatusLabel: status,
	}
	advisoryLockCountMetric.With(labels).Inc()
}

func UpdateAdvisoryUnlockCountMetric(lockType LockType, status string) {
	labels := prometheus.Labels{
		metricsTypeLabel:   string(lockType),
		metricsStatusLabel: status,
	}
	advisoryUnlockCountMetric.With(labels).Inc()
}

func UpdateAdvisoryLockDurationMetric(lockType LockType, status string, startTime time.Time) {
	labels := prometheus.Labels{
		metricsTypeLabel:   string(lockType),
		metricsStatusLabel: status,
	}
	duration := time.Since(startTime)
	advisoryLockDurationMetric.With(labels).Observe(duration.Seconds())
}
