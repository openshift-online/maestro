package services

import (
	"context"
	"testing"

	gm "github.com/onsi/gomega"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

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
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeBundle, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Type: api.ResourceTypeBundle, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"e3eb7db1-b124-4a4d-8bb6-cc779c01b402\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Breviceratops, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Breviceratops, Type: api.ResourceTypeBundle, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
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
	gm.Expect(len(breviceratops)).To(gm.Equal(2))
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
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Type: api.ResourceTypeBundle, Payload: newPayload(t, "{\"id\":\"266a8cd2-2fab-4e89-9bf0-a56425ebcdf8\",\"time\":\"2024-02-05T17:31:05Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"grpc\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"resourceid\":\"c4df9ff0-bfeb-5bc6-a0ab-4c9128d698b4\",\"clustername\":\"b288a9da-8bfe-4c82-94cc-2b48e773fc46\",\"resourceversion\":1,\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"}},{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"nginx\",\"namespace\":\"default\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginxinc/nginx-unprivileged\"}]},\"metadata\":{\"labels\":{\"app\":\"nginx\"}}}}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"nginx\",\"group\":\"apps\",\"resource\":\"deployments\",\"namespace\":\"default\"}}]}}")},
		&api.Resource{ConsumerName: Seismosaurus, Type: api.ResourceTypeSingle, Payload: newPayload(t, "{\"id\":\"75479c10-b537-4261-8058-ca2e36bac384\",\"time\":\"2024-03-07T03:29:03.194843266Z\",\"type\":\"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request\",\"source\":\"maestro\",\"specversion\":\"1.0\",\"datacontenttype\":\"application/json\",\"data\":{\"manifests\":[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}],\"deleteOption\":{\"propagationPolicy\":\"Foreground\"},\"manifestConfigs\":[{\"updateStrategy\":{\"type\":\"ServerSideApply\"},\"resourceIdentifier\":{\"name\":\"test\",\"group\":\"\",\"resource\":\"configmaps\",\"namespace\":\"test\"}}]}}")},
	}
	for _, resource := range resources {
		_, err := resourceService.Create(context.Background(), resource)
		gm.Expect(err).To(gm.BeNil())
	}

	resoruces, err := resourceService.List(types.ListOptions{
		ClusterName: Fukuisaurus,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resoruces)).To(gm.Equal(3))

	resoruces, err = resourceService.List(types.ListOptions{
		ClusterName: Seismosaurus,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resoruces)).To(gm.Equal(1))

	resoruces, err = resourceService.List(types.ListOptions{
		ClusterName:         Seismosaurus,
		CloudEventsDataType: payload.ManifestEventDataType,
	})
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(resoruces)).To(gm.Equal(1))
}
