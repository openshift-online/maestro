package services

import (
	"context"
	"fmt"
	"reflect"
	"time"

	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	logger "github.com/openshift-online/maestro/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"

	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

func init() {
	// Register the metrics for resource service
	RegisterResourceMetrics()
}

type ResourceService interface {
	Get(ctx context.Context, id string) (*api.Resource, *errors.ServiceError)
	Create(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError)
	Update(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError)
	UpdateStatus(ctx context.Context, resource *api.Resource) (*api.Resource, bool, *errors.ServiceError)
	MarkAsDeleting(ctx context.Context, id string) *errors.ServiceError
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.ResourceList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (api.ResourceList, *errors.ServiceError)
	FindBySource(ctx context.Context, source string) (api.ResourceList, *errors.ServiceError)
	List(listOpts cetypes.ListOptions) ([]*api.Resource, error)
	ListWithArgs(ctx context.Context, username string, args *ListArguments, resources *[]api.Resource) (*api.PagingMeta, *errors.ServiceError)
}

func NewResourceService(lockFactory db.LockFactory, resourceDao dao.ResourceDao, events EventService, generic GenericService) ResourceService {
	return &sqlResourceService{
		lockFactory: lockFactory,
		resourceDao: resourceDao,
		events:      events,
		generic:     generic,
	}
}

var _ ResourceService = &sqlResourceService{}

type sqlResourceService struct {
	lockFactory db.LockFactory
	resourceDao dao.ResourceDao
	events      EventService
	generic     GenericService
}

func (s *sqlResourceService) Get(ctx context.Context, id string) (*api.Resource, *errors.ServiceError) {
	resource, err := s.resourceDao.Get(ctx, id)
	if err != nil {
		return nil, handleGetError("Resource", "id", id, err)
	}

	// sync the creationTimestamp and deletionTimestamp from resource meta to work metadata
	s.syncTimestampsFromResourceMeta(resource)

	return resource, nil
}

func (s *sqlResourceService) Create(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError) {
	if resource.Name != "" {
		if err := ValidateResourceName(resource); err != nil {
			return nil, errors.Validation("the name in the resource is invalid, %v", err)
		}
	}
	if err := ValidateManifest(resource.Type, resource.Payload); err != nil {
		return nil, errors.Validation("the manifest in the resource is invalid, %v", err)
	}

	resource, err := s.resourceDao.Create(ctx, resource)
	if err != nil {
		return nil, handleCreateError("Resource", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  resource.ID,
		EventType: api.CreateEventType,
	})
	if eErr != nil {
		return nil, handleCreateError("Resource", err)
	}

	return resource, nil
}

func (s *sqlResourceService) Update(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError) {
	// Updates the resource manifest only when its manifest changes.
	// If there are multiple requests at the same time, it will cause the race conditions among these
	// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, resource.ID, db.Resources)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}

	found, err := s.resourceDao.Get(ctx, resource.ID)
	if err != nil {
		return nil, handleGetError("Resource", "id", resource.ID, err)
	}

	if !found.DeletedAt.Time.IsZero() {
		return nil, errors.Conflict("the resource is under deletion, id: %s", resource.ID)
	}

	// Make sure the requested resource version is consistent with its database version.
	if found.Version != resource.Version {
		return nil, errors.Conflict("the resource version is not the latest, the latest version: %d", found.Version)
	}

	// New manifest is not changed, the update action is not needed.
	if reflect.DeepEqual(resource.Payload, found.Payload) {
		return found, nil
	}

	if err := ValidateManifestUpdate(resource.Type, resource.Payload, found.Payload); err != nil {
		return nil, errors.Validation("the new manifest in the resource is invalid, %v", err)
	}

	// Increase the current resource version and update its manifest.
	found.Version = found.Version + 1
	found.Payload = resource.Payload

	updated, err := s.resourceDao.Update(ctx, found)
	if err != nil {
		return nil, handleUpdateError("Resource", err)
	}

	if _, err := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  updated.ID,
		EventType: api.UpdateEventType,
	}); err != nil {
		return nil, handleUpdateError("Resource", err)
	}

	// Create the set of labels that we will add to all the resource process:
	labels := prometheus.Labels{
		metricsIDLabel:     updated.ID,
		metricsActionLabel: "update",
	}

	// Update the metric containing the number of processed resources:
	resourceProcessedCountMetric.With(labels).Inc()

	return updated, nil
}

func (s *sqlResourceService) UpdateStatus(ctx context.Context, resource *api.Resource) (*api.Resource, bool, *errors.ServiceError) {
	logger := logger.NewOCMLogger(ctx)
	// Updates the resource status only when its status changes.
	// If there are multiple requests at the same time, it will cause the race conditions among these
	// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, resource.ID, db.ResourceStatus)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		return nil, false, errors.DatabaseAdvisoryLock(err)
	}

	found, err := s.resourceDao.Get(ctx, resource.ID)
	if err != nil {
		return nil, false, handleGetError("Resource", "id", resource.ID, err)
	}

	// Make sure the requested resource version is consistent with its database version.
	if found.Version != resource.Version {
		logger.Warning(fmt.Sprintf("Updating status for stale resource; disregard it: id=%s, foundVersion=%d, wantedVersion=%d",
			resource.ID, found.Version, resource.Version))
		return found, false, nil
	}

	// New status is not changed, the update status action is not needed.
	if reflect.DeepEqual(resource.Status, found.Status) {
		return found, false, nil
	}

	resourceStatusEvent, err := api.JSONMAPToCloudEvent(resource.Status)
	if err != nil {
		return nil, false, errors.GeneralError("Unable to convert resource status to cloudevent: %s", err)
	}

	logger.V(4).Info(fmt.Sprintf("Updating resource status with event %s", resourceStatusEvent))

	sequenceID, err := cloudeventstypes.ToString(resourceStatusEvent.Context.GetExtensions()[cetypes.ExtensionStatusUpdateSequenceID])
	if err != nil {
		return nil, false, errors.GeneralError("Unable to get sequence ID from resource status: %s", err)
	}

	foundSequenceID := ""
	if len(found.Status) != 0 {
		foundStatusEvent, err := api.JSONMAPToCloudEvent(found.Status)
		if err != nil {
			return nil, false, errors.GeneralError("Unable to convert resource status to cloudevent: %s", err)
		}

		foundSequenceID, err = cloudeventstypes.ToString(foundStatusEvent.Context.GetExtensions()[cetypes.ExtensionStatusUpdateSequenceID])
		if err != nil {
			return nil, false, errors.GeneralError("Unable to get sequence ID from found resource status: %s", err)
		}
	}

	newer, err := compareSequenceIDs(sequenceID, foundSequenceID)
	if err != nil {
		return nil, false, errors.GeneralError("Unable to compare sequence IDs: %s", err)
	}
	if !newer {
		logger.Warning(fmt.Sprintf("Updating status for stale resource; disregard it: id=%s, foundSequenceID=%s, wantedSequenceID=%s",
			resource.ID, foundSequenceID, sequenceID))
		return found, false, nil
	}

	found.Status = resource.Status
	updated, err := s.resourceDao.Update(ctx, found)
	if err != nil {
		return nil, false, handleUpdateError("Resource", err)
	}

	// Create the set of labels that we will add to all the resource process:
	labels := prometheus.Labels{
		metricsIDLabel:     updated.ID,
		metricsActionLabel: "update",
	}

	// Update the metric containing the number of processed resources:
	resourceProcessedCountMetric.With(labels).Inc()

	return updated, true, nil
}

// MarkAsDeleting marks the resource as deleting by setting the delete_at timestamp.
// The Resource Deletion Flow:
// 1. User requests deletion
// 2. Maestro marks resource as deleting by soft delete, adds delete event to DB
// 3. Maestro handles delete event and sends CloudEvent to work-agent
// 4. Work-agent deletes resource, sends CloudEvent back to Maestro
// 5. Maestro hard deletes resource from DB
func (s *sqlResourceService) MarkAsDeleting(ctx context.Context, id string) *errors.ServiceError {
	// If there are multiple requests to write the resource at the same time, it will cause the race conditions among these
	// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, id, db.Resources)
	// Ensure that the transaction related to this lock always end.
	defer s.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		return errors.DatabaseAdvisoryLock(err)
	}

	if err := s.resourceDao.Delete(ctx, id, false); err != nil {
		return handleDeleteError("Resource", errors.GeneralError("Unable to delete resource: %s", err))
	}

	if _, err := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  id,
		EventType: api.DeleteEventType,
	}); err != nil {
		return handleDeleteError("Resource", err)
	}

	return nil
}

func (s *sqlResourceService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.resourceDao.Delete(ctx, id, true); err != nil {
		return handleDeleteError("Resource", errors.GeneralError("Unable to delete resource: %s", err))
	}

	return nil
}

func (s *sqlResourceService) FindByIDs(ctx context.Context, ids []string) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, handleGetError("Resource", "id", ids, err)
	}
	return resources, nil
}

func (s *sqlResourceService) FindBySource(ctx context.Context, source string) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.FindBySource(ctx, source)
	if err != nil {
		return nil, handleGetError("Resource", "source", source, err)
	}
	return resources, nil
}

func (s *sqlResourceService) All(ctx context.Context) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all resources: %s", err)
	}
	return resources, nil
}

var _ cegeneric.Lister[*api.Resource] = &sqlResourceService{}

// List implements the cegeneric.Lister interface, enabling the cloudevents source client to list resources for responding spec resync from the agent.
// For more details, refer to the cegeneric.Lister interface:
// https://github.com/open-cluster-management-io/sdk-go/blob/d3c47c228d7905ebb20f331f9b72bc5ff6a84789/pkg/cloudevents/generic/interface.go#L36-L39
func (s *sqlResourceService) List(listOpts cetypes.ListOptions) ([]*api.Resource, error) {
	resourceList, err := s.resourceDao.FindByConsumerName(context.TODO(), listOpts.ClusterName)
	if err != nil {
		return nil, err
	}
	return resourceList, nil
}

// ListWithArgs lists resources based on the provided page and filter arguments.
func (s *sqlResourceService) ListWithArgs(ctx context.Context, username string, args *ListArguments, resources *[]api.Resource) (*api.PagingMeta, *errors.ServiceError) {
	paging, serviceErr := s.generic.List(ctx, username, args, resources)
	if serviceErr != nil {
		return nil, serviceErr
	}

	for _, resource := range *resources {
		// sync the creationTimestamp and deletionTimestamp from resource meta to work metadata
		s.syncTimestampsFromResourceMeta(&resource)
	}

	return paging, nil
}

func (s *sqlResourceService) syncTimestampsFromResourceMeta(resource *api.Resource) {
	// fill back the creationTimestamp and deletionTimestamp from resource meta to work metadata if it exists
	workMetaValue, ok := resource.Payload["metadata"]
	if ok {
		workMeta, ok := workMetaValue.(map[string]interface{})
		if ok {
			workMeta["creationTimestamp"] = resource.CreatedAt.Format(time.RFC3339)
			if !resource.DeletedAt.Time.IsZero() {
				workMeta["deletionTimestamp"] = resource.DeletedAt.Time.Format(time.RFC3339)
			}
			resource.Payload["metadata"] = workMeta
		}
	}
}

// Subsystem used to define the metrics:
const metricsSubsystem = "resource"

// Names of the labels added to metrics:
const (
	metricsIDLabel     = "id"
	metricsActionLabel = "action"
)

// metricsLabels - Array of labels added to metrics:
var metricsLabels = []string{
	metricsIDLabel,
	metricsActionLabel,
}

// Names of the metrics:
const (
	processedCountMetric = "processed_total"
)

// Register the metrics:
func RegisterResourceMetrics() {
	prometheus.MustRegister(resourceProcessedCountMetric)
}

// Unregister the metrics:
func UnregisterResourceMetrics() {
	prometheus.Unregister(resourceProcessedCountMetric)
}

// Reset the metrics:
func ResetResourceMetrics() {
	resourceProcessedCountMetric.Reset()
}

// Description of the resource process count metric:
var resourceProcessedCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: metricsSubsystem,
		Name:      processedCountMetric,
		Help:      "Number of processed resources.",
	},
	metricsLabels,
)
