package api

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

func TestDecodeManifestBundle(t *testing.T) {
	cases := []struct {
		name             string
		input            datatypes.JSONMap
		expected         *workpayload.ManifestBundle
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
			expected: &workpayload.ManifestBundle{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: []byte("{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}}"),
						},
					},
					{
						RawExtension: runtime.RawExtension{
							Raw: []byte("{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"nginx\"}},\"spec\":{\"containers\":[{\"image\":\"nginxinc/nginx-unprivileged\",\"name\":\"nginx\"}]}}}}"),
						},
					},
				},
				DeleteOption: &workv1.DeleteOption{
					PropagationPolicy: "Foreground",
				},
				ManifestConfigs: []workv1.ManifestConfigOption{
					{
						UpdateStrategy: &workv1.UpdateStrategy{
							Type: "ServerSideApply",
						},
						ResourceIdentifier: workv1.ResourceIdentifier{
							Name:      "nginx",
							Group:     "apps",
							Resource:  "deployments",
							Namespace: "default",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DecodeManifestBundle(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if c.expected != nil && got != nil {
				if !equality.Semantic.DeepDerivative(c.expected.Manifests[1], got.Manifests[1]) {
					t.Errorf("expected Manifests %s but got: %s", c.expected.Manifests[1].RawExtension.Raw, got.Manifests[1].RawExtension.Raw)
				}
			}
			if !equality.Semantic.DeepDerivative(c.expected, got) {
				t.Errorf("expected %#v but got: %#v", c.expected, got)
			}
		})
	}

}

func TestDecodeManifestBundleToObjects(t *testing.T) {
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
			gotObjects, err := DecodeManifestBundleToObjects(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			if len(gotObjects) != len(c.expected) {
				t.Errorf("expected %d resource in manifest bundle but got: %d", len(c.expected), len(gotObjects))
				return
			}
			for i, expected := range c.expected {
				if !equality.Semantic.DeepDerivative(expected, gotObjects[i]) {
					t.Errorf("expected %#v but got: %#v", expected, gotObjects[i])
				}
			}
		})
	}
}

func TestDecodeBundleStatus(t *testing.T) {
	cases := []struct {
		name             string
		input            datatypes.JSONMap
		expectedJSON     string
		expectedErrorMsg string
	}{
		{
			name:             "empty",
			input:            datatypes.JSONMap{},
			expectedJSON:     "null",
			expectedErrorMsg: "",
		},
		{
			name:             "valid",
			input:            newJSONMap(t, "{\"id\":\"dfaa4da7-915a-4060-962e-4c741c979989\",\"data\":{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestWorkComplete\",\"status\":\"True\",\"message\":\"Apply manifest work complete\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"},{\"type\":\"Available\",\"reason\":\"ResourcesAvailable\",\"status\":\"True\",\"message\":\"All resources are available\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"}],\"resourceStatus\":[{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"message\":\"Apply manifest complete\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"},{\"type\":\"Available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"message\":\"Resource is available\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"},{\"type\":\"StatusFeedbackSynced\",\"reason\":\"NoStatusFeedbackSynced\",\"status\":\"True\",\"message\":\"\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"}],\"resourceMeta\":{\"kind\":\"ConfigMap\",\"name\":\"web\",\"group\":\"\",\"ordinal\":0,\"version\":\"v1\",\"resource\":\"configmaps\",\"namespace\":\"default\"},\"statusFeedback\":{}},{\"conditions\":[{\"type\":\"Applied\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"message\":\"Apply manifest complete\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"},{\"type\":\"Available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"message\":\"Resource is available\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"},{\"type\":\"StatusFeedbackSynced\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"message\":\"\",\"lastTransitionTime\":\"2024-05-21T08:56:35Z\"}],\"resourceMeta\":{\"kind\":\"Deployment\",\"name\":\"web\",\"group\":\"apps\",\"ordinal\":1,\"version\":\"v1\",\"resource\":\"deployments\",\"namespace\":\"default\"},\"statusFeedback\":{\"values\":[{\"name\":\"status\",\"fieldValue\":{\"type\":\"JsonRaw\",\"jsonRaw\":\"{\\\"availableReplicas\\\":2,\\\"conditions\\\":[{\\\"lastTransitionTime\\\":\\\"2024-05-21T08:56:35Z\\\",\\\"lastUpdateTime\\\":\\\"2024-05-21T08:56:38Z\\\",\\\"message\\\":\\\"ReplicaSet \\\\\\\"web-dcffc4f85\\\\\\\" has successfully progressed.\\\",\\\"reason\\\":\\\"NewReplicaSetAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Progressing\\\"},{\\\"lastTransitionTime\\\":\\\"2024-05-21T08:58: 26Z\\\",\\\"lastUpdateTime\\\":\\\"2024-05-21T08:58:26Z\\\",\\\"message\\\":\\\"Deployment has minimum availability.\\\",\\\"reason\\\":\\\"MinimumReplicasAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Available\\\"}],\\\"observedGeneration\\\":2,\\\"readyReplicas\\\":2,\\\"replicas\\\":2,\\\"updatedReplicas\\\":2}\"}}]}}]},\"time\":\"2024-05-21T08:58:31.813194788Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request\",\"source\":\"cluster1-work-agent\",\"resourceid\":\"68ebf474-6709-48bb-b760-386181268064\",\"sequenceid\":\"1792842398301163520\",\"clustername\":\"cluster1\",\"specversion\":\"1.0\",\"originalsource\":\"maestro\",\"datacontenttype\":\"application/json\",\"resourceversion\":\"2\"}"),
			expectedJSON:     "{\"ObservedVersion\":2,\"SequenceID\":\"1792842398301163520\",\"conditions\":[{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"Apply manifest work complete\",\"reason\":\"AppliedManifestWorkComplete\",\"status\":\"True\",\"type\":\"Applied\"},{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"All resources are available\",\"reason\":\"ResourcesAvailable\",\"status\":\"True\",\"type\":\"Available\"}],\"resourceStatus\":[{\"conditions\":[{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"Apply manifest complete\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"type\":\"Applied\"},{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"Resource is available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"\",\"reason\":\"NoStatusFeedbackSynced\",\"status\":\"True\",\"type\":\"StatusFeedbackSynced\"}],\"resourceMeta\":{\"group\":\"\",\"kind\":\"ConfigMap\",\"name\":\"web\",\"namespace\":\"default\",\"ordinal\":0,\"resource\":\"configmaps\",\"version\":\"v1\"},\"statusFeedback\":{}},{\"conditions\":[{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"Apply manifest complete\",\"reason\":\"AppliedManifestComplete\",\"status\":\"True\",\"type\":\"Applied\"},{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"Resource is available\",\"reason\":\"ResourceAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-05-21T08:56:35Z\",\"message\":\"\",\"reason\":\"StatusFeedbackSynced\",\"status\":\"True\",\"type\":\"StatusFeedbackSynced\"}],\"resourceMeta\":{\"group\":\"apps\",\"kind\":\"Deployment\",\"name\":\"web\",\"namespace\":\"default\",\"ordinal\":1,\"resource\":\"deployments\",\"version\":\"v1\"},\"statusFeedback\":{\"values\":[{\"fieldValue\":{\"jsonRaw\":\"{\\\"availableReplicas\\\":2,\\\"conditions\\\":[{\\\"lastTransitionTime\\\":\\\"2024-05-21T08:56:35Z\\\",\\\"lastUpdateTime\\\":\\\"2024-05-21T08:56:38Z\\\",\\\"message\\\":\\\"ReplicaSet \\\\\\\"web-dcffc4f85\\\\\\\" has successfully progressed.\\\",\\\"reason\\\":\\\"NewReplicaSetAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Progressing\\\"},{\\\"lastTransitionTime\\\":\\\"2024-05-21T08:58: 26Z\\\",\\\"lastUpdateTime\\\":\\\"2024-05-21T08:58:26Z\\\",\\\"message\\\":\\\"Deployment has minimum availability.\\\",\\\"reason\\\":\\\"MinimumReplicasAvailable\\\",\\\"status\\\":\\\"True\\\",\\\"type\\\":\\\"Available\\\"}],\\\"observedGeneration\\\":2,\\\"readyReplicas\\\":2,\\\"replicas\\\":2,\\\"updatedReplicas\\\":2}\",\"type\":\"JsonRaw\"},\"name\":\"status\"}]}}]}",
			expectedErrorMsg: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DecodeBundleStatus(c.input)
			if err != nil {
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
				}
				return
			}
			gotBytes, err := json.Marshal(got)
			if err != nil {
				t.Errorf("failed to marshal got resource bundle status: %v", err)
			}

			if string(gotBytes) != c.expectedJSON {
				t.Errorf("expected %s but got: %s", c.expectedJSON, string(gotBytes))
			}
		})
	}

}
