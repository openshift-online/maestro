package api

import (
	"testing"

	. "github.com/onsi/gomega"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJSONMapStausToResourceStatus(t *testing.T) {
	RegisterTestingT(t)
	cases := []struct {
		name     string
		input    datatypes.JSONMap
		expected ResourceStatus
	}{
		{
			name: "empty",
			input: datatypes.JSONMap{
				"ContentStatus":   datatypes.JSONMap{},
				"ReconcileStatus": datatypes.JSONMap{},
			},
			expected: ResourceStatus{
				ContentStatus:   datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{},
			},
		},
		{
			name: "content status",
			input: datatypes.JSONMap{
				"ContentStatus": datatypes.JSONMap{
					"foo": "bar",
				},
				"ReconcileStatus": datatypes.JSONMap{},
			},
			expected: ResourceStatus{
				ContentStatus: datatypes.JSONMap{
					"foo": "bar",
				},
				ReconcileStatus: &ReconcileStatus{},
			},
		},
		{
			name: "reconcile status",
			input: datatypes.JSONMap{
				"ContentStatus": datatypes.JSONMap{},
				"ReconcileStatus": datatypes.JSONMap{
					"ObservedVersion": 1,
					"SequenceID":      "123",
					"Conditions": []datatypes.JSONMap{
						{
							"type":    "foo",
							"status":  "True",
							"reason":  "bar",
							"message": "baz",
						},
					},
				},
			},
			expected: ResourceStatus{
				ContentStatus: datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{
					ObservedVersion: 1,
					SequenceID:      "123",
					Conditions: []metav1.Condition{
						{
							Type:    "foo",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
					},
				},
			},
		},
		{
			name: "reconcile status with deleted Condition",
			input: datatypes.JSONMap{
				"ContentStatus": datatypes.JSONMap{},
				"ReconcileStatus": datatypes.JSONMap{
					"ObservedVersion": 1,
					"SequenceID":      "123",
					"Conditions": []datatypes.JSONMap{
						{
							"type":    "foo",
							"status":  "True",
							"reason":  "bar",
							"message": "baz",
						},
						{
							"type":    "Deleted",
							"status":  "True",
							"reason":  "bar",
							"message": "baz",
						},
					},
				},
			},
			expected: ResourceStatus{
				ContentStatus: datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{
					ObservedVersion: 1,
					SequenceID:      "123",
					Conditions: []metav1.Condition{
						{
							Type:    "foo",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
						{
							Type:    "Deleted",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := JSONMapStausToResourceStatus(tc.input)
			Expect(err).To(BeNil())

			Expect(actual.ContentStatus).To(Equal(tc.expected.ContentStatus))
			Expect(actual.ReconcileStatus).To(Equal(tc.expected.ReconcileStatus))
		})
	}
}

func TestResourceStatusToJSONMap(t *testing.T) {
	RegisterTestingT(t)
	cases := []struct {
		name     string
		input    *ResourceStatus
		expected datatypes.JSONMap
	}{
		{
			name: "empty",
			input: &ResourceStatus{
				ContentStatus:   datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{},
			},
			expected: datatypes.JSONMap{
				"ContentStatus": map[string]interface{}{},
				"ReconcileStatus": map[string]interface{}{
					"ObservedVersion": float64(0),
					"SequenceID":      "",
					"Conditions":      nil,
				},
			},
		},
		{
			name: "content status",
			input: &ResourceStatus{
				ContentStatus: datatypes.JSONMap{
					"foo": "bar",
				},
				ReconcileStatus: &ReconcileStatus{},
			},
			expected: datatypes.JSONMap{
				"ContentStatus": map[string]interface{}{
					"foo": "bar",
				},
				"ReconcileStatus": map[string]interface{}{
					"ObservedVersion": float64(0),
					"SequenceID":      "",
					"Conditions":      nil,
				},
			},
		},
		{
			name: "reconcile status",
			input: &ResourceStatus{
				ContentStatus: datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{
					ObservedVersion: 1,
					SequenceID:      "123",
					Conditions: []metav1.Condition{
						{
							Type:    "foo",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
					},
				},
			},
			expected: datatypes.JSONMap{
				"ContentStatus": map[string]interface{}{},
				"ReconcileStatus": map[string]interface{}{
					"ObservedVersion": float64(1),
					"SequenceID":      "123",
					"Conditions": []interface{}{
						map[string]interface{}{
							"type":               "foo",
							"status":             "True",
							"reason":             "bar",
							"message":            "baz",
							"lastTransitionTime": nil,
						},
					},
				},
			},
		},
		{
			name: "reconcile status with deleted Condition",
			input: &ResourceStatus{
				ContentStatus: datatypes.JSONMap{},
				ReconcileStatus: &ReconcileStatus{
					ObservedVersion: 1,
					SequenceID:      "123",
					Conditions: []metav1.Condition{
						{
							Type:    "foo",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
						{
							Type:    "Deleted",
							Status:  "True",
							Reason:  "bar",
							Message: "baz",
						},
					},
				},
			},
			expected: datatypes.JSONMap{
				"ContentStatus": map[string]interface{}{},
				"ReconcileStatus": map[string]interface{}{
					"ObservedVersion": float64(1),
					"SequenceID":      "123",
					"Conditions": []interface{}{
						map[string]interface{}{
							"type":               "foo",
							"status":             "True",
							"reason":             "bar",
							"message":            "baz",
							"lastTransitionTime": nil,
						},
						map[string]interface{}{
							"type":               "Deleted",
							"status":             "True",
							"reason":             "bar",
							"message":            "baz",
							"lastTransitionTime": nil,
						},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := ResourceStatusToJSONMap(tc.input)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(tc.expected))
		})
	}
}
