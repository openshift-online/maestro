package services

import (
	"context"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	logger "github.com/openshift-online/maestro/pkg/logger"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

// This flag will only be used in integration test to prove that the advisory lock works
var DisableAdvisoryLock = false

type DinosaurService interface {
	Get(ctx context.Context, id string) (*api.Dinosaur, *errors.ServiceError)
	Create(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, *errors.ServiceError)
	Replace(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.DinosaurList, *errors.ServiceError)

	FindBySpecies(ctx context.Context, species string) (api.DinosaurList, *errors.ServiceError)
	FindByIDs(ctx context.Context, ids []string) (api.DinosaurList, *errors.ServiceError)

	// idempotent functions for the control plane, but can also be called synchronously by any actor
	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewDinosaurService(lockFactory db.LockFactory, dinosaurDao dao.DinosaurDao, events EventService) DinosaurService {
	return &sqlDinosaurService{
		lockFactory: lockFactory,
		dinosaurDao: dinosaurDao,
		events:      events,
	}
}

var _ DinosaurService = &sqlDinosaurService{}

type sqlDinosaurService struct {
	lockFactory db.LockFactory
	dinosaurDao dao.DinosaurDao
	events      EventService
}

func (s *sqlDinosaurService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	dinosaur, err := s.dinosaurDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this dinosaur: %s", dinosaur.ID)

	return nil
}

func (s *sqlDinosaurService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)
	logger.Infof("This dino didn't make it to the asteroid: %s", id)
	return nil
}

func (s *sqlDinosaurService) Get(ctx context.Context, id string) (*api.Dinosaur, *errors.ServiceError) {
	dinosaur, err := s.dinosaurDao.Get(ctx, id)
	if err != nil {
		return nil, handleGetError("Dinosaur", "id", id, err)
	}
	return dinosaur, nil
}

func (s *sqlDinosaurService) Create(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, *errors.ServiceError) {
	dinosaur, err := s.dinosaurDao.Create(ctx, dinosaur)
	if err != nil {
		return nil, handleCreateError("Dinosaur", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  dinosaur.ID,
		EventType: api.CreateEventType,
	})
	if eErr != nil {
		return nil, handleCreateError("Dinosaur", err)
	}

	return dinosaur, nil
}

func (s *sqlDinosaurService) Replace(ctx context.Context, dinosaur *api.Dinosaur) (*api.Dinosaur, *errors.ServiceError) {
	if !DisableAdvisoryLock {
		// Updates the dinosaur species only when its species changes.
		// If there are multiple requests at the same time, it will cause the race conditions among these
		// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
		lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, dinosaur.ID, db.Dinosaurs)
		if err != nil {
			return nil, errors.DatabaseAdvisoryLock(err)
		}
		defer s.lockFactory.Unlock(ctx, lockOwnerID)
	}

	found, err := s.dinosaurDao.Get(ctx, dinosaur.ID)
	if err != nil {
		return nil, handleGetError("Dinosaur", "id", dinosaur.ID, err)
	}

	// New species is no change, the update action is not needed.
	if found.Species == dinosaur.Species {
		return found, nil
	}

	found.Species = dinosaur.Species
	updated, err := s.dinosaurDao.Replace(ctx, found)
	if err != nil {
		return nil, handleUpdateError("Dinosaur", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  updated.ID,
		EventType: api.UpdateEventType,
	})
	if eErr != nil {
		return nil, handleUpdateError("Dinosaur", err)
	}
	return updated, nil
}

func (s *sqlDinosaurService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.dinosaurDao.Delete(ctx, id); err != nil {
		return handleDeleteError("Dinosaur", errors.GeneralError("Unable to delete dinosaur: %s", err))
	}

	_, err := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if err != nil {
		return handleDeleteError("Dinosaur", err)
	}

	return nil
}

func (s *sqlDinosaurService) FindByIDs(ctx context.Context, ids []string) (api.DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all dinosaurs: %s", err)
	}
	return dinosaurs, nil
}

func (s *sqlDinosaurService) FindBySpecies(ctx context.Context, species string) (api.DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.FindBySpecies(ctx, species)
	if err != nil {
		return nil, handleGetError("Dinosaur", "species", species, err)
	}
	return dinosaurs, nil
}

func (s *sqlDinosaurService) All(ctx context.Context) (api.DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all dinosaurs: %s", err)
	}
	return dinosaurs, nil
}
