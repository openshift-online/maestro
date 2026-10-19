package services

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	sourcecodec "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/source/codec"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

// ConsumerCascadeReason distinguishes operator-requested finalization from an agent acknowledgement.
const ConsumerCascadeReason = "ConsumerDeletionCascade"

var consumerCascadedResourcesTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Subsystem: "consumer_delete",
	Name:      "cascaded_resources_total",
	Help:      "Resources finalized in committed explicit consumer deletion cascades",
})

// init registers the bounded-cardinality counter for committed cascade finalizations.
func init() {
	prometheus.MustRegister(consumerCascadedResourcesTotal)
}

// buildConsumerCascadeEvent preserves the deleted resource's identity and payload in its durable terminal event.
func buildConsumerCascadeEvent(resource *api.Resource) (*api.StatusEvent, error) {
	status, err := buildConsumerDeletedStatus(resource)
	if err != nil {
		return nil, err
	}
	return &api.StatusEvent{
		ResourceID:      resource.ID,
		ResourceSource:  resource.Source,
		ResourceType:    resource.Type,
		Payload:         resource.Payload,
		Status:          status,
		StatusEventType: api.StatusDeleteEventType,
	}, nil
}

// buildConsumerDeletedStatus preserves the source identity extensions required by
// SDK work watchers, including work metadata and the original resource generation.
func buildConsumerDeletedStatus(resource *api.Resource) (map[string]interface{}, error) {
	spec, err := api.JSONMAPToCloudEvent(resource.Payload)
	if err != nil {
		return nil, err
	}
	// Validate the payload that the gRPC broadcaster later restores into the
	// terminal status, before any row can be committed as deleted.
	if err := spec.DataAs(&workpayload.ManifestBundle{}); err != nil {
		return nil, fmt.Errorf("cannot decode resource spec for cascade: %w", err)
	}
	evt := cloudevents.NewEvent()
	evt.SetID(uuid.NewString())
	evt.SetSource("maestro")
	evt.SetTime(time.Now())
	evt.SetType((&cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.UpdateRequestAction,
	}).String())
	evt.SetExtension(cetypes.ExtensionResourceID, resource.ID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, int64(resource.Version))
	evt.SetExtension(cetypes.ExtensionClusterName, resource.ConsumerName)
	evt.SetExtension(cetypes.ExtensionOriginalSource, resource.Source)
	// Deleted conditions bypass sequence ordering in source stores. Still provide
	// a valid sequence string, required by the SDK's status decoder.
	evt.SetExtension(cetypes.ExtensionStatusUpdateSequenceID, "0")
	if workMeta, ok := spec.Extensions()[cetypes.ExtensionWorkMeta]; ok {
		evt.SetExtension(cetypes.ExtensionWorkMeta, workMeta)
	}
	status := &workpayload.ManifestBundleStatus{}
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               common.ResourceDeleted,
		Status:             metav1.ConditionTrue,
		Reason:             ConsumerCascadeReason,
		Message:            fmt.Sprintf("Resource %s finalized by explicit deletion of its decommissioned consumer.", resource.ID),
		ObservedGeneration: int64(resource.Version),
	})
	if err := evt.SetData(cloudevents.ApplicationJSON, status); err != nil {
		return nil, err
	}
	if _, err := sourcecodec.NewManifestBundleCodec().Decode(&evt); err != nil {
		return nil, fmt.Errorf("cannot decode cascade status for source watchers: %w", err)
	}
	return api.CloudEventToJSONMap(&evt)
}
