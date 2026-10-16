package config

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// TestDeleteRecoveryConfigValidation checks retry limits, overflow and disabled recovery.
func TestDeleteRecoveryConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*EventServerConfig)
		valid  bool
	}{
		{"defaults", func(c *EventServerConfig) {}, true},
		{"disabled", func(c *EventServerConfig) { c.DeleteEventRepublishInterval = 0 }, true},
		{"retirement disabled", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = 0 }, true},
		{"both disabled", func(c *EventServerConfig) {
			c.DeleteEventRepublishInterval = 0
			c.StaleDeleteEventThreshold = 0
		}, true},
		{"negative stale threshold", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = -1 }, false},
		{"overflow stale threshold", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = math.MaxInt }, false},
		{"maximum stale threshold", func(c *EventServerConfig) {
			c.StaleDeleteEventThreshold = int(math.MaxInt64 / int64(time.Second))
		}, true},
		{"negative stale threshold with recovery disabled", func(c *EventServerConfig) {
			c.DeleteEventRepublishInterval = 0
			c.StaleDeleteEventThreshold = -1
		}, false},
		{"stale threshold below initial interval", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = 30 }, true},
		{"stale threshold equals initial interval", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = 60 }, true},
		{"stale threshold above initial interval", func(c *EventServerConfig) { c.StaleDeleteEventThreshold = 120 }, true},
		{"negative interval", func(c *EventServerConfig) { c.DeleteEventRepublishInterval = -1 }, false},
		{"overflow interval", func(c *EventServerConfig) { c.DeleteEventRepublishInterval = math.MaxInt }, false},
		{"overflow maximum", func(c *EventServerConfig) { c.DeleteEventRepublishMaxInterval = math.MaxInt }, false},
		{"zero maximum", func(c *EventServerConfig) { c.DeleteEventRepublishMaxInterval = 0 }, false},
		{"maximum below initial", func(c *EventServerConfig) { c.DeleteEventRepublishMaxInterval = 1 }, false},
		{"zero batch", func(c *EventServerConfig) { c.DeleteEventRepublishBatchSize = 0 }, false},
		{"negative batch", func(c *EventServerConfig) { c.DeleteEventRepublishBatchSize = -1 }, false},
		{"unbounded batch", func(c *EventServerConfig) { c.DeleteEventRepublishBatchSize = 1001 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewEventServerConfig()
			tc.change(c)
			if err := c.ReadFiles(); (err == nil) != tc.valid {
				t.Fatalf("validation = %v, want valid=%t", err, tc.valid)
			}
		})
	}
}

// TestEventServerConfig verifies event-server defaults and command-line overrides.
func TestEventServerConfig(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]string
		want  *EventServerConfig
	}{
		{
			name:  "default subscription type",
			input: map[string]string{},
			want: &EventServerConfig{
				SubscriptionType: "shared",
				ConsistentHashConfig: &ConsistentHashConfig{
					PartitionCount:    7,
					ReplicationFactor: 20,
					Load:              1.25,
				},
				UndeliveredResourceThreshold:    600,
				StaleDeleteEventThreshold:       3600,
				DeleteEventRepublishInterval:    60,
				DeleteEventRepublishMaxInterval: 3600,
				DeleteEventRepublishBatchSize:   25,
			},
		},
		{
			name: "broadcast subscription type",
			input: map[string]string{
				"subscription-type": "broadcast",
			},
			want: &EventServerConfig{
				SubscriptionType: "broadcast",
				ConsistentHashConfig: &ConsistentHashConfig{
					PartitionCount:    7,
					ReplicationFactor: 20,
					Load:              1.25,
				},
				UndeliveredResourceThreshold:    600,
				StaleDeleteEventThreshold:       3600,
				DeleteEventRepublishInterval:    60,
				DeleteEventRepublishMaxInterval: 3600,
				DeleteEventRepublishBatchSize:   25,
			},
		},
		{
			name: "custom consistent hash config",
			input: map[string]string{
				"subscription-type":                  "broadcast",
				"consistent-hash-partition-count":    "10",
				"consistent-hash-replication-factor": "30",
				"consistent-hash-load":               "1.5",
			},
			want: &EventServerConfig{
				SubscriptionType: "broadcast",
				ConsistentHashConfig: &ConsistentHashConfig{
					PartitionCount:    10,
					ReplicationFactor: 30,
					Load:              1.5,
				},
				UndeliveredResourceThreshold:    600,
				StaleDeleteEventThreshold:       3600,
				DeleteEventRepublishInterval:    60,
				DeleteEventRepublishMaxInterval: 3600,
				DeleteEventRepublishBatchSize:   25,
			},
		},
	}

	config := NewEventServerConfig()
	pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs := pflag.CommandLine
	config.AddFlags(fs)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// set flags
			for key, value := range tc.input {
				fs.Set(key, value)
			}
			if !reflect.DeepEqual(config, tc.want) {
				t.Errorf("NewEventServerConfig() = %v; want %v", config, tc.want)
			}
			// clear flags
			fs.VisitAll(func(f *pflag.Flag) {
				fs.Lookup(f.Name).Changed = false
			})
		})
	}
}

// TestDeleteRecoveryBatchFlag checks that the fleet budget flag reaches validated configuration.
func TestDeleteRecoveryBatchFlag(t *testing.T) {
	c := NewEventServerConfig()
	fs := pflag.NewFlagSet("recovery", pflag.ContinueOnError)
	c.AddFlags(fs)
	if fs.Lookup("delete-event-republish-batch-size").DefValue != "25" {
		t.Fatal("default recovery batch must be 25")
	}
	if err := fs.Parse([]string{"--delete-event-republish-batch-size=100"}); err != nil {
		t.Fatal(err)
	}
	if c.DeleteEventRepublishBatchSize != 100 {
		t.Fatal("explicit batch size must override the default")
	}
	if err := c.ValidateDeleteRecovery(); err != nil {
		t.Fatal(err)
	}
}

// TestConsumerDeleteCascadeConfig checks opt-in flag parsing and the positive age-guard requirement.
func TestConsumerDeleteCascadeConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enabled   bool
		threshold int
		valid     bool
	}{
		{"disabled", false, 0, true},
		{"positive threshold", true, 3600, true},
		{"disabled threshold", true, 0, false},
		{"negative threshold", true, -1, false},
		{"overflow threshold", true, math.MaxInt, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewEventServerConfig()
			if c.ConsumerDeleteCascade {
				t.Fatal("consumer delete cascade must be opt-in")
			}
			fs := pflag.NewFlagSet("cascade", pflag.ContinueOnError)
			c.AddFlags(fs)
			if fs.Lookup("consumer-delete-cascade").DefValue != "false" {
				t.Fatal("cascade flag must default to false")
			}
			if tc.enabled {
				if err := fs.Parse([]string{"--consumer-delete-cascade"}); err != nil {
					t.Fatal(err)
				}
			}
			c.StaleDeleteEventThreshold = tc.threshold
			if err := c.ReadFiles(); (err == nil) != tc.valid {
				t.Fatalf("validation = %v, want valid=%t", err, tc.valid)
			}
		})
	}
}
