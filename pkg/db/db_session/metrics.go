package db_session

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Notify channel capacity in github.com/lib/pq
	notifyChannelCapacity = 32
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

	notifyChannelFullCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "db",
			Name:      "notify_channel_full_total",
			Help:      "Total number of times the NOTIFY channel buffer reached capacity (32/32)",
		},
		[]string{"channel"},
	)
)

func init() {
	prometheus.MustRegister(notifyChannelDepthGauge)
	prometheus.MustRegister(notifyChannelFullCounter)
}
