package config

import (
	"github.com/spf13/pflag"
)

type HealthCheckConfig struct {
	BindPort           string `json:"bind_port"`
	EnableHTTPS        bool   `json:"enable_https"`
	HeartbeartInterval int    `json:"heartbeat_interval"`
}

func NewHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		BindPort:           "8083",
		EnableHTTPS:        false,
		HeartbeartInterval: 15,
	}
}

func (c *HealthCheckConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.BindPort, "health-check-server-bindport", c.BindPort, "Health check server bind port")
	fs.BoolVar(&c.EnableHTTPS, "enable-health-check-https", c.EnableHTTPS, "Enable HTTPS for health check server")
	fs.IntVar(&c.HeartbeartInterval, "heartbeat-interval", c.HeartbeartInterval, "Heartbeat interval for health check server")
}

func (c *HealthCheckConfig) ReadFiles() error {
	return nil
}
