package services

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

// StaleDeleteReason is the condition reason set on the synthetic status event created by
// FinalizeStaleDelete, so it is distinguishable in status history from a real agent ack.
const StaleDeleteReason = "StaleDeleteAssumedComplete"

// staleDeleteNotifyFailedTotal counts resources that were successfully hard-deleted by
// FinalizeStaleDelete but whose synthetic delete status event failed to send. This gap is
// deliberately not retried (see FinalizeStaleDelete), so it does not self-heal: an operator
// needs to notice and, if needed, replay the notification for that resource ID.
var staleDeleteNotifyFailedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Subsystem: metricsSubsystem,
		Name:      "stale_delete_notify_failed_total",
		Help:      "Total number of resources hard-deleted by the stale-delete detector for which sending the synthetic delete status event failed; the resource is gone but watchers may not have been notified and this will not self-heal",
	},
)

func init() {
	prometheus.MustRegister(staleDeleteNotifyFailedTotal)
}

// FinalizeStaleDelete assumes the delete of a resource that has been soft-deleted for
// longer than the hard-delete threshold has succeeded, even though its agent never
// acknowledged it. It synthesizes a status event carrying a ResourceDeleted condition
// (the same condition a real agent ack would carry), matching the handling in
// cmd/maestro/server/event_server.go's HandleStatusUpdate for a genuine ack, then
// hard-deletes the resource so watchers (e.g. Clusters Service) stop retrying.
//
// This is a deliberate trade-off: after the hard-delete threshold, Maestro prioritizes
// unblocking the stuck-delete loop over waiting indefinitely for an agent that may be
// permanently gone. The threshold must be long enough that a still-connected agent would
// already have acknowledged the delete.
func FinalizeStaleDelete(ctx context.Context, found *api.Resource, resourceService ResourceService, statusEventService StatusEventService) *errors.ServiceError {
	logger := klog.FromContext(ctx).WithValues("resourceID", found.ID)

	// Hard-delete before notifying, and in a separate step from building/sending the
	// status event. statusEventService.Create and resourceService.Delete are independent
	// writes (no shared transaction), and buildSyntheticDeletedStatus mints a fresh event
	// each call with no dedup key. Deleting first makes a retry after a mid-way failure
	// safe: if Delete fails, nothing has been sent yet and the next detector tick retries
	// cleanly; once Delete succeeds, the resource can never be found by FindStaleDeleting
	// again, so the notification below can run at most once.
	//
	// That also means it must not be retried here: StatusEvent.BeforeCreate mints a fresh
	// ID on every call, and the DAO's Create can durably persist the row before its
	// pg_notify fails, so blindly retrying the same call on error risks inserting a
	// duplicate terminal event rather than fixing anything. Instead, a failure here is
	// surfaced as a dedicated metric for an operator to act on, since it will not self-heal.
	if svcErr := resourceService.Delete(ctx, found.ID); svcErr != nil {
		return svcErr
	}

	status, err := buildSyntheticDeletedStatus(found.ID)
	if err != nil {
		staleDeleteNotifyFailedTotal.Inc()
		return errors.GeneralError("resource %s was hard-deleted but building its synthetic deleted status failed, the delete notification was not sent: %s", found.ID, err)
	}

	if _, sErr := statusEventService.Create(ctx, &api.StatusEvent{
		ResourceID:      found.ID,
		ResourceSource:  found.Source,
		ResourceType:    found.Type,
		Payload:         found.Payload,
		Status:          status,
		StatusEventType: api.StatusDeleteEventType,
	}); sErr != nil {
		staleDeleteNotifyFailedTotal.Inc()
		return errors.GeneralError("resource %s was hard-deleted but sending its synthetic delete status event failed: %s", found.ID, sErr.Error())
	}

	logger.Info("hard-deleted resource stuck soft-deleted past the hard-delete threshold and sent a synthetic delete status event")

	return nil
}

// buildSyntheticDeletedStatus builds a resource status JSONMap carrying a ResourceDeleted
// condition, shaped the same way a real agent status ack would be, so it round-trips
// through the same CloudEvent (de)serialization as genuine status updates.
func buildSyntheticDeletedStatus(resourceID string) (map[string]interface{}, error) {
	evt := cloudevents.NewEvent()
	evt.SetID(uuid.New().String())
	evt.SetSource("maestro")
	evt.SetTime(time.Now())
	evt.SetType((&cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.UpdateRequestAction,
	}).String())
	evt.SetExtension(cetypes.ExtensionResourceID, resourceID)

	status := &workpayload.ManifestBundleStatus{}
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:    common.ResourceDeleted,
		Status:  metav1.ConditionTrue,
		Reason:  StaleDeleteReason,
		Message: fmt.Sprintf("Maestro assumed resource %s was deleted after it remained soft-deleted past the hard-delete threshold with no acknowledgment from its agent.", resourceID),
	})

	if err := evt.SetData(cloudevents.ApplicationJSON, status); err != nil {
		return nil, err
	}

	return api.CloudEventToJSONMap(&evt)
}
