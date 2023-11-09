package mocks

import (
	"context"

	"github.com/openshift-online/maestro/pkg/dao"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

var _ dao.DinosaurDao = &dinosaurDaoMock{}

type dinosaurDaoMock struct {
	dinosaurs api.DinosaurList
}

func NewDinosaurDao() *dinosaurDaoMock {
	return &dinosaurDaoMock{}
}

func (d *dinosaurDaoMock) Get(ctx context.Context, id string) (*api.Dinosaur, error) {
	for _, dino := range d.dinosaurs {
		if dino.ID == id {
			return dino, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *dinosaurDaoMock) Create(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, error) {
	d.dinosaurs = append(d.dinosaurs, dinosaur)
	return dinosaur, nil
}

func (d *dinosaurDaoMock) Replace(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, error) {
	return nil, errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) FindByIDs(ctx context.Context, ids []string) (api.DinosaurList, error) {
	return nil, errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) FindBySpecies(ctx context.Context, species string) (api.DinosaurList, error) {
	var dinos api.DinosaurList
	for _, dino := range d.dinosaurs {
		if dino.Species == species {
			dinos = append(dinos, dino)
		}
	}
	return dinos, nil
}

func (d *dinosaurDaoMock) All(ctx context.Context) (api.DinosaurList, error) {
	return d.dinosaurs, nil
}
