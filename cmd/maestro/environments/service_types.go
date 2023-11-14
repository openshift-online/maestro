package environments

import (
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/services"
)

type ResourceServiceLocator func() services.ResourceService

func NewResourceServiceLocator(env *Env) ResourceServiceLocator {
	return func() services.ResourceService {
		return services.NewResourceService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			dao.NewResourceDao(&env.Database.SessionFactory),
			env.Services.Events(),
		)
	}
}

type GenericServiceLocator func() services.GenericService

func NewGenericServiceLocator(env *Env) GenericServiceLocator {
	return func() services.GenericService {
		return services.NewGenericService(dao.NewGenericDao(&env.Database.SessionFactory))
	}
}

type EventServiceLocator func() services.EventService

func NewEventServiceLocator(env *Env) EventServiceLocator {
	return func() services.EventService {
		return services.NewEventService(dao.NewEventDao(&env.Database.SessionFactory))
	}
}

type ConsumerServiceLocator func() services.ConsumerService

func NewConsumerServiceLocator(env *Env) ConsumerServiceLocator {
	return func() services.ConsumerService {
		return services.NewConsumerService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			dao.NewConsumerDao(&env.Database.SessionFactory),
			env.Services.Events(),
		)
	}
}
