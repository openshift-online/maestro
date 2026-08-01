package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	. "github.com/onsi/gomega"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/server"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

// testPayload builds a minimal, valid CloudEvent-shaped payload usable as a resource's Payload.
func testPayload(t *testing.T) datatypes.JSONMap {
	t.Helper()
	data := `{
		"id":"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8",
		"time":"2024-02-05T17:31:05Z",
		"type":"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request",
		"source":"grpc",
		"specversion":"1.0",
		"datacontenttype":"application/json",
		"resourceid":"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4",
		"clustername":"cluster1",
		"resourceversion":1,
		"data":{"manifests":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"nginx","namespace":"default"}}]}
	}`
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

// fakeAgentEventServer is a minimal implementation of server.AgentEventServer used to observe
// whether HandleEvent (i.e. publishing to the agent) was invoked.
type fakeAgentEventServer struct {
	handledEvents []*ce.Event
}

func (f *fakeAgentEventServer) HandleEvent(_ context.Context, evt *ce.Event) error {
	f.handledEvents = append(f.handledEvents, evt)
	return nil
}

func (f *fakeAgentEventServer) RegisterService(_ context.Context, _ types.CloudEventsDataType, _ server.Service) {
}

func (f *fakeAgentEventServer) Subscribers() sets.Set[string] {
	return sets.New[string]()
}

// fakeResourceService is a configurable mock of services.ResourceService used to test
// GRPCBroker's OnCreate/OnUpdate/OnDelete handlers in isolation.
type fakeResourceService struct {
	services.ResourceService
	resource  *api.Resource
	resources []*api.Resource
	getErr    *errors.ServiceError
}

func (m *fakeResourceService) Get(_ context.Context, _ string) (*api.Resource, *errors.ServiceError) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.resource, nil
}

func (m *fakeResourceService) List(_ context.Context, _ types.ListOptions) ([]*api.Resource, error) {
	return m.resources, nil
}

// TestGRPCBrokerOnCreateSkipsResourceMarkedAsDeleting verifies that OnCreate does not publish a
// create_request to the agent for a resource that is already marked as deleting. This guards
// against ARO-28432, where sending (or worse, hard-deleting without notifying the agent) for a
// stale/retried Create event orphaned resources already applied on the managed cluster.
func TestGRPCBrokerOnCreateSkipsResourceMarkedAsDeleting(t *testing.T) {
	RegisterTestingT(t)

	resource := &api.Resource{
		Meta: api.Meta{
			ID:        "res-1",
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		ConsumerName: "cluster1",
	}
	fakeSvc := &fakeResourceService{resource: resource}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnCreate(context.Background(), resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(BeEmpty(), "OnCreate must not publish create_request for a resource marked as deleting")
}

// TestGRPCBrokerOnCreateSkipsWhenResourceNotFound verifies OnCreate is a no-op (not an error)
// when the resource has already been hard-deleted.
func TestGRPCBrokerOnCreateSkipsWhenResourceNotFound(t *testing.T) {
	RegisterTestingT(t)

	fakeSvc := &fakeResourceService{getErr: errors.NotFound("resource not found")}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnCreate(context.Background(), "missing-id")
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(BeEmpty())
}

// TestGRPCBrokerOnCreatePublishesForNormalResource verifies OnCreate publishes a create_request
// for a resource that is not marked as deleting.
func TestGRPCBrokerOnCreatePublishesForNormalResource(t *testing.T) {
	RegisterTestingT(t)

	resource := &api.Resource{
		Meta:         api.Meta{ID: "res-2"},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resource: resource}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnCreate(context.Background(), resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(HaveLen(1))
}

// TestGRPCBrokerOnDeleteSkipsWhenResourceNotFound verifies OnDelete returns nil (allowing the
// event to be reconciled) instead of an error when the resource has already been hard-deleted,
// preventing infinite retries of the delete event.
func TestGRPCBrokerOnDeleteSkipsWhenResourceNotFound(t *testing.T) {
	RegisterTestingT(t)

	fakeSvc := &fakeResourceService{getErr: errors.NotFound("resource not found")}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnDelete(context.Background(), "missing-id")
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(BeEmpty())
}

// TestGRPCBrokerOnUpdateSkipsWhenResourceNotFound verifies OnUpdate returns nil instead of an
// error when the resource has already been hard-deleted.
func TestGRPCBrokerOnUpdateSkipsWhenResourceNotFound(t *testing.T) {
	RegisterTestingT(t)

	fakeSvc := &fakeResourceService{getErr: errors.NotFound("resource not found")}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnUpdate(context.Background(), "missing-id")
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(BeEmpty())
}

// TestGRPCBrokerOnUpdateSkipsResourceMarkedAsDeleting verifies that OnUpdate does not publish an
// update_request to the agent for a resource that is already marked as deleting. Re-publishing the
// spec here re-creates a ManifestWork whose delete the agent has already processed, which keeps the
// bundle non-empty so its delete never completes (observed in stg/int). Mirrors the OnCreate guard.
func TestGRPCBrokerOnUpdateSkipsResourceMarkedAsDeleting(t *testing.T) {
	RegisterTestingT(t)

	resource := &api.Resource{
		Meta: api.Meta{
			ID:        "res-4",
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resource: resource}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnUpdate(context.Background(), resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(BeEmpty(), "OnUpdate must not publish update_request for a resource marked as deleting")
}

// TestGRPCBrokerOnUpdatePublishesForNormalResource verifies OnUpdate publishes an update_request
// for a resource that is not marked as deleting.
func TestGRPCBrokerOnUpdatePublishesForNormalResource(t *testing.T) {
	RegisterTestingT(t)

	resource := &api.Resource{
		Meta:         api.Meta{ID: "res-5"},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resource: resource}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnUpdate(context.Background(), resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(HaveLen(1))
}

// TestGRPCBrokerOnDeletePublishesForExistingResource verifies OnDelete publishes a
// delete_request for a resource that still exists and is marked as deleting.
func TestGRPCBrokerOnDeletePublishesForExistingResource(t *testing.T) {
	RegisterTestingT(t)

	resource := &api.Resource{
		Meta: api.Meta{
			ID:        "res-3",
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resource: resource}
	fakeEventServer := &fakeAgentEventServer{}
	broker := &GRPCBroker{
		resourceService: fakeSvc,
		eventServer:     fakeEventServer,
	}

	err := broker.OnDelete(context.Background(), resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(fakeEventServer.handledEvents).To(HaveLen(1))
}

// TestGRPCBrokerServiceListExcludesDeletingResources verifies that List does not return
// CloudEvents for resources marked as deleting. Including them causes the agent to re-apply
// a ManifestWork that it already deleted, creating an infinite delete-then-recreate loop.
// The sdk-go source client handles the missing resource correctly during resync: it detects
// that the resource exists on the agent but not in the server's response, and sends a
// delete_request to the agent.
func TestGRPCBrokerServiceListExcludesDeletingResources(t *testing.T) {
	RegisterTestingT(t)

	live := &api.Resource{
		Meta:         api.Meta{ID: "live-1"},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	deleting := &api.Resource{
		Meta: api.Meta{
			ID:        "deleting-1",
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resources: []*api.Resource{live, deleting}}
	svc := &GRPCBrokerService{resourceService: fakeSvc}

	evts, err := svc.List(context.Background(), types.ListOptions{ClusterName: "cluster1"})
	Expect(err).NotTo(HaveOccurred())
	Expect(evts).To(HaveLen(1), "List must exclude resources marked as deleting")
}

// TestGRPCBrokerServiceListIncludesLiveResources verifies that List returns
// CloudEvents for live (non-deleting) resources.
func TestGRPCBrokerServiceListIncludesLiveResources(t *testing.T) {
	RegisterTestingT(t)

	r1 := &api.Resource{
		Meta:         api.Meta{ID: "res-1"},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	r2 := &api.Resource{
		Meta:         api.Meta{ID: "res-2"},
		ConsumerName: "cluster1",
		Payload:      testPayload(t),
	}
	fakeSvc := &fakeResourceService{resources: []*api.Resource{r1, r2}}
	svc := &GRPCBrokerService{resourceService: fakeSvc}

	evts, err := svc.List(context.Background(), types.ListOptions{ClusterName: "cluster1"})
	Expect(err).NotTo(HaveOccurred())
	Expect(evts).To(HaveLen(2), "List must include all live resources")
}
