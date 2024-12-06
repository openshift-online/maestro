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
	PulseInterval        int64                 `json:"pulse_interval"`
	SubscriptionType     string                `json:"subscription_type"`
	ConsistentHashConfig *ConsistentHashConfig `json:"consistent_hash_config"`
}

// ConsistentHashConfig contains the configuration for the consistent hashing algorithm.
type ConsistentHashConfig struct {
	PartitionCount    int     `json:"partition_count"`
	ReplicationFactor int     `json:"replication_factor"`
	Load              float64 `json:"load"`
}

// NewPulseServerConfig creates a new PulseServerConfig with default 15 second pulse interval.
func NewPulseServerConfig() *PulseServerConfig {
	return &PulseServerConfig{
		PulseInterval:        15,
		SubscriptionType:     "shared",
		ConsistentHashConfig: NewConsistentHashConfig(),
	}
}

// NewConsistentHashConfig creates a new ConsistentHashConfig with default values.
//   - PartitionCount: 7
//   - ReplicationFactor: 20
//   - Load: 1.25
func NewConsistentHashConfig() *ConsistentHashConfig {
	return &ConsistentHashConfig{
		PartitionCount:    7,
		ReplicationFactor: 20,
		Load:              1.25,
	}
}

// AddFlags configures the PulseServerConfig with command line flags.
// It allows users to customize the interval for maestro instance pulses and subscription type.
//   - "pulse-interval" sets the time between maestro instance pulses (in seconds) to indicate its liveness (default: 15 seconds).
//   - "subscription-type" specifies the subscription type for resource status updates from message broker, either "shared" or "broadcast".
//     "shared" subscription type uses MQTT feature to ensure only one Maestro instance receives resource status messages.
//     "broadcast" subscription type will make all Maestro instances to receive resource status messages and hash the message to determine which instance should process it.
//     If subscription type is "broadcast", ConsistentHashConfig settings can be configured for the hashing algorithm.
func (c *PulseServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.Int64Var(&c.PulseInterval, "pulse-interval", c.PulseInterval, "Sets the pulse interval for maestro instances (seconds) to indicate liveness")
	fs.StringVar(&c.SubscriptionType, "subscription-type", c.SubscriptionType, "Sets the subscription type for resource status updates from message broker, Options: \"shared\" (only one instance receives resource status message, MQTT feature ensures exclusivity) or \"broadcast\" (all instances receive messages, hashed to determine processing instance)")
	c.ConsistentHashConfig.AddFlags(fs)
}

func (c *PulseServerConfig) ReadFiles() error {
	c.ConsistentHashConfig.ReadFiles()
	return nil
}

// AddFlags configures the ConsistentHashConfig with command line flags. Only take effect when subscription type is "broadcast".
// It allows users to customize the partition count, replication factor, and load for the consistent hashing algorithm.
func (c *ConsistentHashConfig) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.PartitionCount, "consistent-hash-partition-count", c.PartitionCount, "Sets the partition count for consistent hashing algorithm, select a big PartitionCount for more consumers. only take effect when subscription type is \"broadcast\"")
	fs.IntVar(&c.ReplicationFactor, "consistent-hash-replication-factor", c.ReplicationFactor, "Sets the replication factor for maestro instances to be replicated on consistent hash ring. only take effect when subscription type is \"broadcast\"")
	fs.Float64Var(&c.Load, "consistent-hash-load", c.Load, "Sets the load for consistent hashing algorithm, only take effect when subscription type is \"broadcast\"")
}

func (c *ConsistentHashConfig) ReadFiles() error {
	return nil
}
