package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	"github.com/openshift-online/maestro/pkg/db"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
	"github.com/openshift-online/maestro/pkg/services"
)

type unavailableMonitorLockFactory struct{}

func (unavailableMonitorLockFactory) NewAdvisoryLock(context.Context, string, db.LockType) (string, error) {
	return "", nil
}

func (unavailableMonitorLockFactory) NewNonBlockingLock(context.Context, string, db.LockType) (string, bool, error) {
	return "not-acquired", false, nil
}

func (unavailableMonitorLockFactory) Unlock(context.Context, string) {}

func gaugeValue(g prometheus.Gauge) float64 {
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		return -1
	}
	return m.GetGauge().GetValue()
}

func TestStalledDeleteMonitor(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	resourcesDao := mocks.NewResourceDao()
	eventsDao := mocks.NewEventDao()
	resourceService := services.NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourcesDao, services.NewEventService(eventsDao), nil, 60)

	threshold := 5 * time.Minute
	monitor := NewStalledDeleteMonitor(resourceService, dbmocks.NewMockAdvisoryLockFactory(), int(threshold.Seconds()))

	// Resource soft-deleted 10 minutes ago — stalled, older than the threshold.
	stalledResource := &api.Resource{
		Meta:         api.Meta{ID: "stalled-1"},
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err := resourcesDao.Create(ctx, stalledResource)
	Expect(err).To(BeNil())
	Expect(resourcesDao.Delete(ctx, "stalled-1", false)).To(Succeed())
	stalledResource.DeletedAt.Time = time.Now().Add(-10 * time.Minute)

	// Resource soft-deleted 1 minute ago — too young, should not count as stalled.
	freshResource := &api.Resource{
		Meta:         api.Meta{ID: "fresh-1"},
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err = resourcesDao.Create(ctx, freshResource)
	Expect(err).To(BeNil())
	Expect(resourcesDao.Delete(ctx, "fresh-1", false)).To(Succeed())
	freshResource.DeletedAt.Time = time.Now().Add(-1 * time.Minute)

	// Resource not deleted at all — should not count as stalled.
	liveResource := &api.Resource{
		Meta:         api.Meta{ID: "live-1"},
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err = resourcesDao.Create(ctx, liveResource)
	Expect(err).To(BeNil())

	monitor.Run(ctx)

	Expect(gaugeValue(stalledDeleteResourcesGauge)).To(Equal(1.0))
}

func TestStalledDeleteMonitorNoStalledResources(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	resourcesDao := mocks.NewResourceDao()
	eventsDao := mocks.NewEventDao()
	resourceService := services.NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourcesDao, services.NewEventService(eventsDao), nil, 60)

	threshold := 5 * time.Minute
	monitor := NewStalledDeleteMonitor(resourceService, dbmocks.NewMockAdvisoryLockFactory(), int(threshold.Seconds()))

	liveResource := &api.Resource{
		Meta:         api.Meta{ID: "live-1"},
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err := resourcesDao.Create(ctx, liveResource)
	Expect(err).To(BeNil())

	monitor.Run(ctx)

	Expect(gaugeValue(stalledDeleteResourcesGauge)).To(Equal(0.0))
}

func TestStalledDeleteMonitorRefreshesGaugeWithoutLogLock(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	resourcesDao := mocks.NewResourceDao()
	eventsDao := mocks.NewEventDao()
	resourceService := services.NewResourceService(dbmocks.NewMockAdvisoryLockFactory(), resourcesDao, services.NewEventService(eventsDao), nil, 60)
	resource := &api.Resource{
		Meta:         api.Meta{ID: "stalled-without-lock"},
		ConsumerName: "cluster1",
		Source:       "test",
	}
	_, err := resourcesDao.Create(ctx, resource)
	Expect(err).To(BeNil())
	Expect(resourcesDao.Delete(ctx, resource.ID, false)).To(Succeed())
	resource.DeletedAt.Time = time.Now().Add(-10 * time.Minute)

	monitor := NewStalledDeleteMonitor(resourceService, unavailableMonitorLockFactory{}, 5*60)
	stalledDeleteResourcesGauge.Set(0)
	monitor.Run(ctx)

	// The lock only controls log deduplication; every replica must publish its fresh gauge.
	Expect(gaugeValue(stalledDeleteResourcesGauge)).To(Equal(1.0))
}
