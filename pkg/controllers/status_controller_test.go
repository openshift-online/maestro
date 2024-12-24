package controllers

import (
	"testing"
)

func TestBatchStatusEventIDs(t *testing.T) {
	const batchSize = 500

	cases := []struct {
		name           string
		statusEventIDs []string
		expected       [][]string
	}{
		{
			name:           "empty input",
			statusEventIDs: []string{},
			expected:       [][]string{},
		},
		{
			name:           "single batch less than batch size",
			statusEventIDs: make([]string, 499),
			expected:       [][]string{make([]string, 499)},
		},
		{
			name:           "single batch equal to batch size",
			statusEventIDs: make([]string, batchSize),
			expected:       [][]string{make([]string, batchSize)},
		},
		{
			name:           "multiple batches full",
			statusEventIDs: make([]string, batchSize*2),
			expected:       [][]string{make([]string, batchSize), make([]string, batchSize)},
		},
		{
			name:           "multiple batches partial last",
			statusEventIDs: make([]string, batchSize+100),
			expected:       [][]string{make([]string, batchSize), make([]string, 100)},
		},
		{
			name:           "multiple batches full partial last",
			statusEventIDs: make([]string, batchSize*2+300),
			expected:       [][]string{make([]string, batchSize), make([]string, batchSize), make([]string, 300)},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := batchStatusEventIDs(tt.statusEventIDs, batchSize)

			// Ensure the number of batches is correct
			if len(result) != len(tt.expected) {
				t.Errorf("number of batches mismatch, got %d, want %d", len(result), len(tt.expected))
			}

			// Check the length of each batch
			for i := range result {
				if len(result[i]) != len(tt.expected[i]) {
					t.Errorf("length of batch %d mismatch, got %d, want %d", i+1, len(result[i]), len(tt.expected[i]))
				}
			}
		})
	}
}
