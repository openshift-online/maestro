package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func TestNewTablePrinter(t *testing.T) {
	var buf bytes.Buffer
	printer := NewTablePrinter(&buf)

	if printer == nil {
		t.Fatal("NewTablePrinter() returned nil")
	}

	if printer.writer == nil {
		t.Error("NewTablePrinter() writer is nil")
	}
}

func TestTablePrinter_Flush(t *testing.T) {
	var buf bytes.Buffer
	printer := NewTablePrinter(&buf)

	err := printer.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}
}

func TestPrintResourceBundleList(t *testing.T) {
	now := time.Now()
	bundles := []openapi.ResourceBundle{
		{
			Id:           openapi.PtrString("bundle-1"),
			Name:         openapi.PtrString("test-bundle-1"),
			ConsumerName: openapi.PtrString("consumer1"),
			Version:      openapi.PtrInt32(1),
			CreatedAt:    &now,
			Status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Applied",
						"status": "True",
					},
				},
			},
		},
		{
			Id:           openapi.PtrString("bundle-2"),
			Name:         openapi.PtrString("test-bundle-2"),
			ConsumerName: openapi.PtrString("consumer2"),
			Version:      openapi.PtrInt32(2),
			CreatedAt:    &now,
			Status:       map[string]interface{}{},
		},
	}

	var buf bytes.Buffer
	err := PrintResourceBundleList(&buf, bundles)

	if err != nil {
		t.Fatalf("PrintResourceBundleList() error = %v", err)
	}

	output := buf.String()

	// Verify header is present
	if !strings.Contains(output, "ID") {
		t.Error("PrintResourceBundleList() output missing ID header")
	}
	if !strings.Contains(output, "NAME") {
		t.Error("PrintResourceBundleList() output missing NAME header")
	}
	if !strings.Contains(output, "CONSUMER") {
		t.Error("PrintResourceBundleList() output missing CONSUMER header")
	}

	// Verify data is present
	if !strings.Contains(output, "bundle-1") {
		t.Error("PrintResourceBundleList() output missing bundle-1")
	}
	if !strings.Contains(output, "test-bundle-1") {
		t.Error("PrintResourceBundleList() output missing test-bundle-1")
	}
	if !strings.Contains(output, "Applied") {
		t.Error("PrintResourceBundleList() output missing Applied status")
	}
}

func TestPrintResourceBundle(t *testing.T) {
	now := time.Now()
	bundle := &openapi.ResourceBundle{
		Id:           openapi.PtrString("bundle-1"),
		Name:         openapi.PtrString("test-bundle"),
		ConsumerName: openapi.PtrString("consumer1"),
		Version:      openapi.PtrInt32(1),
		CreatedAt:    &now,
		UpdatedAt:    &now,
		Status: map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Applied",
					"status": "True",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := PrintResourceBundle(&buf, bundle)

	if err != nil {
		t.Fatalf("PrintResourceBundle() error = %v", err)
	}

	output := buf.String()

	// Verify fields are present
	if !strings.Contains(output, "FIELD") {
		t.Error("PrintResourceBundle() output missing FIELD header")
	}
	if !strings.Contains(output, "ID") {
		t.Error("PrintResourceBundle() output missing ID field")
	}
	if !strings.Contains(output, "bundle-1") {
		t.Error("PrintResourceBundle() output missing bundle-1")
	}
	if !strings.Contains(output, "test-bundle") {
		t.Error("PrintResourceBundle() output missing test-bundle")
	}
	if !strings.Contains(output, "Applied") {
		t.Error("PrintResourceBundle() output missing Applied status")
	}
}

func TestPrintConsumerList(t *testing.T) {
	now := time.Now()
	labels := map[string]string{
		"env":  "prod",
		"team": "platform",
	}

	consumers := []openapi.Consumer{
		{
			Id:        openapi.PtrString("consumer-1"),
			Name:      openapi.PtrString("test-consumer-1"),
			Labels:    &labels,
			CreatedAt: &now,
		},
		{
			Id:        openapi.PtrString("consumer-2"),
			Name:      openapi.PtrString("test-consumer-2"),
			CreatedAt: &now,
		},
	}

	var buf bytes.Buffer
	err := PrintConsumerList(&buf, consumers)

	if err != nil {
		t.Fatalf("PrintConsumerList() error = %v", err)
	}

	output := buf.String()

	// Verify header is present
	if !strings.Contains(output, "ID") {
		t.Error("PrintConsumerList() output missing ID header")
	}
	if !strings.Contains(output, "NAME") {
		t.Error("PrintConsumerList() output missing NAME header")
	}
	if !strings.Contains(output, "LABELS") {
		t.Error("PrintConsumerList() output missing LABELS header")
	}

	// Verify data is present
	if !strings.Contains(output, "consumer-1") {
		t.Error("PrintConsumerList() output missing consumer-1")
	}
	if !strings.Contains(output, "test-consumer-1") {
		t.Error("PrintConsumerList() output missing test-consumer-1")
	}
	// Labels should be present (env=prod or team=platform)
	if !strings.Contains(output, "env=prod") && !strings.Contains(output, "team=platform") {
		t.Error("PrintConsumerList() output missing labels")
	}
}

func TestPrintConsumer(t *testing.T) {
	now := time.Now()
	labels := map[string]string{
		"env": "staging",
	}

	consumer := &openapi.Consumer{
		Id:        openapi.PtrString("consumer-1"),
		Name:      openapi.PtrString("test-consumer"),
		Labels:    &labels,
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	var buf bytes.Buffer
	err := PrintConsumer(&buf, consumer)

	if err != nil {
		t.Fatalf("PrintConsumer() error = %v", err)
	}

	output := buf.String()

	// Verify fields are present
	if !strings.Contains(output, "FIELD") {
		t.Error("PrintConsumer() output missing FIELD header")
	}
	if !strings.Contains(output, "ID") {
		t.Error("PrintConsumer() output missing ID field")
	}
	if !strings.Contains(output, "consumer-1") {
		t.Error("PrintConsumer() output missing consumer-1")
	}
	if !strings.Contains(output, "test-consumer") {
		t.Error("PrintConsumer() output missing test-consumer")
	}
	if !strings.Contains(output, "env=staging") {
		t.Error("PrintConsumer() output missing labels")
	}
}

func TestPrintResourceBundleStatus(t *testing.T) {
	status := map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Applied",
				"status":             "True",
				"reason":             "AppliedSuccessfully",
				"message":            "Resource applied successfully",
				"lastTransitionTime": "2024-01-01T00:00:00Z",
			},
		},
	}

	var buf bytes.Buffer
	err := PrintResourceBundleStatus(&buf, "bundle-123", status)

	if err != nil {
		t.Fatalf("PrintResourceBundleStatus() error = %v", err)
	}

	output := buf.String()

	// Verify basic fields
	if !strings.Contains(output, "bundle-123") {
		t.Error("PrintResourceBundleStatus() output missing bundle ID")
	}
	if !strings.Contains(output, "Applied") {
		t.Error("PrintResourceBundleStatus() output missing Applied status")
	}

	// Verify condition details
	if !strings.Contains(output, "Conditions:") {
		t.Error("PrintResourceBundleStatus() output missing Conditions section")
	}
	if !strings.Contains(output, "AppliedSuccessfully") {
		t.Error("PrintResourceBundleStatus() output missing reason")
	}
	if !strings.Contains(output, "Resource applied successfully") {
		t.Error("PrintResourceBundleStatus() output missing message")
	}
}

func TestGetStringPtr(t *testing.T) {
	tests := []struct {
		name string
		ptr  *string
		want string
	}{
		{
			name: "non-nil pointer",
			ptr:  openapi.PtrString("test-value"),
			want: "test-value",
		},
		{
			name: "nil pointer",
			ptr:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringPtr(tt.ptr)
			if got != tt.want {
				t.Errorf("getStringPtr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInt32Ptr(t *testing.T) {
	tests := []struct {
		name string
		ptr  *int32
		want int32
	}{
		{
			name: "non-nil pointer",
			ptr:  openapi.PtrInt32(42),
			want: 42,
		},
		{
			name: "nil pointer",
			ptr:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getInt32Ptr(tt.ptr)
			if got != tt.want {
				t.Errorf("getInt32Ptr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		time *time.Time
		want string
	}{
		{
			name: "valid time",
			time: func() *time.Time {
				t := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
				return &t
			}(),
			want: "2024-01-15 14:30:45",
		},
		{
			name: "nil time",
			time: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels *map[string]string
		want   []string // Expected substrings (labels can be in any order)
	}{
		{
			name: "non-nil labels",
			labels: &map[string]string{
				"env":  "prod",
				"team": "platform",
			},
			want: []string{"env=prod", "team=platform"},
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   []string{},
		},
		{
			name:   "empty labels",
			labels: &map[string]string{},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLabels(tt.labels)

			if len(tt.want) == 0 {
				if got != "" {
					t.Errorf("formatLabels() = %v, want empty string", got)
				}
				return
			}

			// Verify all expected labels are present
			for _, expectedLabel := range tt.want {
				if !strings.Contains(got, expectedLabel) {
					t.Errorf("formatLabels() = %v, should contain %v", got, expectedLabel)
				}
			}
		})
	}
}

func TestGetStatusFromMap(t *testing.T) {
	tests := []struct {
		name   string
		status map[string]interface{}
		want   string
	}{
		{
			name: "Applied status",
			status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Applied",
						"status": "True",
					},
				},
			},
			want: "Applied",
		},
		{
			name: "Pending status",
			status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Applied",
						"status": "False",
					},
				},
			},
			want: "Pending",
		},
		{
			name:   "empty status",
			status: map[string]interface{}{},
			want:   "Unknown",
		},
		{
			name: "no conditions",
			status: map[string]interface{}{
				"other": "value",
			},
			want: "Unknown",
		},
		{
			name: "invalid conditions format",
			status: map[string]interface{}{
				"conditions": "not-an-array",
			},
			want: "Unknown",
		},
		{
			name: "empty conditions",
			status: map[string]interface{}{
				"conditions": []interface{}{},
			},
			want: "Unknown",
		},
		{
			name: "no Applied condition",
			status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "OtherCondition",
						"status": "True",
					},
				},
			},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStatusFromMap(tt.status)
			if got != tt.want {
				t.Errorf("getStatusFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintResourceBundleList_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := PrintResourceBundleList(&buf, []openapi.ResourceBundle{})

	if err != nil {
		t.Fatalf("PrintResourceBundleList() error = %v", err)
	}

	output := buf.String()

	// Should still have header
	if !strings.Contains(output, "ID") {
		t.Error("PrintResourceBundleList() empty list missing header")
	}
}

func TestPrintConsumerList_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := PrintConsumerList(&buf, []openapi.Consumer{})

	if err != nil {
		t.Fatalf("PrintConsumerList() error = %v", err)
	}

	output := buf.String()

	// Should still have header
	if !strings.Contains(output, "ID") {
		t.Error("PrintConsumerList() empty list missing header")
	}
}

func TestPrintResourceBundleStatus_NoConditions(t *testing.T) {
	status := map[string]interface{}{}

	var buf bytes.Buffer
	err := PrintResourceBundleStatus(&buf, "bundle-123", status)

	if err != nil {
		t.Fatalf("PrintResourceBundleStatus() error = %v", err)
	}

	output := buf.String()

	// Should show Unknown status
	if !strings.Contains(output, "Unknown") {
		t.Error("PrintResourceBundleStatus() should show Unknown for empty status")
	}
}
