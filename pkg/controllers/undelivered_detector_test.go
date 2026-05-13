package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
	"github.com/openshift-online/maestro/pkg/services"
)

func TestUndeliveredDetector(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	resourcesDao := mocks.NewResourceDao()
	eventsDao := mocks.NewEventDao()
	resourceService := services.NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourcesDao, services.NewEventService(eventsDao), nil)
	eventService := services.NewEventService(eventsDao)

	threshold := 5 * time.Minute
	detector := NewUndeliveredDetector(resourceService, eventService, dbmocks.NewMockAdvisoryLockFactory(), int(threshold.Seconds()))

	// Resource created 10 minutes ago with no status — should be re-published
	staleResource := &api.Resource{
		Meta:         api.Meta{ID: "stale-1", CreatedAt: time.Now().Add(-10 * time.Minute)},
		Version:      1,
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err := resourcesDao.Create(ctx, staleResource)
	Expect(err).To(BeNil())

	// Resource created 1 minute ago with no status — too young, should NOT be re-published
	freshResource := &api.Resource{
		Meta:         api.Meta{ID: "fresh-1", CreatedAt: time.Now().Add(-1 * time.Minute)},
		Version:      1,
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err = resourcesDao.Create(ctx, freshResource)
	Expect(err).To(BeNil())

	// Resource created 10 minutes ago WITH status — already delivered, should NOT be re-published
	deliveredResource := &api.Resource{
		Meta:         api.Meta{ID: "delivered-1", CreatedAt: time.Now().Add(-10 * time.Minute)},
		Version:      1,
		ConsumerName: "cluster1",
		Source:       "test",
		Status:       map[string]interface{}{"some": "status"},
	}
	_, err = resourcesDao.Create(ctx, deliveredResource)
	Expect(err).To(BeNil())

	// Resource created 10 minutes ago, version > 1, no status — should get UpdateEventType
	updatedResource := &api.Resource{
		Meta:         api.Meta{ID: "updated-1", CreatedAt: time.Now().Add(-10 * time.Minute)},
		Version:      3,
		ConsumerName: "cluster2",
		Source:       "test",
	}
	_, err = resourcesDao.Create(ctx, updatedResource)
	Expect(err).To(BeNil())

	detector.Run(ctx)

	events, err := eventsDao.All(ctx)
	Expect(err).To(BeNil())

	// Should have exactly 2 events: one for stale-1 (Create) and one for updated-1 (Update)
	Expect(events).To(HaveLen(2))

	eventsBySourceID := map[string]*api.Event{}
	for _, e := range events {
		eventsBySourceID[e.SourceID] = e
	}

	Expect(eventsBySourceID).To(HaveKey("stale-1"))
	Expect(eventsBySourceID["stale-1"].EventType).To(Equal(api.CreateEventType))
	Expect(eventsBySourceID["stale-1"].Source).To(Equal("Resources"))

	Expect(eventsBySourceID).To(HaveKey("updated-1"))
	Expect(eventsBySourceID["updated-1"].EventType).To(Equal(api.UpdateEventType))

	Expect(eventsBySourceID).NotTo(HaveKey("fresh-1"))
	Expect(eventsBySourceID).NotTo(HaveKey("delivered-1"))
}
