package config

import (
	"github.com/spf13/pflag"
)

// PulseServerConfig contains the configuration for the maestro pulse server.
type PulseServerConfig struct {
	PulseInterval           int64 `json:"pulse_interval"`
	CheckInterval           int64 `json:"check_interval"`
	EnableConsistentHashing bool  `json:"enable_consistent_hashing"`
}

// NewPulseServerConfig creates a new PulseServerConfig with default 10 second pulse and check intervals.
func MewPulseServerConfig() *PulseServerConfig {
	return &PulseServerConfig{
		PulseInterval:           10,
		CheckInterval:           10,
		EnableConsistentHashing: false,
	}
}

// AddFlags configures the PulseServerConfig with command line flags.
// It allows users to customize the timing parameters for maestro instance pulses and checks.
// - "pulse-interval" sets the time between maestro instance pulses (in seconds) to indicate its liveness (default: 10 seconds).
// - "check-interval" determines how often health checks are performed on maestro instances (in seconds) to maintain the active instance list (default: 10 seconds).
// Instances not pulsing within the last three check intervals are considered as dead.
// - "enable-consistent-hashing" enables consistent hashing for resource status update distribution among running maestro instances (default: false).
func (c *PulseServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.Int64Var(&c.PulseInterval, "pulse-interval", c.PulseInterval, "Set the pulse interval for maestro instances (seconds) to indicate liveness (default: 10 seconds)")
	fs.Int64Var(&c.CheckInterval, "check-interval", c.CheckInterval, "Set the interval for health checks on maestro instances (seconds) to maintain the active instance list (default: 10 seconds)")
	fs.BoolVar(&c.EnableConsistentHashing, "enable-consistent-hashing", c.EnableConsistentHashing, "Enable consistent hashing for resource status update distribution among running maestro instances (default: false)")
}

func (c *PulseServerConfig) ReadFiles() error {
	return nil
}
