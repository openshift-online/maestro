package config

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/pflag"
)

type SubscriptionType string

const (
	SharedSubscriptionType    SubscriptionType = "shared"
	BroadcastSubscriptionType SubscriptionType = "broadcast"
)

// EventServerConfig contains the configuration for the message queue event server.
type EventServerConfig struct {
	SubscriptionType                string                `json:"subscription_type"`
	ConsistentHashConfig            *ConsistentHashConfig `json:"consistent_hash_config"`
	UndeliveredResourceThreshold    int                   `json:"undelivered_resource_threshold"`
	StaleDeleteEventThreshold       int                   `json:"stale_delete_event_threshold"`
	DeleteEventRepublishInterval    int                   `json:"delete_event_republish_interval"`
	DeleteEventRepublishMaxInterval int                   `json:"delete_event_republish_max_interval"`
	DeleteEventRepublishBatchSize   int                   `json:"delete_event_republish_batch_size"`
}

// ConsistentHashConfig contains the configuration for the consistent hashing algorithm.
type ConsistentHashConfig struct {
	PartitionCount    int     `json:"partition_count"`
	ReplicationFactor int     `json:"replication_factor"`
	Load              float64 `json:"load"`
}

// NewEventServerConfig creates a new EventServerConfig with default settings.
func NewEventServerConfig() *EventServerConfig {
	return &EventServerConfig{
		SubscriptionType:                "shared",
		ConsistentHashConfig:            NewConsistentHashConfig(),
		UndeliveredResourceThreshold:    600,
		StaleDeleteEventThreshold:       3600,
		DeleteEventRepublishInterval:    60,
		DeleteEventRepublishMaxInterval: 3600,
		DeleteEventRepublishBatchSize:   100,
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

// AddFlags configures the EventServerConfig with command line flags.
// It allows users to customize the subscription type and ConsistentHashConfig settings.
//   - "subscription-type" specifies the subscription type for resource status updates from message broker, either "shared" or "broadcast".
//     "shared" subscription type uses MQTT feature to ensure only one Maestro instance receives resource status messages.
//     "broadcast" subscription type will make all Maestro instances to receive resource status messages and hash the message to determine which instance should process it.
//     If subscription type is "broadcast", ConsistentHashConfig settings can be configured for the hashing algorithm.
func (c *EventServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.SubscriptionType, "subscription-type", c.SubscriptionType, "Sets the subscription type for resource status updates from message broker, Options: \"shared\" (only one instance receives resource status message, MQTT feature ensures exclusivity) or \"broadcast\" (all instances receive messages, hashed to determine processing instance)")
	fs.IntVar(&c.UndeliveredResourceThreshold, "undelivered-resource-threshold", c.UndeliveredResourceThreshold, "Seconds a resource can have no status (NULL) before being re-published to the message broker. Set to 0 to disable. Default: 600 (10 minutes)")
	fs.IntVar(&c.StaleDeleteEventThreshold, "stale-delete-event-threshold", c.StaleDeleteEventThreshold, "Seconds a resource can remain soft-deleted with an unreconciled delete event before that event is retired (the agent is assumed gone). Set to 0 to disable. Default: 3600 (1 hour)")
	fs.IntVar(&c.DeleteEventRepublishInterval, "delete-event-republish-interval", c.DeleteEventRepublishInterval, "Initial delete recovery backoff in seconds. Recovery runs without incoming requests until agent acknowledgement. Set to 0 to disable the recovery scheduler, not initial deletes. Default: 60")
	fs.IntVar(&c.DeleteEventRepublishMaxInterval, "delete-event-republish-max-interval", c.DeleteEventRepublishMaxInterval, "Maximum delete recovery backoff in seconds, not a deletion age limit. Default: 3600")
	fs.IntVar(&c.DeleteEventRepublishBatchSize, "delete-event-republish-batch-size", c.DeleteEventRepublishBatchSize, "Maximum recovery consumers and tombstone initializations per fleet-wide one-second round (1-1000). At most one recovery event per consumer per round. Default: 100")
	c.ConsistentHashConfig.AddFlags(fs)
}

func (c *EventServerConfig) ReadFiles() error {
	if err := c.ValidateDeleteRecovery(); err != nil {
		return err
	}
	return c.ConsistentHashConfig.ReadFiles()
}

func (c *EventServerConfig) ValidateDeleteRecovery() error {
	maxSeconds := int64(math.MaxInt64 / int64(time.Second))
	if c.DeleteEventRepublishInterval < 0 || int64(c.DeleteEventRepublishInterval) > maxSeconds {
		return fmt.Errorf("delete-event-republish-interval must be between 0 and %d seconds", maxSeconds)
	}
	if c.DeleteEventRepublishMaxInterval < 1 || int64(c.DeleteEventRepublishMaxInterval) > maxSeconds ||
		c.DeleteEventRepublishMaxInterval < c.DeleteEventRepublishInterval {
		return fmt.Errorf("delete-event-republish-max-interval must be positive, fit a duration and be at least the initial interval")
	}
	if c.DeleteEventRepublishBatchSize < 1 || c.DeleteEventRepublishBatchSize > 1000 {
		return fmt.Errorf("delete-event-republish-batch-size must be between 1 and 1000")
	}
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
