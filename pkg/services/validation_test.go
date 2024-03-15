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

func TestValidateNewManifest(t *testing.T) {
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
			err := ValidateManifest(c.manifest)
			if err != nil && err.Error() != c.expectedErrorMsg {
				t.Errorf("expected %#v but got: %#v", c.expectedErrorMsg, err)
			}
		})
	}
}

func TestValidateUpdateManifest(t *testing.T) {
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
			err := ValidateManifestUpdate(c.newManifest, c.oldManifest)
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
