package services

import (
	"context"
	"testing"

	gm "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
)

func TestResourceFindByConsumerID(t *testing.T) {
	gm.RegisterTestingT(t)

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events)

	const Fukuisaurus = "Fukuisaurus"
	const Seismosaurus = "Seismosaurus"
	const Breviceratops = "Breviceratops"

	resources := api.ResourceList{
		&api.Resource{ConsumerName: Fukuisaurus, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
		&api.Resource{ConsumerName: Fukuisaurus, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
		&api.Resource{ConsumerName: Seismosaurus, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
		&api.Resource{ConsumerName: Seismosaurus, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
		&api.Resource{ConsumerName: Breviceratops, Manifest: newManifest(t, "{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"test\",\"namespace\":\"test\"}}")},
	}
	for _, resource := range resources {
		_, err := resourceService.Create(context.Background(), resource)
		gm.Expect(err).To(gm.BeNil())
	}
	fukuisaurus, err := resourceService.FindByConsumerName(context.Background(), Fukuisaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(fukuisaurus)).To(gm.Equal(3))

	seismosaurus, err := resourceService.FindByConsumerName(context.Background(), Seismosaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(seismosaurus)).To(gm.Equal(2))

	breviceratops, err := resourceService.FindByConsumerName(context.Background(), Breviceratops)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(breviceratops)).To(gm.Equal(1))
}

func TestCreateInvalidResource(t *testing.T) {
	gm.RegisterTestingT(t)

	resourceDAO := mocks.NewResourceDao()
	events := NewEventService(mocks.NewEventDao())
	resourceService := NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourceDAO, events)

	resource := &api.Resource{ConsumerName: "invalidation", Manifest: newManifest(t, "{}")}

	_, err := resourceService.Create(context.Background(), resource)
	gm.Expect(err).ShouldNot(gm.BeNil())

	invalidations, err := resourceService.FindByConsumerName(context.Background(), "invalidation")
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(invalidations)).To(gm.Equal(0))
}
