package api

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/api/equality"
)

func TestEncodeManifest(t *testing.T) {
	cases := []struct {
		name             string
		input            map[string]interface{}
		deleteOption     map[string]interface{}
		updateStrategy   map[string]interface{}
		expected         datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:     "empty",
			input:    map[string]interface{}{},
			expected: datatypes.JSONMap{},
		},
		{
			name:     "valid",
			input:    newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expected: newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
		},
		{
			name:           "valid",
			deleteOption:   newJSONMap(t, "{\"propagationPolicy\": \"Orphan\"}"),
			updateStrategy: newJSONMap(t, "{\"type\": \"CreateOnly\"}"),
			input:          newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expected:       newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"configOption\":{\"updateStrategy\": {\"type\": \"CreateOnly\"}},\"deleteOption\": {\"propagationPolicy\": \"Orphan\"},\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifest, err := EncodeManifest(c.input, c.deleteOption, c.updateStrategy)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if !equality.Semantic.DeepDerivative(c.expected, gotManifest) {
				t.Errorf("expected %#v but got: %#v", c.expected, gotManifest)
			}
		})
	}
}

func TestDecodeManifest(t *testing.T) {
	cases := []struct {
		name             string
		input            datatypes.JSONMap
		expected         map[string]interface{}
		expectedErrorMsg string
	}{
		{
			name:             "empty",
			input:            datatypes.JSONMap{},
			expected:         nil,
			expectedErrorMsg: "",
		},
		{
			name:     "valid",
			input:    newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expected: newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifest, err := DecodeManifest(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if !equality.Semantic.DeepDerivative(c.expected, gotManifest) {
				t.Errorf("expected %#v but got: %#v", c.expected, gotManifest)
			}
		})
	}
}

func TestDecodeManifestBundle(t *testing.T) {
	cases := []struct {
		name             string
		input            datatypes.JSONMap
		expected         []map[string]interface{}
		expectedErrorMsg string
	}{
		{
			name:             "empty",
			input:            datatypes.JSONMap{},
			expected:         nil,
			expectedErrorMsg: "",
		},
		{
			name:  "valid",
			input: newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
			expected: []map[string]interface{}{
				newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}}"),
				newJSONMap(t, "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}"),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifests, err := DecodeManifestBundle(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if len(gotManifests) != len(c.expected) {
				t.Errorf("expected %d resource in manifest bundle but got: %d", len(c.expected), len(gotManifests))
				return
			}
			for i, expected := range c.expected {
				if !equality.Semantic.DeepDerivative(expected, gotManifests[i]) {
					t.Errorf("expected %#v but got: %#v", expected, gotManifests[i])
				}
			}
		})
	}
}

func TestDecodeStatus(t *testing.T) {
	cases := []struct {
		name             string
		input            datatypes.JSONMap
		expected         map[string]interface{}
		expectedErrorMsg string
	}{
		{
			name:             "empty",
			input:            datatypes.JSONMap{},
			expected:         nil,
			expectedErrorMsg: "",
		},
		{
			name:     "valid",
			input:    newJSONMap(t, "{\"id\":\"1f21fcbe-3e41-4639-ab8d-1713c578e4cd\",\"time\":\"2024-03-07T03:29:12.094854533Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.status.update_request\",\"source\":\"maestro-agent-59d9c485d9-7bvwb\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"b9368296-3200-42ec-bfbb-f7d44a06c4e0\",\"sequenceid\":\"1765580430112722944\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"originalsource\":\"maestro\",\"resourceversion\":\"1\",\"data\":{\"status\":{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"message\":\"Apply manifest complete\",\"lastTransitionTime\":\"2024-03-07T03:29:03Z\"},{\"type\":\"Available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"message\":\"Resource is available\",\"lastTransitionTime\":\"2024-03-07T03:29:03Z\"},{\"type\":\"StatusFeedbackSynced\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"message\":\"\",\"lastTransitionTime\":\"2024-03-07T03:29:03Z\"}],\"resourceMeta\":{\"kind\":\"Deployment\",\"name\":\"nginx1\",\"group\":\"apps\",\"ordinal\":0,\"version\":\"v1\",\"resource\":\"deployments\",\"namespace\":\"default\"},\"statusFeedback\":{\"values\":[{\"name\":\"status\",\"fieldValue\":{\"type\":\"JsonRaw\",\"jsonRaw\":\"{\\\"availableReplicas\\\":1,\\\"conditions\\\":[{\\\"lastTransitionTime\\\":\\\"2024-03-07T03:29:06Z\\\",\\\"lastUpdateTime\\\":\\\"2024-03-07T03:29:06Z\\\",\\\"message\\\":\\\"Deployment has minimum availability.\\\",\\\"reason\\\":\\\"MinimumReplicasAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Available\\\"},{\\\"lastTransitionTime\\\":\\\"2024-03-07T03:29:03Z\\\",\\\"lastUpdateTime\\\":\\\"2024-03-07T03:29:06Z\\\",\\\"message\\\":\\\"ReplicaSet \\\\\\\"nginx1-5d6b548959\\\\\\\" has successfully progressed.\\\",\\\"reason\\\":\\\"NewReplicaSetAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Progressing\\\"}],\\\"observedGeneration\\\":1,\\\"readyReplicas\\\":1,\\\"replicas\\\":1,\\\"updatedReplicas\\\":1}\"}}]}},\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestWorkComplete\",\"status\":\"True\",\"message\":\"Apply manifest work complete\",\"lastTransitionTime\":\"2024-03-07T03:29:03Z\"},{\"type\":\"Available\",\"reason\":\"ResourcesAvailable\",\"status\":\"True\",\"message\":\"All resources are available\",\"lastTransitionTime\":\"2024-03-07T03:29:03Z\"}]}}"),
			expected: newJSONMap(t, "{\"ContentStatus\":{\"availableReplicas\":1,\"observedGeneration\":1,\"readyReplicas\":1,\"replicas\":1,\"updatedReplicas\":1,\"conditions\":[{\"lastTransitionTime\":\"2024-03-07T03:29:06Z\",\"lastUpdateTime\":\"2024-03-07T03:29:06Z\",\"message\":\"Deployment has minimum availability.\",\"reason\":\"MinimumReplicasAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-03-07T03:29:03Z\",\"lastUpdateTime\":\"2024-03-07T03:29:06Z\",\"message\":\"ReplicaSet \\\"nginx1-5d6b548959\\\" has successfully progressed.\",\"reason\":\"NewReplicaSetAvailable\",\"status\":\"True\",\"type\":\"Progressing\"}]},\"ReconcileStatus\":{\"Conditions\":[{\"lastTransitionTime\":\"2024-03-07T03:29:03Z\",\"message\":\"Apply manifest complete\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"type\":\"Applied\"},{\"lastTransitionTime\":\"2024-03-07T03:29:03Z\",\"message\":\"Resource is available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-03-07T03:29:03Z\",\"message\":\"\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"type\":\"StatusFeedbackSynced\"}],\"ObservedVersion\":1,\"SequenceID\":\"1765580430112722944\"}}"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifest, err := DecodeStatus(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if !equality.Semantic.DeepDerivative(c.expected, gotManifest) {
				t.Errorf("expected %#v but got: %#v", c.expected, gotManifest)
			}
		})
	}
}

func newJSONMap(t *testing.T, data string) datatypes.JSONMap {
	jsonmap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data), &jsonmap); err != nil {
		t.Fatal(err)
	}

	return jsonmap
}
