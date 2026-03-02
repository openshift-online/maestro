package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	gm "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
)

const (
	Fukuisaurus   = "b288a9da-8bfe-4c82-94cc-2b48e773fc46"
	Seismosaurus  = "e3eb7db1-b124-4a4d-8bb6-cc779c01b402"
	Breviceratops = "c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4"
)

func TestResourceFindByConsumerID(t *testing.T) {
	gm.RegisterTestingT(t)

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())

	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	resources := api.ResourceList{
		&api.Resource{ConsumerName: Fukuisaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"e3eb7db1-b124-4a4d-8bb6-cc779c01b402\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"e3eb7db1-b124-4a4d-8bb6-cc779c01b402\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Breviceratops, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
	}
	for _, resource := range resources {
		_, err := resourceService.Create(context.Background(), resource)
		gm.Expect(err).To(gm.BeNil())
	}
	fukuisaurus, err := resourceDAO.FindByConsumerName(context.Background(), Fukuisaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(fukuisaurus)).To(gm.Equal(3))

	seismosaurus, err := resourceDAO.FindByConsumerName(context.Background(), Seismosaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(seismosaurus)).To(gm.Equal(2))

	breviceratops, err := resourceDAO.FindByConsumerName(context.Background(), Breviceratops)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(breviceratops)).To(gm.Equal(1))
}

func TestCreateInvalidResource(t *testing.T) {
	gm.RegisterTestingT(t)

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	resource := &api.Resource{ConsumerName: "invalidation", Payload: newPayload(t, "{}")}

	_, svcErr := resourceService.Create(context.Background(), resource)
	gm.Expect(svcErr).ShouldNot(gm.BeNil())

	invalidations, err := resourceDAO.FindByConsumerName(context.Background(), "invalidation")
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(invalidations)).To(gm.Equal(0))
}

func TestResourceList(t *testing.T) {
	gm.RegisterTestingT(t)

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())

	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)
	resources := api.ResourceList{
		&api.Resource{ConsumerName: Fukuisaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"quay.io/nginx/nginx-unprivileged:latest\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
	}
	for _, resource := range resources {
		_, err := resourceService.Create(context.Background(), resource)
		gm.Expect(err).To(gm.BeNil())
	}

	resources, err := resourceService.List(context.Background(), types.ListOptions{
		ClusterName:         Fukuisaurus,
		CloudEventsDataType: payload.ManifestBundleEventDataType,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resources)).To(gm.Equal(2))

	resources, err = resourceService.List(context.Background(), types.ListOptions{
		ClusterName: Seismosaurus,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resources)).To(gm.Equal(1))

	resources, err = resourceService.List(context.Background(), types.ListOptions{
		ClusterName:         Seismosaurus,
		CloudEventsDataType: payload.ManifestBundleEventDataType,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resources)).To(gm.Equal(1))
}

// TestTimeToFirstStatusMetric_FirstStatusUpdate verifies that the metric is recorded
// when a resource's status transitions from empty to non-empty
func TestTimeToFirstStatusMetric_FirstStatusUpdate(t *testing.T) {
	gm.RegisterTestingT(t)

	// Reset metrics before test
	ResetResourceMetrics()

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	// Create a resource with empty status
	resource := &api.Resource{
		Meta: api.Meta{
			ID: "test-resource-1",
		},
		ConsumerName: "test-consumer",
		Source:       "test-source",
		Version:      1,
		Payload:      newPayload(t, validManifestBundle),
		Status:       datatypes.JSONMap{}, // Empty status
	}
	resource.CreatedAt = time.Now().Add(-5 * time.Second) // Created 5 seconds ago

	created, err := resourceService.Create(context.Background(), resource)
	gm.Expect(err).To(gm.BeNil())

	// Update the resource with first status
	statusUpdate := &api.Resource{
		Meta: api.Meta{
			ID: created.ID,
		},
		Version: created.Version,
		Status:  createTestStatus(t, created.ID, "1"),
	}

	_, updated, svcErr := resourceService.UpdateStatus(context.Background(), statusUpdate)
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(updated).To(gm.BeTrue())

	// Verify the metric was recorded (count should be 1)
	metricCount := testutil.CollectAndCount(resourceTimeToFirstStatusMetric)
	gm.Expect(metricCount).To(gm.BeNumerically(">", 0), "Metric should be recorded for first status update")
}

// TestTimeToFirstStatusMetric_SubsequentStatusUpdates verifies that the metric is NOT recorded
// when a resource already has status (subsequent updates)
func TestTimeToFirstStatusMetric_SubsequentStatusUpdates(t *testing.T) {
	gm.RegisterTestingT(t)

	// Reset metrics before test
	ResetResourceMetrics()

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	// Create a resource
	resource := &api.Resource{
		Meta: api.Meta{
			ID: "test-resource-2",
		},
		ConsumerName: "test-consumer-2",
		Source:       "test-source-2",
		Version:      1,
		Payload:      newPayload(t, validManifestBundle),
		Status:       datatypes.JSONMap{}, // Empty status initially
	}
	resource.CreatedAt = time.Now().Add(-10 * time.Second)

	created, err := resourceService.Create(context.Background(), resource)
	gm.Expect(err).To(gm.BeNil())

	// First status update - should record metric
	statusUpdate1 := &api.Resource{
		Meta: api.Meta{
			ID: created.ID,
		},
		Version: created.Version,
		Status:  createTestStatus(t, created.ID, "1"),
	}
	_, updated, svcErr := resourceService.UpdateStatus(context.Background(), statusUpdate1)
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(updated).To(gm.BeTrue())

	// Get the resource to get the updated status
	updatedResource, svcErr := resourceService.Get(context.Background(), created.ID)
	gm.Expect(svcErr).To(gm.BeNil())

	// Verify metric count is 1 after first update
	metricCount := testutil.CollectAndCount(resourceTimeToFirstStatusMetric)
	gm.Expect(metricCount).To(gm.Equal(1), "Should have one histogram observation")

	// Second status update - should NOT record metric again
	statusUpdate2 := &api.Resource{
		Meta: api.Meta{
			ID: updatedResource.ID,
		},
		Version: updatedResource.Version,
		Status:  createTestStatus(t, updatedResource.ID, "2"),
	}
	_, updated, svcErr = resourceService.UpdateStatus(context.Background(), statusUpdate2)
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(updated).To(gm.BeTrue())

	// Verify metric count is still 1 (not incremented)
	metricCountAfter := testutil.CollectAndCount(resourceTimeToFirstStatusMetric)
	gm.Expect(metricCountAfter).To(gm.Equal(1), "Metric should not be recorded for subsequent status updates")
}

// TestTimeToFirstStatusMetric_StaleVersionUpdate verifies that the metric is NOT recorded
// when the version is stale (version mismatch)
func TestTimeToFirstStatusMetric_StaleVersionUpdate(t *testing.T) {
	gm.RegisterTestingT(t)

	// Reset metrics before test
	ResetResourceMetrics()

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	// Create a resource
	resource := &api.Resource{
		Meta: api.Meta{
			ID: "test-resource-3",
		},
		ConsumerName: "test-consumer-3",
		Source:       "test-source-3",
		Version:      2, // Current version is 2
		Payload:      newPayload(t, validManifestBundle),
		Status:       datatypes.JSONMap{}, // Empty status
	}
	resource.CreatedAt = time.Now().Add(-5 * time.Second)

	created, err := resourceService.Create(context.Background(), resource)
	gm.Expect(err).To(gm.BeNil())

	// Try to update status with stale version (version 1 instead of 2)
	staleStatusUpdate := &api.Resource{
		Meta: api.Meta{
			ID: created.ID,
		},
		Version: 1, // Stale version
		Status:  createTestStatus(t, created.ID, "1"),
	}

	_, updated, svcErr := resourceService.UpdateStatus(context.Background(), staleStatusUpdate)
	gm.Expect(svcErr).To(gm.BeNil())
	gm.Expect(updated).To(gm.BeFalse(), "Status update should be rejected for stale version")

	// Verify metric was NOT recorded
	metricCount := testutil.CollectAndCount(resourceTimeToFirstStatusMetric)
	gm.Expect(metricCount).To(gm.Equal(0), "Metric should not be recorded for stale version updates")
}

// TestTimeToFirstStatusMetric_MetricLabels verifies that the metric labels are set correctly
func TestTimeToFirstStatusMetric_MetricLabels(t *testing.T) {
	gm.RegisterTestingT(t)

	// Reset metrics before test
	ResetResourceMetrics()

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events, nil)

	testCases := []struct {
		consumerName string
		source       string
	}{
		{consumerName: "cluster-a", source: "grpc"},
		{consumerName: "cluster-b", source: "mqtt"},
		{consumerName: "cluster-c", source: "rest-api"},
	}

	for i, tc := range testCases {
		resource := &api.Resource{
			Meta: api.Meta{
				ID: fmt.Sprintf("test-resource-labels-%d", i),
			},
			ConsumerName: tc.consumerName,
			Source:       tc.source,
			Version:      1,
			Payload:      newPayload(t, validManifestBundle),
			Status:       datatypes.JSONMap{},
		}
		resource.CreatedAt = time.Now().Add(-3 * time.Second)

		created, err := resourceService.Create(context.Background(), resource)
		gm.Expect(err).To(gm.BeNil())

		// Update status
		statusUpdate := &api.Resource{
			Meta: api.Meta{
				ID: created.ID,
			},
			Version: created.Version,
			Status:  createTestStatus(t, created.ID, "1"),
		}
		_, updated, svcErr := resourceService.UpdateStatus(context.Background(), statusUpdate)
		gm.Expect(svcErr).To(gm.BeNil())
		gm.Expect(updated).To(gm.BeTrue())
	}

	// Verify all 3 metrics were recorded (one for each test case)
	metricCount := testutil.CollectAndCount(resourceTimeToFirstStatusMetric)
	gm.Expect(metricCount).To(gm.Equal(len(testCases)),
		"Metric should be recorded for each resource with different labels")
}

// createTestStatus creates a simple test status with a sequence ID
func createTestStatus(t *testing.T, resourceID, sequenceID string) datatypes.JSONMap {
	source := "test-agent"
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.UpdateRequestAction,
	}

	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithResourceID(resourceID).
		WithResourceVersion(1).
		WithStatusUpdateSequenceID(sequenceID)

	evt := evtBuilder.NewEvent()

	statusPayload := &payload.ManifestBundleStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "Applied",
				Status:  "True",
				Message: fmt.Sprintf("Test status with sequence %s", sequenceID),
			},
		},
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, statusPayload); err != nil {
		t.Fatalf("failed to set cloud event data: %v", err)
	}

	statusMap, err := api.CloudEventToJSONMap(&evt)
	if err != nil {
		t.Fatalf("failed to convert cloudevent to status map: %v", err)
	}

	return statusMap
}

const validManifestBundle = `{
	"id": "266a8cd2-2fab-4e89-9bf0-a56425ebcdf8",
	"time": "2024-02-05T17:31:05Z",
	"type": "io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request",
	"source": "grpc",
	"specversion": "1.0",
	"datacontenttype": "application/json",
	"resourceid": "test-resource",
	"clustername": "test-cluster",
	"resourceversion": 1,
	"data": {
		"manifests": [{
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
				"name": "test-cm",
				"namespace": "default"
			}
		}]
	}
}`
