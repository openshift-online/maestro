package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-online/maestro/pkg/api"
	"gorm.io/datatypes"
)

func TestValidateConsumer(t *testing.T) {
	cases := []struct {
		name             string
		consumer         *api.Consumer
		expectedErrorMsg string
	}{
		{
			name: "validated",
			consumer: &api.Consumer{
				Name: "test",
			},
		},
		{
			name: "wrong name",
			consumer: &api.Consumer{
				Name: "_",
			},
			expectedErrorMsg: "consumer.name: Invalid value: \"_\": a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')",
		},
		{
			name: "max length",
			consumer: &api.Consumer{
				Name: maxName(),
			},
			expectedErrorMsg: fmt.Sprintf("consumer.name: Invalid value: \"%s\": must be no more than 63 characters", maxName()),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConsumer(c.consumer)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateResourceName(t *testing.T) {
	cases := []struct {
		name             string
		resource         *api.Resource
		expectedErrorMsg string
	}{
		{
			name: "validated",
			resource: &api.Resource{
				Name: "test",
			},
		},
		{
			name: "wrong name",
			resource: &api.Resource{
				Name: "_",
			},
			expectedErrorMsg: "resource.name: Invalid value: \"_\": a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')",
		},
		{
			name: "max length",
			resource: &api.Resource{
				Name: maxName(),
			},
			expectedErrorMsg: fmt.Sprintf("resource.name: Invalid value: \"%s\": must be no more than 63 characters", maxName()),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateResourceName(c.resource)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateNewManifest(t *testing.T) {
	cases := []struct {
		name             string
		resType          api.ResourceType
		manifest         datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:     "validated single manifest",
			resType:  api.ResourceTypeSingle,
			manifest: newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
		},
		{
			name:     "validated bundle manifest",
			resType:  api.ResourceTypeBundle,
			manifest: newManifest(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
		},
		{
			name:             "invalidated single manifest",
			resType:          api.ResourceTypeSingle,
			manifest:         newManifest(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
			expectedErrorMsg: "manifest is empty",
		},
		{
			name:             "invalidated bundle manifest",
			resType:          api.ResourceTypeBundle,
			manifest:         newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expectedErrorMsg: "manifest bundle is empty",
		},
		{
			name:             "invalidated resource type",
			resType:          "invalid",
			manifest:         newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expectedErrorMsg: "unknown resource type: invalid",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateManifest(c.resType, c.manifest)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateNewObject(t *testing.T) {
	cases := []struct {
		name             string
		manifest         datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:     "validated",
			manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
		},
		{
			name:             "no apiVersion",
			manifest:         newManifest(t, "{\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "apiVersion: Required value: field not set",
		},
		{
			name:             "no kind",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "kind: Required value: field not set",
		},
		{
			name:             "no name",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"namespace\":\"test\"}}"),
			expectedErrorMsg: "metadata.name: Required value: field not set",
		},
		{
			name:             "has generate name",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"generateName\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "metadata.generateName: Forbidden: field cannot be set",
		},
		{
			name:             "has resource version",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"resourceVersion\":\"123\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "metadata.resourceVersion: Forbidden: field cannot be set",
		},
		{
			name:             "has deletion grace period seconds",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"deletionGracePeriodSeconds\":10,\"namespace\":\"test\"}}"),
			expectedErrorMsg: "metadata.deletionGracePeriodSeconds: Forbidden: field cannot be set",
		},
		{
			name:             "wrong namespace",
			manifest:         newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"_\"}}"),
			expectedErrorMsg: "metadata.namespace: Invalid value: \"_\": a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')",
		},
		{
			name:             "wrong api version (no version)",
			manifest:         newManifest(t, "{\"apiVersion\":\"apps/\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "apiVersion: Invalid value: \"apps/\": version not set",
		},
		{
			name:             "wrong api version (no version)",
			manifest:         newManifest(t, "{\"apiVersion\":\"/v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "apiVersion: Invalid value: \"/v1\": group not set",
		},
		{
			name:             "wrong api version",
			manifest:         newManifest(t, "{\"apiVersion\":\"apps/v1/test\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "apiVersion: Invalid value: \"apps/v1/test\": bad format",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateObject(c.manifest)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateUpdateManifest(t *testing.T) {
	cases := []struct {
		name             string
		resType          api.ResourceType
		newManifest      datatypes.JSONMap
		oldManifest      datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:        "validated single manifest",
			resType:     api.ResourceTypeSingle,
			newManifest: newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			oldManifest: newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
		},
		{
			name:        "validated bundle manifest",
			resType:     api.ResourceTypeBundle,
			newManifest: newManifest(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
			oldManifest: newManifest(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
		},
		{
			name:             "invalidated single manifest",
			resType:          api.ResourceTypeSingle,
			newManifest:      newManifest(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}"),
			oldManifest:      newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expectedErrorMsg: "new or old manifest is empty",
		},
		{
			name:             "invalidated bundle manifest",
			resType:          api.ResourceTypeBundle,
			newManifest:      newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			oldManifest:      newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expectedErrorMsg: "new or old manifest bundle is empty",
		},
		{
			name:             "invalidated resource type",
			resType:          "invalid",
			newManifest:      newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			oldManifest:      newManifest(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifests.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifest\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}}}"),
			expectedErrorMsg: "unknown resource type: invalid",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateManifestUpdate(c.resType, c.newManifest, c.oldManifest)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateUpdateObject(t *testing.T) {
	cases := []struct {
		name             string
		newManifest      datatypes.JSONMap
		oldManifest      datatypes.JSONMap
		expectedErrorMsg string
	}{
		{
			name:        "validated",
			newManifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			oldManifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
		},
		{
			name:             "apiVersion mismatch",
			newManifest:      newManifest(t, "{\"apiVersion\":\"v2\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			oldManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "apiVersion: Invalid value: \"v2\": field is immutable",
		},
		{
			name:             "kind mismatch",
			newManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"Test\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			oldManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "kind: Invalid value: \"Test\": field is immutable",
		},
		{
			name:             "name mismatch",
			newManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test2\",\"namespace\":\"test\"}}"),
			oldManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test1\",\"namespace\":\"test\"}}"),
			expectedErrorMsg: "metadata.name: Invalid value: \"test2\": field is immutable",
		},
		{
			name:             "namespace mismatch",
			newManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test2\"}}"),
			oldManifest:      newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test1\"}}"),
			expectedErrorMsg: "metadata.namespace: Invalid value: \"test2\": field is immutable",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateObjectUpdate(c.newManifest, c.oldManifest)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func newManifest(t *testing.T, data string) datatypes.JSONMap {
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data), &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}

func maxName() string {
	n := []string{}
	for i := 0; i < 64; i++ {
		n = append(n, "a")
	}
	return strings.Join(n, "")
}
