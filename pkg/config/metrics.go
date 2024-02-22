package config

import (
	"time"

	"github.com/spf13/pflag"
)

type MetricsConfig struct {
	BindPort                      string        `json:"bind_port"`
	EnableHTTPS                   bool          `json:"enable_https"`
	LabelMetricsInclusionDuration time.Duration `json:"label_metrics_inclusion_duration"`
}

func NewMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		BindPort:                      "8080",
		EnableHTTPS:                   false,
		LabelMetricsInclusionDuration: 7 * 24 * time.Hour,
	}
}

func (s *MetricsConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindPort, "metrics-server-bindport", s.BindPort, "Metrics server bind port")
	fs.BoolVar(&s.EnableHTTPS, "enable-metrics-https", s.EnableHTTPS, "Enable HTTPS for metrics server")
	fs.DurationVar(&s.LabelMetricsInclusionDuration, "label-metrics-inclusion-duration", 7*24*time.Hour, "A cluster's last telemetry date needs be within in this duration in order to have labels collected")
}

func (s *MetricsConfig) ReadFiles() error {
	return nil
}
