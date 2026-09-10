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
				UndeliveredResourceThreshold:   600,
				StaleDeleteEventThreshold:      3600,
				StaleDeleteHardDeleteThreshold: 21600,
				DeleteEventRepublishInterval:   60,
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
				UndeliveredResourceThreshold:   600,
				StaleDeleteEventThreshold:      3600,
				StaleDeleteHardDeleteThreshold: 21600,
				DeleteEventRepublishInterval:   60,
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
				UndeliveredResourceThreshold:   600,
				StaleDeleteEventThreshold:      3600,
				StaleDeleteHardDeleteThreshold: 21600,
				DeleteEventRepublishInterval:   60,
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

func TestEventServerConfigReadFilesValidatesStaleDeleteThresholds(t *testing.T) {
	cases := []struct {
		name                           string
		staleDeleteEventThreshold      int
		staleDeleteHardDeleteThreshold int
		wantErr                        bool
	}{
		{name: "hard-delete disabled", staleDeleteEventThreshold: 3600, staleDeleteHardDeleteThreshold: 0, wantErr: false},
		{name: "hard-delete greater than event threshold", staleDeleteEventThreshold: 3600, staleDeleteHardDeleteThreshold: 21600, wantErr: false},
		{name: "hard-delete equal to event threshold", staleDeleteEventThreshold: 3600, staleDeleteHardDeleteThreshold: 3600, wantErr: true},
		{name: "hard-delete less than event threshold", staleDeleteEventThreshold: 3600, staleDeleteHardDeleteThreshold: 600, wantErr: true},
		{name: "hard-delete enabled while stale-delete detector disabled", staleDeleteEventThreshold: 0, staleDeleteHardDeleteThreshold: 21600, wantErr: true},
		{name: "both disabled", staleDeleteEventThreshold: 0, staleDeleteHardDeleteThreshold: 0, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := NewEventServerConfig()
			config.StaleDeleteEventThreshold = tc.staleDeleteEventThreshold
			config.StaleDeleteHardDeleteThreshold = tc.staleDeleteHardDeleteThreshold

			err := config.ReadFiles()
			if tc.wantErr && err == nil {
				t.Errorf("ReadFiles() = nil; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ReadFiles() = %v; want nil", err)
			}
		})
	}
}
