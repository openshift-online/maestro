package config

import (
	"math"
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

func TestDeleteRecoveryConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*EventServerConfig)
		valid  bool
	}{
		{"defaults", func(c *EventServerConfig) {}, true},
		{"disabled", func(c *EventServerConfig) { c.DeleteEventRepublishInterval = 0 }, true},
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
