package services

import (
	"context"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

type StatusEventService interface {
	Get(ctx context.Context, id string) (*api.StatusEvent, *errors.ServiceError)
	Create(ctx context.Context, event *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError)
	Replace(ctx context.Context, event *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.StatusEventList, *errors.ServiceError)
	FindByIDs(ctx context.Context, ids []string) (api.StatusEventList, *errors.ServiceError)

	FindAllUnreconciledEvents(ctx context.Context) (api.StatusEventList, *errors.ServiceError)
	DeleteAllReconciledEvents(ctx context.Context) *errors.ServiceError
	DeleteAllEvents(ctx context.Context, eventIDs []string) *errors.ServiceError
}

func NewStatusEventService(statusEventDao dao.StatusEventDao) StatusEventService {
	return &sqlStatusEventService{
		statusEventDao: statusEventDao,
	}
}

var _ StatusEventService = &sqlStatusEventService{}

type sqlStatusEventService struct {
	statusEventDao dao.StatusEventDao
}

func (s *sqlStatusEventService) Get(ctx context.Context, id string) (*api.StatusEvent, *errors.ServiceError) {
	event, err := s.statusEventDao.Get(ctx, id)
	if err != nil {
		return nil, handleGetError("StatusEvent", "id", id, err)
	}
	return event, nil
}

func (s *sqlStatusEventService) Create(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError) {
	event, err := s.statusEventDao.Create(ctx, statusEvent)
	if err != nil {
		return nil, handleCreateError("StatusEvent", err)
	}
	return event, nil
}

func (s *sqlStatusEventService) Replace(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, *errors.ServiceError) {
	statusEvent, err := s.statusEventDao.Replace(ctx, statusEvent)
	if err != nil {
		return nil, handleUpdateError("StatusEvent", err)
	}
	return statusEvent, nil
}

func (s *sqlStatusEventService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.statusEventDao.Delete(ctx, id); err != nil {
		return handleDeleteError("StatusEvent", errors.GeneralError("Unable to delete status event: %s", err))
	}
	return nil
}

func (s *sqlStatusEventService) FindByIDs(ctx context.Context, ids []string) (api.StatusEventList, *errors.ServiceError) {
	statusEvents, err := s.statusEventDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all status events: %s", err)
	}
	return statusEvents, nil
}

func (s *sqlStatusEventService) All(ctx context.Context) (api.StatusEventList, *errors.ServiceError) {
	statusEvents, err := s.statusEventDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all status events: %s", err)
	}
	return statusEvents, nil
}

func (s *sqlStatusEventService) FindAllUnreconciledEvents(ctx context.Context) (api.StatusEventList, *errors.ServiceError) {
	statusEvents, err := s.statusEventDao.FindAllUnreconciledEvents(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get unreconciled status events: %s", err)
	}
	return statusEvents, nil
}

func (s *sqlStatusEventService) DeleteAllReconciledEvents(ctx context.Context) *errors.ServiceError {
	if err := s.statusEventDao.DeleteAllReconciledEvents(ctx); err != nil {
		return handleDeleteError("StatusEvent", errors.GeneralError("Unable to delete reconciled status events: %s", err))
	}
	return nil
}

func (s *sqlStatusEventService) DeleteAllEvents(ctx context.Context, eventIDs []string) *errors.ServiceError {
	if err := s.statusEventDao.DeleteAllEvents(ctx, eventIDs); err != nil {
		return handleDeleteError("StatusEvent", errors.GeneralError("Unable to delete events %s: %s", eventIDs, err))
	}
	return nil
}
