package grpcsource

import (
	"testing"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	workv1 "open-cluster-management.io/api/work/v1"
)

func TestToManifestWork(t *testing.T) {
	workload, err := marshal(map[string]interface{}{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		input    *openapi.ResourceBundle
		expected *workv1.ManifestWork
	}{
		{
			name: "covert a resource bundle - has empty fields",
			input: &openapi.ResourceBundle{
				Metadata: map[string]interface{}{
					"name":      "test",
					"namespace": "testns",
				},
				Manifests: []map[string]interface{}{
					{"a": "b"},
				},
				DeleteOption:    map[string]any{},
				ManifestConfigs: []map[string]interface{}{},
				Status:          nil,
			},
			expected: &workv1.ManifestWork{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "testns",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: workload,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "covert a resource bundle",
			input: &openapi.ResourceBundle{
				Metadata: map[string]interface{}{
					"name":      "test",
					"namespace": "testns",
				},
				Manifests: []map[string]interface{}{
					{"a": "b"},
				},
				DeleteOption: map[string]any{
					"propagationPolicy": "Foreground",
				},
				ManifestConfigs: []map[string]interface{}{
					{
						"resourceIdentifier": map[string]interface{}{
							"name": "test",
						},
					},
				},
				Status: map[string]interface{}{
					"conditions": []map[string]interface{}{
						{
							"type": "Test",
						},
					},
				},
			},
			expected: &workv1.ManifestWork{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "testns",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: workload,
								},
							},
						},
					},
					DeleteOption: &workv1.DeleteOption{
						PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
					},
					ManifestConfigs: []workv1.ManifestConfigOption{
						{
							ResourceIdentifier: workv1.ResourceIdentifier{Name: "test"},
						},
					},
				},
				Status: workv1.ManifestWorkStatus{
					Conditions: []v1.Condition{
						{
							Type: "Test",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			work, err := ToManifestWork(c.input)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}

			if !equality.Semantic.DeepEqual(c.expected, work) {
				t.Errorf("expected %v, but got %v", c.expected, work)
			}
		})
	}
}

func TestToLabelSearch(t *testing.T) {
	cases := []struct {
		name                string
		opts                v1.ListOptions
		expectedSelectable  bool
		expectedLabelSearch string
	}{
		{
			name:                "no label selector",
			opts:                v1.ListOptions{},
			expectedSelectable:  false,
			expectedLabelSearch: "",
		},
		{
			name:                "selector everything",
			opts:                v1.ListOptions{LabelSelector: labels.Everything().String()},
			expectedSelectable:  false,
			expectedLabelSearch: "",
		},
		{
			name:                "one equals selector",
			opts:                v1.ListOptions{LabelSelector: "a=b"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'@>'{"a":"b"}'`,
		},
		{
			name:                "multiple equals selector",
			opts:                v1.ListOptions{LabelSelector: "a=b,c==d"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'@>'{"a":"b","c":"d"}'`,
		},
		{
			name:                "multiple not equals selector",
			opts:                v1.ListOptions{LabelSelector: "a!=b,c!=d"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'->>'a'<>'b' and payload->'metadata'->'labels'->>'c'<>'d'`,
		},
		{
			name:                "exist selector",
			opts:                v1.ListOptions{LabelSelector: "a"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'->>'a'<>null`,
		},
		{
			name:                "in selector",
			opts:                v1.ListOptions{LabelSelector: "env in (a)"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'->>'env'in('a')`,
		},
		{
			name:                "not in selector",
			opts:                v1.ListOptions{LabelSelector: "env notin (a)"},
			expectedSelectable:  true,
			expectedLabelSearch: `payload->'metadata'->'labels'->>'env'<>'a'`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, labelSearch, selectable, err := ToLabelSearch(c.opts)
			if c.expectedSelectable != selectable {
				t.Errorf("expected %v, but got %v", c.expectedSelectable, selectable)
			}

			if c.expectedLabelSearch != labelSearch {
				t.Errorf("expected %s, but got %s", c.expectedLabelSearch, labelSearch)
			}

			if err != nil {
				t.Errorf("expected error %v", err)
			}
		})
	}
}
