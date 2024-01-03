package config

import (
	"github.com/spf13/pflag"
)

type DispatcherConfig struct {
	PulseInterval            int64 `json:"pulse_interval"`
	CheckInterval            int64 `json:"check_interval"`
	InstanceExpirationPeriod int64 `json:"instance_expiration_period"`
}

func MewDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		PulseInterval:            10,
		CheckInterval:            10,
		InstanceExpirationPeriod: 30,
	}
}

func (c *DispatcherConfig) AddFlags(fs *pflag.FlagSet) {
	fs.Int64Var(&c.PulseInterval, "pulse-interval", c.PulseInterval, "Interval in seconds between pulses")
	fs.Int64Var(&c.CheckInterval, "check-interval", c.CheckInterval, "Interval in seconds between checks")
	fs.Int64Var(&c.InstanceExpirationPeriod, "instance-expiration-period", c.InstanceExpirationPeriod, "Maximum time in seconds between pulses before an instance is considered dead")
}

func (c *DispatcherConfig) ReadFiles() error {
	return nil
}
