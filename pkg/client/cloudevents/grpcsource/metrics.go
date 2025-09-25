package grpcsource

import "github.com/prometheus/client_golang/prometheus"

func init() {
	// Register the metrics:
	prometheus.MustRegister(sourceClientRegisteredWatchersGaugeMetric)
}

// Description of the source client registered watchers gauge metric:
var sourceClientRegisteredWatchersGaugeMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Subsystem: "source_client",
		Name:      "registered_watchers",
		Help:      "Number of registered watchers for a source client.",
	},
	[]string{"source", "namespace"},
)

func ResetsourceClientRegisteredWatchersGaugeMetric() {
	sourceClientRegisteredWatchersGaugeMetric.Reset()
}
