package config

import (
	"github.com/spf13/pflag"
)

type SubscriptionType string

const (
	SharedSubscriptionType    SubscriptionType = "shared"
	BroadcastSubscriptionType SubscriptionType = "broadcast"
)

// PulseServerConfig contains the configuration for the maestro pulse server.
type PulseServerConfig struct {
	PulseInterval    int64  `json:"pulse_interval"`
	SubscriptionType string `json:"subscription_type"`
}

// NewPulseServerConfig creates a new PulseServerConfig with default 15 second pulse interval.
func MewPulseServerConfig() *PulseServerConfig {
	return &PulseServerConfig{
		PulseInterval:    15,
		SubscriptionType: "shared",
	}
}

// AddFlags configures the PulseServerConfig with command line flags.
// It allows users to customize the interval for maestro instance pulses and subscription type.
//   - "pulse-interval" sets the time between maestro instance pulses (in seconds) to indicate its liveness (default: 15 seconds).
//   - "subscription-type" specifies the subscription type for resource status updates from message broker, either "shared" or "broadcast".
//     "shared" subscription type uses MQTT feature to ensure only one Maestro instance receives resource status messages.
//     "broadcast" subscription type will make all Maestro instances to receive resource status messages and hash the message to determine which instance should process it.
func (c *PulseServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.Int64Var(&c.PulseInterval, "pulse-interval", c.PulseInterval, "Sets the pulse interval for maestro instances (seconds) to indicate liveness (default: 10 seconds)")
	fs.StringVar(&c.SubscriptionType, "subscription-type", c.SubscriptionType, "Sets the subscription type for resource status updates from message broker, Options: \"shared\" (only one instance receives resource status message, MQTT feature ensures exclusivity) or \"broadcast\" (all instances receive messages, hashed to determine processing instance) (default: \"shared\")")
}

func (c *PulseServerConfig) ReadFiles() error {
	return nil
}
