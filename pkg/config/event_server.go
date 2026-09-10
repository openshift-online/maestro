package config

import (
	"fmt"

	"github.com/spf13/pflag"
)

type SubscriptionType string

const (
	SharedSubscriptionType    SubscriptionType = "shared"
	BroadcastSubscriptionType SubscriptionType = "broadcast"
)

// EventServerConfig contains the configuration for the message queue event server.
type EventServerConfig struct {
	SubscriptionType               string                `json:"subscription_type"`
	ConsistentHashConfig           *ConsistentHashConfig `json:"consistent_hash_config"`
	UndeliveredResourceThreshold   int                   `json:"undelivered_resource_threshold"`
	StaleDeleteEventThreshold      int                   `json:"stale_delete_event_threshold"`
	StaleDeleteHardDeleteThreshold int                   `json:"stale_delete_hard_delete_threshold"`
	DeleteEventRepublishInterval   int                   `json:"delete_event_republish_interval"`
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
		SubscriptionType:               "shared",
		ConsistentHashConfig:           NewConsistentHashConfig(),
		UndeliveredResourceThreshold:   600,
		StaleDeleteEventThreshold:      3600,
		StaleDeleteHardDeleteThreshold: 21600,
		DeleteEventRepublishInterval:   60,
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
	fs.IntVar(&c.StaleDeleteHardDeleteThreshold, "stale-delete-hard-delete-threshold", c.StaleDeleteHardDeleteThreshold, "Seconds a resource can remain soft-deleted, with its delete event already retired as stale, before Maestro assumes the delete succeeded, hard-deletes the resource, and sends a synthetic delete status event so watchers (e.g. Clusters Service) stop retrying. Must be greater than stale-delete-event-threshold. Set to 0 to disable. Default: 21600 (6 hours)")
	fs.IntVar(&c.DeleteEventRepublishInterval, "delete-event-republish-interval", c.DeleteEventRepublishInterval, "Seconds before a delete event is re-published for a resource that remains soft-deleted when another delete request arrives, healing agents that lost the deletion state. Raising it trades healing latency for fewer queued events per stuck resource. Set to 0 to disable republishing. Default: 60 (1 minute)")
	c.ConsistentHashConfig.AddFlags(fs)
}

func (c *EventServerConfig) ReadFiles() error {
	if err := c.ConsistentHashConfig.ReadFiles(); err != nil {
		return err
	}
	if c.StaleDeleteHardDeleteThreshold != 0 &&
		(c.StaleDeleteEventThreshold <= 0 || c.StaleDeleteHardDeleteThreshold <= c.StaleDeleteEventThreshold) {
		return fmt.Errorf("stale-delete-hard-delete-threshold (%d) must be 0 (disabled) or greater than a positive stale-delete-event-threshold (%d): a resource must be given the shorter threshold's full chance to have its delete event retired before the hard-delete pass assumes the delete succeeded, and hard-delete has no effect while the stale-delete detector itself is disabled",
			c.StaleDeleteHardDeleteThreshold, c.StaleDeleteEventThreshold)
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
