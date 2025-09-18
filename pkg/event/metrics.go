package event

import "github.com/prometheus/client_golang/prometheus"

func init() {
	// Register the metrics:
	prometheus.MustRegister(grpcRegisteredSourceClientsGaugeMetric)
}

// Description of the gRPC registered source clients gauge metric:
var grpcRegisteredSourceClientsGaugeMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Subsystem: "grpc_server",
		Name:      "registered_source_clients",
		Help:      "Number of registered source clients on the grpc server.",
	},
	[]string{"source"},
)
