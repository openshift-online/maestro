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

	dinos := api.ResourceList{
		&api.Resource{ConsumerID: Fukuisaurus},
		&api.Resource{ConsumerID: Fukuisaurus},
		&api.Resource{ConsumerID: Fukuisaurus},
		&api.Resource{ConsumerID: Seismosaurus},
		&api.Resource{ConsumerID: Seismosaurus},
		&api.Resource{ConsumerID: Breviceratops},
	}
	for _, dino := range dinos {
		_, err := resourceService.Create(context.Background(), dino)
		gm.Expect(err).To(gm.BeNil())
	}
	fukuisaurus, err := resourceService.FindByConsumerIDs(context.Background(), Fukuisaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(fukuisaurus)).To(gm.Equal(3))

	seismosaurus, err := resourceService.FindByConsumerIDs(context.Background(), Seismosaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(seismosaurus)).To(gm.Equal(2))

	breviceratops, err := resourceService.FindByConsumerIDs(context.Background(), Breviceratops)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(breviceratops)).To(gm.Equal(1))
}
