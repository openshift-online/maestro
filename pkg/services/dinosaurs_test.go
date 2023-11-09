package services

import (
	"context"
	"testing"

	gm "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
)

func TestDinosaurFindBySpecies(t *testing.T) {
	gm.RegisterTestingT(t)

	dinoDAO := mocks.NewDinosaurDao()
	events := NewEventService(mocks.NewEventDao())
	dinoService := NewDinosaurService(dbmocks.NewMockAdvisoryLockFactory(), dinoDAO, events)

	const Fukuisaurus = "Fukuisaurus"
	const Seismosaurus = "Seismosaurus"
	const Breviceratops = "Breviceratops"

	dinos := api.DinosaurList{
		&api.Dinosaur{Species: Fukuisaurus},
		&api.Dinosaur{Species: Fukuisaurus},
		&api.Dinosaur{Species: Fukuisaurus},
		&api.Dinosaur{Species: Seismosaurus},
		&api.Dinosaur{Species: Seismosaurus},
		&api.Dinosaur{Species: Breviceratops},
	}
	for _, dino := range dinos {
		_, err := dinoService.Create(context.Background(), dino)
		gm.Expect(err).To(gm.BeNil())
	}
	fukuisaurus, err := dinoService.FindBySpecies(context.Background(), Fukuisaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(fukuisaurus)).To(gm.Equal(3))

	seismosaurus, err := dinoService.FindBySpecies(context.Background(), Seismosaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(seismosaurus)).To(gm.Equal(2))

	breviceratops, err := dinoService.FindBySpecies(context.Background(), Breviceratops)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(breviceratops)).To(gm.Equal(1))
}
