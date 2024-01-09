package config

import (
	"github.com/spf13/pflag"
)

type DispatcherConfig struct {
	EnableDispatcher bool  `json:"enable_dispatcher"`
	PulseInterval    int64 `json:"pulse_interval"`
	CheckInterval    int64 `json:"check_interval"`
}

func MewDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		EnableDispatcher: false,
		PulseInterval:    10,
		CheckInterval:    10,
	}
}

// AddFlags configures the DispatcherConfig with command line flags.
// It allows users to customize the timing parameters for maestro instance pulses and checks.
// - "pulse-interval" sets the time between maestro instance pulses (in seconds) to indicate liveness (default: 10 seconds).
// - "check-interval" determines how often health checks are performed on maestro instances (in seconds) to maintain the instance list (default: 10 seconds).
// Instances not pulsing within the last three check intervals are considered as dead.
func (c *DispatcherConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.EnableDispatcher, "enable-dispatcher", c.EnableDispatcher, "Enable the resource status updates dispatcher for coordinating updates across multiple Maestro instances.")
	fs.Int64Var(&c.PulseInterval, "pulse-interval", c.PulseInterval, "Set the pulse interval for maestro instances (seconds) to indicate liveness (default: 10 seconds)")
	fs.Int64Var(&c.CheckInterval, "check-interval", c.CheckInterval, "Set the interval for health checks on maestro instances (seconds) to maintain the active instance list (default: 10 seconds)")
}

func (c *DispatcherConfig) ReadFiles() error {
	return nil
}
