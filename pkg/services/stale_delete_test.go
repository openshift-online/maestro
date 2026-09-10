package services

import (
	"context"
	"testing"

	gm "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
	"github.com/openshift-online/maestro/pkg/errors"
)

// fakeStatusEventService is a minimal in-memory StatusEventService double, only
// implementing what FinalizeStaleDelete needs (Create), for asserting the synthetic
// delete status event it sends.
type fakeStatusEventService struct {
	created  []*api.StatusEvent
	failNext bool
}

func (f *fakeStatusEventService) Get(ctx context.Context, id string) (*api.StatusEvent, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) Create(ctx context.Context, event *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError) {
	if f.failNext {
		f.failNext = false
		return nil, errors.GeneralError("simulated status event create failure")
	}
	f.created = append(f.created, event)
	return event, nil
}

func (f *fakeStatusEventService) Replace(ctx context.Context, event *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) Delete(ctx context.Context, id string) *errors.ServiceError {
	return errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) All(ctx context.Context) (api.StatusEventList, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) FindByIDs(ctx context.Context, ids []string) (api.StatusEventList, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) FindAllUnreconciledEvents(ctx context.Context) (api.StatusEventList, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) DeleteAllReconciledEvents(ctx context.Context) *errors.ServiceError {
	return errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) DeleteAllEvents(ctx context.Context, eventIDs []string) *errors.ServiceError {
	return errors.NotImplemented("StatusEvent")
}

func (f *fakeStatusEventService) GetNotificationQueueUsage(ctx context.Context) (*float64, *errors.ServiceError) {
	return nil, errors.NotImplemented("StatusEvent")
}

var _ StatusEventService = &fakeStatusEventService{}

// failNDeletesResourceService wraps a real ResourceService, failing the first N calls to
// Delete before delegating, to simulate a transient hard-delete failure.
type failNDeletesResourceService struct {
	ResourceService
	remainingFailures int
}

func (f *failNDeletesResourceService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if f.remainingFailures > 0 {
		f.remainingFailures--
		return errors.GeneralError("simulated transient delete failure")
	}
	return f.ResourceService.Delete(ctx, id)
}

// TestFinalizeStaleDeleteHardDeletesAndNotifies ensures FinalizeStaleDelete sends a
// synthetic status event carrying a ResourceDeleted condition (the same signal a real
// agent ack sends), then hard-deletes the resource so it is gone even from an Unscoped
// lookup.
func TestFinalizeStaleDeleteHardDeletesAndNotifies(t *testing.T) {
	gm.RegisterTestingT(t)

	ctx := context.Background()
	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil, 60)
	statusEvents := &fakeStatusEventService{}

	resource, svcErr := resourceService.Create(ctx, &api.Resource{
		ConsumerName: Fukuisaurus,
		Source:       "grpc",
		Payload:      newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}}]}}"),
	})
	gm.Expect(svcErr).To(gm.BeNil())

	gm.Expect(resourceService.MarkAsDeleting(ctx, resource.ID)).To(gm.BeNil())

	found, err := resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).To(gm.BeNil())

	gm.Expect(FinalizeStaleDelete(ctx, found, resourceService, statusEvents)).To(gm.BeNil())

	// the resource is now hard-deleted
	_, err = resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).ShouldNot(gm.BeNil())

	// a single synthetic delete status event was sent, carrying a ResourceDeleted condition
	gm.Expect(statusEvents.created).To(gm.HaveLen(1))
	sent := statusEvents.created[0]
	gm.Expect(sent.ResourceID).To(gm.Equal(resource.ID))
	gm.Expect(sent.StatusEventType).To(gm.Equal(api.StatusDeleteEventType))

	statusEvt, err := api.JSONMAPToCloudEvent(sent.Status)
	gm.Expect(err).To(gm.BeNil())
	statusPayload := &workpayload.ManifestBundleStatus{}
	gm.Expect(statusEvt.DataAs(statusPayload)).To(gm.BeNil())
	gm.Expect(meta.IsStatusConditionTrue(statusPayload.Conditions, common.ResourceDeleted)).To(gm.BeTrue())
}

// TestFinalizeStaleDeleteNotifyFailureAfterSuccessfulDeleteIsNotRetried documents that once
// the hard-delete succeeds, a failure sending the synthetic status event is surfaced as an
// error but is never retried (the resource is already gone, so FindStaleDeleting can't find
// it again): retrying here would risk inserting a duplicate terminal event instead, since
// StatusEvent.BeforeCreate mints a fresh ID on every call.
func TestFinalizeStaleDeleteNotifyFailureAfterSuccessfulDeleteIsNotRetried(t *testing.T) {
	gm.RegisterTestingT(t)

	ctx := context.Background()
	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil, 60)
	statusEvents := &fakeStatusEventService{failNext: true}

	resource, svcErr := resourceService.Create(ctx, &api.Resource{
		ConsumerName: Fukuisaurus,
		Source:       "grpc",
		Payload:      newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}}]}}"),
	})
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(resourceService.MarkAsDeleting(ctx, resource.ID)).To(gm.BeNil())

	found, err := resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).To(gm.BeNil())

	gm.Expect(FinalizeStaleDelete(ctx, found, resourceService, statusEvents)).ShouldNot(gm.BeNil())

	// the resource is gone even though the notification failed
	_, err = resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).ShouldNot(gm.BeNil())
	gm.Expect(statusEvents.created).To(gm.BeEmpty())

	// a later detector tick can no longer find this resource to retry the notification
	stillStale, svcErr := resourceService.FindStaleDeleting(ctx, 0)
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(stillStale).To(gm.BeEmpty())
}

// TestFinalizeStaleDeleteRetryAfterDeleteFailureDoesNotDuplicateNotification ensures that
// when the hard-delete step fails, no status event is sent, and a subsequent successful
// retry sends exactly one - never a duplicate terminal delete notification per retry.
func TestFinalizeStaleDeleteRetryAfterDeleteFailureDoesNotDuplicateNotification(t *testing.T) {
	gm.RegisterTestingT(t)

	ctx := context.Background()
	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil, 60)
	flakyResourceService := &failNDeletesResourceService{ResourceService: resourceService, remainingFailures: 1}
	statusEvents := &fakeStatusEventService{}

	resource, svcErr := resourceService.Create(ctx, &api.Resource{
		ConsumerName: Fukuisaurus,
		Source:       "grpc",
		Payload:      newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}}]}}"),
	})
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(resourceService.MarkAsDeleting(ctx, resource.ID)).To(gm.BeNil())

	found, err := resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).To(gm.BeNil())

	// first attempt: the hard-delete fails, so no notification must be sent, and the
	// resource must remain in place for the next detector tick to retry
	gm.Expect(FinalizeStaleDelete(ctx, found, flakyResourceService, statusEvents)).ShouldNot(gm.BeNil())
	gm.Expect(statusEvents.created).To(gm.BeEmpty())
	_, err = resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).To(gm.BeNil())

	// retry: hard-delete now succeeds, exactly one notification is sent
	gm.Expect(FinalizeStaleDelete(ctx, found, flakyResourceService, statusEvents)).To(gm.BeNil())
	gm.Expect(statusEvents.created).To(gm.HaveLen(1))
	_, err = resourceDAO.Get(ctx, resource.ID)
	gm.Expect(err).ShouldNot(gm.BeNil())
}
