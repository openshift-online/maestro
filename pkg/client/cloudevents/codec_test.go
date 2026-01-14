package cloudevents

import (
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

func TestNewCodec(t *testing.T) {
	sourceID := "test-source-id"
	codec := NewCodec(sourceID)

	if codec == nil {
		t.Fatal("expected codec to be non-nil")
	}

	if codec.sourceID != sourceID {
		t.Errorf("expected sourceID %s but got: %s", sourceID, codec.sourceID)
	}
}

func TestEventDataType(t *testing.T) {
	codec := NewCodec("test-source")
	dataType := codec.EventDataType()

	expectedDataType := workpayload.ManifestBundleEventDataType
	if dataType != expectedDataType {
		t.Errorf("expected data type %s but got: %s", expectedDataType, dataType)
	}
}

func TestEncode(t *testing.T) {
	codec := NewCodec("test-source")
	resourceID := uuid.New().String()
	consumerName := "cluster1"

	cases := []struct {
		name             string
		source           string
		eventType        cetypes.CloudEventsType
		resource         *api.Resource
		validateEvent    func(*testing.T, *cloudevents.Event)
		expectedErrorMsg string
	}{
		{
			name:      "encode resource without metadata",
			source:    "test-source",
			eventType: cetypes.CloudEventsType{CloudEventsDataType: workpayload.ManifestBundleEventDataType, SubResource: cetypes.SubResourceSpec, Action: "create"},
			resource: &api.Resource{
				Meta: api.Meta{
					ID: resourceID,
				},
				Version:      1,
				ConsumerName: consumerName,
				Payload: datatypes.JSONMap{
					"specversion":     "1.0",
					"datacontenttype": "application/json",
					"data": map[string]interface{}{
						"manifests": []interface{}{
							map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      "test-config",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
			validateEvent: func(t *testing.T, evt *cloudevents.Event) {
				if evt.Source() != "test-source" {
					t.Errorf("expected source 'test-source' but got: %s", evt.Source())
				}
				if evt.Type() != "io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create" {
					t.Errorf("unexpected event type: %s", evt.Type())
				}
				ext := evt.Extensions()
				if ext[cetypes.ExtensionResourceID] != resourceID {
					t.Errorf("expected resourceID %s but got: %v", resourceID, ext[cetypes.ExtensionResourceID])
				}
				// Check resource version - CloudEvents stores numeric extensions as int32
				if version, ok := ext[cetypes.ExtensionResourceVersion].(int32); !ok || version != 1 {
					t.Errorf("expected resourceVersion 1 but got: %v (type: %T)", ext[cetypes.ExtensionResourceVersion], ext[cetypes.ExtensionResourceVersion])
				}
				if ext[cetypes.ExtensionClusterName] != consumerName {
					t.Errorf("expected clusterName %s but got: %v", consumerName, ext[cetypes.ExtensionClusterName])
				}
				if _, ok := ext[cetypes.ExtensionDeletionTimestamp]; ok {
					t.Error("expected no deletion timestamp extension")
				}
			},
		},
		{
			name:      "encode resource with metadata",
			source:    "test-source",
			eventType: cetypes.CloudEventsType{CloudEventsDataType: workpayload.ManifestBundleEventDataType, SubResource: cetypes.SubResourceSpec, Action: "create"},
			resource: &api.Resource{
				Meta: api.Meta{
					ID: resourceID,
				},
				Version:      2,
				ConsumerName: consumerName,
				Payload: datatypes.JSONMap{
					"specversion":     "1.0",
					"datacontenttype": "application/json",
					cetypes.ExtensionWorkMeta: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "original-name",
							"namespace": "default",
						},
					},
					"data": map[string]interface{}{
						"manifests": []interface{}{},
					},
				},
			},
			validateEvent: func(t *testing.T, evt *cloudevents.Event) {
				ext := evt.Extensions()
				// Check resource version - CloudEvents stores numeric extensions as int32
				if version, ok := ext[cetypes.ExtensionResourceVersion].(int32); !ok || version != 2 {
					t.Errorf("expected resourceVersion 2 but got: %v (type: %T)", ext[cetypes.ExtensionResourceVersion], ext[cetypes.ExtensionResourceVersion])
				}
				// Verify that metadata name was reset to resource ID
				meta, ok := ext[cetypes.ExtensionWorkMeta].(string)
				if !ok {
					t.Fatalf("expected metadata to be string but got: %T", ext[cetypes.ExtensionWorkMeta])
				}
				if !strings.Contains(meta, resourceID) {
					t.Errorf("expected metadata to contain %s, but got: %v", resourceID, meta)
				}
			},
		},
		{
			name:      "encode resource with deletion timestamp",
			source:    "test-source",
			eventType: cetypes.CloudEventsType{CloudEventsDataType: workpayload.ManifestBundleEventDataType, SubResource: cetypes.SubResourceSpec, Action: "delete"},
			resource: &api.Resource{
				Meta: api.Meta{
					ID:        resourceID,
					DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
				},
				Version:      3,
				ConsumerName: consumerName,
				Payload: datatypes.JSONMap{
					"specversion":     "1.0",
					"datacontenttype": "application/json",
					"data": map[string]interface{}{
						"manifests": []interface{}{},
					},
				},
			},
			validateEvent: func(t *testing.T, evt *cloudevents.Event) {
				ext := evt.Extensions()
				if _, ok := ext[cetypes.ExtensionDeletionTimestamp]; !ok {
					t.Error("expected deletion timestamp extension to be set")
				}
				// Event ID and time should be set for deletion
				if evt.ID() == "" {
					t.Error("expected event ID to be set for deletion")
				}
				if evt.Time().IsZero() {
					t.Error("expected event time to be set for deletion")
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			evt, err := codec.Encode(c.source, c.eventType, c.resource)
			if c.expectedErrorMsg != "" {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if err.Error() != c.expectedErrorMsg {
					t.Errorf("expected error %q but got: %q", c.expectedErrorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if evt == nil {
				t.Fatal("expected event to be non-nil")
			}

			if c.validateEvent != nil {
				c.validateEvent(t, evt)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	consumerName := "cluster1"

	cases := []struct {
		name             string
		event            *cloudevents.Event
		expectedResource *api.Resource
		expectedErrorMsg string
	}{
		{
			name: "decode valid status event",
			event: func() *cloudevents.Event {
				evt := cloudevents.NewEvent()
				evt.SetID(uuid.New().String())
				evt.SetSource("cluster1-work-agent")
				evt.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
				evt.SetExtension(cetypes.ExtensionResourceID, resourceID)
				evt.SetExtension(cetypes.ExtensionResourceVersion, int64(5))
				evt.SetExtension(cetypes.ExtensionClusterName, consumerName)
				evt.SetExtension(cetypes.ExtensionOriginalSource, "maestro")
				evt.SetDataContentType("application/json")
				_ = evt.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Applied",
							"status": "True",
						},
					},
				})
				return &evt
			}(),
			expectedResource: &api.Resource{
				Meta: api.Meta{
					ID: resourceID,
				},
				Version:      5,
				ConsumerName: consumerName,
			},
		},
		{
			name: "decode event with invalid type",
			event: func() *cloudevents.Event {
				evt := cloudevents.NewEvent()
				evt.SetType("invalid-type")
				return &evt
			}(),
			expectedErrorMsg: "failed to parse cloud event type invalid-type",
		},
		{
			name: "decode event with unsupported data type",
			event: func() *cloudevents.Event {
				evt := cloudevents.NewEvent()
				evt.SetType("io.open-cluster-management.works.v1alpha1.other.spec.create")
				return &evt
			}(),
			expectedErrorMsg: "unsupported cloudevents data type io.open-cluster-management.works.v1alpha1.other",
		},
		{
			name: "decode event with missing resourceID extension",
			event: func() *cloudevents.Event {
				evt := cloudevents.NewEvent()
				evt.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
				evt.SetExtension(cetypes.ExtensionResourceVersion, int64(1))
				evt.SetExtension(cetypes.ExtensionClusterName, consumerName)
				evt.SetExtension(cetypes.ExtensionOriginalSource, "maestro")
				return &evt
			}(),
			expectedErrorMsg: "failed to get resourceid extension",
		},
		{
			name: "decode event with unmatched source ID",
			event: func() *cloudevents.Event {
				evt := cloudevents.NewEvent()
				evt.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
				evt.SetExtension(cetypes.ExtensionResourceID, resourceID)
				evt.SetExtension(cetypes.ExtensionResourceVersion, int64(1))
				evt.SetExtension(cetypes.ExtensionClusterName, consumerName)
				evt.SetExtension(cetypes.ExtensionOriginalSource, "different-source")
				evt.SetDataContentType("application/json")
				_ = evt.SetData(cloudevents.ApplicationJSON, map[string]interface{}{})
				return &evt
			}(),
			expectedErrorMsg: "unmatched original source id different-source for resource " + resourceID,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resource, err := codec.Decode(c.event)

			if c.expectedErrorMsg != "" {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				// Use Contains check for error messages since some include dynamic IDs
				if len(c.expectedErrorMsg) > 0 && len(err.Error()) >= len(c.expectedErrorMsg) {
					if err.Error()[:len(c.expectedErrorMsg)] != c.expectedErrorMsg {
						t.Errorf("expected error to start with %q but got: %q", c.expectedErrorMsg, err.Error())
					}
				} else {
					t.Errorf("expected error %q but got: %q", c.expectedErrorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resource == nil {
				t.Fatal("expected resource to be non-nil")
			}

			if c.expectedResource != nil {
				if resource.ID != c.expectedResource.ID {
					t.Errorf("expected ID %s but got: %s", c.expectedResource.ID, resource.ID)
				}
				if resource.Version != c.expectedResource.Version {
					t.Errorf("expected Version %d but got: %d", c.expectedResource.Version, resource.Version)
				}
				if resource.ConsumerName != c.expectedResource.ConsumerName {
					t.Errorf("expected ConsumerName %s but got: %s", c.expectedResource.ConsumerName, resource.ConsumerName)
				}
				if resource.Status == nil {
					t.Error("expected Status to be non-nil")
				}
			}
		})
	}
}

func TestResetPayloadMetadataNameWithResID(t *testing.T) {
	resourceID := uuid.New().String()

	cases := []struct {
		name         string
		resource     *api.Resource
		validateFunc func(*testing.T, *api.Resource)
	}{
		{
			name: "resource without metadata",
			resource: &api.Resource{
				Meta: api.Meta{
					ID: resourceID,
				},
				Payload: datatypes.JSONMap{
					"data": map[string]interface{}{
						"manifests": []interface{}{},
					},
				},
			},
			validateFunc: func(t *testing.T, res *api.Resource) {
				// Should not have metadata after reset
				if _, ok := res.Payload[cetypes.ExtensionWorkMeta]; ok {
					t.Error("expected no metadata in payload")
				}
			},
		},
		{
			name: "resource with metadata object",
			resource: &api.Resource{
				Meta: api.Meta{
					ID: resourceID,
				},
				Payload: datatypes.JSONMap{
					cetypes.ExtensionWorkMeta: map[string]interface{}{
						"name":      "original-name",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"manifests": []interface{}{},
					},
				},
			},
			validateFunc: func(t *testing.T, res *api.Resource) {
				meta, ok := res.Payload[cetypes.ExtensionWorkMeta]
				if !ok {
					t.Fatal("expected metadata in payload")
				}
				metaMap, ok := meta.(map[string]interface{})
				if !ok {
					t.Fatal("expected metadata to be map[string]interface{}")
				}
				gotName, ok := metaMap["name"].(string)
				if !ok {
					t.Fatalf("expected metadata name to be string but got: %T", metaMap["name"])
				}
				if gotName != resourceID {
					t.Errorf("expected metadata name to be %q but got: %q", resourceID, gotName)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := resetPayloadMetadataNameWithResID(c.resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.validateFunc != nil {
				c.validateFunc(t, c.resource)
			}
		})
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()

	originalResource := &api.Resource{
		Meta: api.Meta{
			ID: resourceID,
		},
		Version:      10,
		ConsumerName: "cluster1",
		Payload: datatypes.JSONMap{
			"specversion":     "1.0",
			"datacontenttype": "application/json",
			"data": map[string]interface{}{
				"manifests": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test",
							"namespace": "default",
						},
					},
				},
			},
		},
	}

	// Encode
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}
	evt, err := codec.Encode("test-source", eventType, originalResource)
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	// Modify event to simulate a status update response
	evt.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
	evt.SetExtension(cetypes.ExtensionOriginalSource, "maestro")
	_ = evt.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Applied",
				"status": "True",
			},
		},
	})

	// Decode
	decodedResource, err := codec.Decode(evt)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Verify
	if decodedResource.ID != originalResource.ID {
		t.Errorf("expected ID %s but got: %s", originalResource.ID, decodedResource.ID)
	}
	if decodedResource.Version != originalResource.Version {
		t.Errorf("expected Version %d but got: %d", originalResource.Version, decodedResource.Version)
	}
	if decodedResource.ConsumerName != originalResource.ConsumerName {
		t.Errorf("expected ConsumerName %s but got: %s", originalResource.ConsumerName, decodedResource.ConsumerName)
	}
	if decodedResource.Status == nil {
		t.Error("expected Status to be non-nil")
	}
}
