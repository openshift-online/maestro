package db_session

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Notify channel capacity in github.com/lib/pq
	notifyChannelCapacity = 32
	// Threshold for alerting (80% full)
	notifyChannelThreshold = 25
)

// Metric for PostgreSQL NOTIFY channel buffer depth
var (
	notifyChannelDepthGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "db",
			Name:      "notify_channel_depth",
			Help:      "Current depth of the PostgreSQL NOTIFY channel buffer (capacity: 32)",
		},
		[]string{"channel"},
	)

	notifyChannelNearFullCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "db",
			Name:      "notify_channel_near_full_total",
			Help:      "Total number of times the NOTIFY channel buffer exceeded threshold (>25/32)",
		},
		[]string{"channel"},
	)
)

func init() {
	prometheus.MustRegister(notifyChannelDepthGauge)
	prometheus.MustRegister(notifyChannelNearFullCounter)
}
