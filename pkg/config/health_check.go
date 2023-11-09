package config

import (
	"github.com/spf13/pflag"
)

type HealthCheckConfig struct {
	BindAddress string `json:"bind_address"`
	EnableHTTPS bool   `json:"enable_https"`
}

func NewHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		BindAddress: "localhost:8083",
		EnableHTTPS: false,
	}
}

func (c *HealthCheckConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.BindAddress, "health-check-server-bindaddress", c.BindAddress, "Health check server bind adddress")
	fs.BoolVar(&c.EnableHTTPS, "enable-health-check-https", c.EnableHTTPS, "Enable HTTPS for health check server")
}

func (c *HealthCheckConfig) ReadFiles() error {
	return nil
}
