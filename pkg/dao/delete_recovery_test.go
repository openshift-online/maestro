package dao

import (
	"math"
	"testing"
	"time"
)

func TestDeleteRecoveryBackoff(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		previous, initial, cap, want time.Duration
	}{
		{"first", time.Minute, time.Minute, time.Hour, 2 * time.Minute},
		{"progress", 2 * time.Minute, time.Minute, time.Hour, 4 * time.Minute},
		{"cap", 40 * time.Minute, time.Minute, time.Hour, time.Hour},
		{"changed config", time.Hour, time.Minute, 10 * time.Minute, 10 * time.Minute},
		{"null bootstrap", 0, time.Minute, time.Hour, 2 * time.Minute},
		{"overflow", math.MaxInt64 - 1, time.Second, math.MaxInt64, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := nextDeleteRetryDelay(tc.previous, tc.initial, tc.cap)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for range 100 {
				delay := jitterDeleteRetryDelay(got)
				if delay < got/2 || delay >= got {
					t.Fatalf("jitter %v outside [%v, %v)", delay, got/2, got)
				}
			}
		})
	}
}
