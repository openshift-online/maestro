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
		manifest         map[string]interface{}
		deleteOption     map[string]interface{}
		manifestConfig   map[string]interface{}
		expected         datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:     "empty",
			manifest: map[string]interface{}{},
			expected: datatypes.JSONMap{},
		},
		{
			name:           "valid",
			manifest:       newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			manifestConfig: newJSONMap(t, "{\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"ServerSideApply\"}}"),
			expected:       newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"feedbackRules\":[{\"jsonPaths\":[{\"name\":\"status\",\"path\":\".status\"}],\"type\":\"JSONPaths\"}],\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"ServerSideApply\"}}],\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}]}}"),
		},
		{
			name:           "valid",
			deleteOption:   newJSONMap(t, "{\"propagationPolicy\": \"Orphan\"}"),
			manifest:       newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			manifestConfig: newJSONMap(t, "{\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"CreateOnly\"}}"),
			expected:       newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"deleteOption\":{\"propagationPolicy\":\"Orphan\"},\"manifestConfigs\":[{\"feedbackRules\":[{\"jsonPaths\":[{\"name\":\"status\",\"path\":\".status\"}],\"type\":\"JSONPaths\"}],\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"CreateOnly\"}}],\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}]}}"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifest, err := EncodeManifest(c.manifest, c.deleteOption, c.manifestConfig)
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
		name                   string
		input                  datatypes.JSONMap
		expectedManifest       map[string]interface{}
		expectedDeleteOption   map[string]interface{}
		expectedManifestConfig map[string]interface{}
		expectedErrorMsg       string
	}{
		{
			name:                   "empty",
			input:                  datatypes.JSONMap{},
			expectedManifest:       nil,
			expectedDeleteOption:   nil,
			expectedManifestConfig: nil,
			expectedErrorMsg:       "",
		},
		{
			name:                   "valid",
			input:                  newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"deleteOption\":{\"propagationPolicy\":\"Orphan\"},\"manifestConfigs\":[{\"feedbackRules\":[{\"jsonPaths\":[{\"name\":\"status\",\"path\":\".status\"}],\"type\":\"JSONPaths\"}],\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"CreateOnly\"}}],\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}]}}"),
			expectedManifest:       newJSONMap(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedDeleteOption:   newJSONMap(t, "{\"propagationPolicy\": \"Orphan\"}"),
			expectedManifestConfig: newJSONMap(t, "{\"resourceIdentifier\":{\"group\":\"\",\"name\":\"test\",\"namespace\":\"test\",\"resource\":\"configmaps\"},\"updateStrategy\":{\"type\":\"CreateOnly\"},\"feedbackRules\":[{\"jsonPaths\":[{\"name\":\"status\",\"path\":\".status\"}],\"type\":\"JSONPaths\"}]}"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotManifest, gotDeleteOption, gotManifestConfig, err := DecodeManifest(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if !equality.Semantic.DeepDerivative(c.expectedManifest, gotManifest) {
				t.Errorf("expected %#v but got: %#v", c.expectedManifest, gotManifest)
			}
			if !equality.Semantic.DeepDerivative(c.expectedDeleteOption, gotDeleteOption) {
				t.Errorf("expected %#v but got: %#v", c.expectedDeleteOption, gotDeleteOption)
			}
			if !equality.Semantic.DeepDerivative(c.expectedManifestConfig, gotManifestConfig) {
				t.Errorf("expected %#v but got: %#v", c.expectedManifestConfig, gotManifestConfig)
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
			input:    newJSONMap(t, "{\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"id\":\"1f21fcbe-3e41-4639-ab8d-1713c578e4cd\",\"time\":\"2024-03-07T03:29:12.094854533Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.status.update_request\",\"source\":\"maestro-agent-59d9c485d9-7bvwb\",\"resourceid\":\"b9368296-3200-42ec-bfbb-f7d44a06c4e0\",\"sequenceid\":\"1765580430112722944\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"originalsource\":\"maestro\",\"resourceversion\":\"1\",\"data\":{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestWorkComplete\",\"status\":\"True\",\"message\":\"Apply manifest work complete\",\"lastTransitionTime\":\"2024-03-07T03:56:35Z\"},{\"type\":\"Available\",\"reason\":\"ResourcesAvailable\",\"status\":\"True\",\"message\":\"All resources are available\",\"lastTransitionTime\":\"2024-03-07T03:56:35Z\"}],\"resourceStatus\":[{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"message\":\"Apply manifest complete\",\"lastTransitionTime\":\"2024-03-07T03:56:35Z\"},{\"type\":\"Available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"message\":\"Resource is available\",\"lastTransitionTime\":\"2024-03-07T03:56:35Z\"},{\"type\":\"StatusFeedbackSynced\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"message\":\"\",\"lastTransitionTime\":\"2024-03-07T03:56:35Z\"}],\"resourceMeta\":{\"kind\":\"Deployment\",\"name\":\"nginx\",\"group\":\"apps\",\"ordinal\":1,\"version\":\"v1\",\"resource\":\"deployments\",\"namespace\":\"default\"},\"statusFeedback\":{\"values\":[{\"name\":\"status\",\"fieldValue\":{\"type\":\"JsonRaw\",\"jsonRaw\":\"{\\\"availableReplicas\\\":2,\\\"conditions\\\":[{\\\"lastTransitionTime\\\":\\\"2024-03-07T03:56:35Z\\\",\\\"lastUpdateTime\\\":\\\"2024-03-07T03:56:38Z\\\",\\\"message\\\":\\\"ReplicaSet \\\\\\\"nginx-5d6b548959\\\\\\\" has successfully progressed.\\\",\\\"reason\\\":\\\"NewReplicaSetAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Progressing\\\"},{\\\"lastTransitionTime\\\":\\\"2024-03-07T03:58: 26Z\\\",\\\"lastUpdateTime\\\":\\\"2024-03-07T03:58:26Z\\\",\\\"message\\\":\\\"Deployment has minimum availability.\\\",\\\"reason\\\":\\\"MinimumReplicasAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Available\\\"}],\\\"observedGeneration\\\":2,\\\"readyReplicas\\\":2,\\\"replicas\\\":2,\\\"updatedReplicas\\\":2}\"}}]}}]}}"),
			expected: newJSONMap(t, "{\"ContentStatus\":{\"availableReplicas\":2,\"conditions\":[{\"lastTransitionTime\":\"2024-03-07T03:56:35Z\",\"lastUpdateTime\":\"2024-03-07T03:56:38Z\",\"message\":\"ReplicaSet \\\"nginx-5d6b548959\\\" has successfully progressed.\",\"reason\":\"NewReplicaSetAvailable\",\"status\":\"True\",\"type\":\"Progressing\"},{\"lastTransitionTime\":\"2024-03-07T03:58: 26Z\",\"lastUpdateTime\":\"2024-03-07T03:58:26Z\",\"message\":\"Deployment has minimum availability.\",\"reason\":\"MinimumReplicasAvailable\",\"status\":\"True\",\"type\":\"Available\"}],\"observedGeneration\":2,\"readyReplicas\":2,\"replicas\":2,\"updatedReplicas\":2},\"ReconcileStatus\":{\"Conditions\":[{\"lastTransitionTime\":\"2024-03-07T03:56:35Z\",\"message\":\"Apply manifest complete\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"type\":\"Applied\"},{\"lastTransitionTime\":\"2024-03-07T03:56:35Z\",\"message\":\"Resource is available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-03-07T03:56:35Z\",\"message\":\"\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"type\":\"StatusFeedbackSynced\"}],\"ObservedVersion\":1,\"SequenceID\":\"1765580430112722944\"}}"),
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

func newJSONMAPList(t *testing.T, data ...string) []map[string]any {
	jsonmapList := []map[string]any{}
	for _, d := range data {
		jsonmapList = append(jsonmapList, newJSONMap(t, d))
	}

	return jsonmapList
}
