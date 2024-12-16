package config

import (
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

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
