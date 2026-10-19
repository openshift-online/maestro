package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/db/db_session"
	"github.com/openshift-online/maestro/pkg/handlers"
	"github.com/openshift-online/maestro/pkg/services"
	"github.com/openshift-online/maestro/test"
)

// cascadeConsumerService enables the real transactional cascade with a one-hour age guard.
func cascadeConsumerService(h *test.Helper) services.ConsumerService {
	return services.NewConsumerServiceWithCascade(dao.NewConsumerDao(&h.DBFactory),
		dao.NewConsumerCascadeDao(h.DBFactory), 3600)
}

// cascadeResource creates a watchable resource and optionally backdates its soft deletion.
func cascadeResource(h *test.Helper, consumer string, age time.Duration) *api.Resource {
	ctx := context.Background()
	resource, err := h.NewResource(uuid.NewString(), consumer, "work-"+uuid.NewString(), "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())
	// Reuse the suite's standard source label so this watcher does not leave a
	// new zero-valued metric series that breaks later exact metric assertions.
	resource.Source = "maestro"
	spec, err := api.JSONMAPToCloudEvent(resource.Payload)
	Expect(err).NotTo(HaveOccurred())
	workMeta, err := json.Marshal(metav1.ObjectMeta{Name: "work-" + resource.ID, Namespace: consumer})
	Expect(err).NotTo(HaveOccurred())
	spec.SetExtension(cetypes.ExtensionWorkMeta, string(workMeta))
	resource.Payload, err = api.CloudEventToJSONMap(spec)
	Expect(err).NotTo(HaveOccurred())
	resource, svcErr := h.Env().Services.Resources().Create(ctx, resource)
	Expect(svcErr).To(BeNil())
	if age != 0 {
		Expect(h.Env().Services.Resources().MarkAsDeleting(ctx, resource.ID)).To(BeNil())
		Expect(h.DBFactory.New(ctx).Exec("UPDATE resources SET deleted_at = ? WHERE id = ?",
			time.Now().Add(-age), resource.ID).Error).To(Succeed())
	}
	return resource
}

// cascadeMetricValue reads a counter or gauge, optionally selecting its source label.
func cascadeMetricValue(name string, source string) float64 {
	families, err := prometheus.DefaultGatherer.Gather()
	Expect(err).NotTo(HaveOccurred())
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			matches := source == ""
			for _, label := range metric.Label {
				if label.GetName() == "source" && label.GetValue() == source {
					matches = true
				}
			}
			if matches {
				if metric.Counter != nil {
					return metric.Counter.GetValue()
				}
				return metric.GetGauge().GetValue()
			}
		}
	}
	return 0
}

// TestConsumerCascadeSafety verifies refusals leave every resource and event untouched.
func TestConsumerCascadeSafety(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		age     time.Duration
	}{
		{"flag off preserves old tombstones", false, 2 * time.Hour},
		{"live resource", true, 0},
		{"recent deletion", true, time.Minute},
		{"future deletion", true, -time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := test.RegisterIntegration(t)
			consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
			Expect(err).NotTo(HaveOccurred())
			cascadeResource(h, consumer.Name, 3*time.Hour)
			cascadeResource(h, consumer.Name, tc.age)
			service := services.NewConsumerService(dao.NewConsumerDao(&h.DBFactory))
			if tc.enabled {
				service = cascadeConsumerService(h)
			}
			before := cascadeMetricValue("consumer_delete_cascaded_resources_total", "")
			handler := handlers.NewConsumerHandler(service, h.Env().Services.Resources(), h.Env().Services.Generic())
			request := mux.SetURLVars(httptest.NewRequest(http.MethodDelete, "/consumers/"+consumer.ID, nil), map[string]string{"id": consumer.ID})
			recorder := httptest.NewRecorder()
			handler.Delete(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusForbidden))
			var count int64
			conn := h.DBFactory.New(context.Background())
			Expect(conn.Raw("SELECT count(*) FROM resources WHERE consumer_name = ?", consumer.Name).Scan(&count).Error).To(Succeed())
			Expect(count).To(Equal(int64(2)))
			Expect(conn.Model(&api.StatusEvent{}).Where("status_event_type = ?", api.StatusDeleteEventType).Count(&count).Error).To(Succeed())
			Expect(count).To(BeZero())
			_, err = dao.NewConsumerDao(&h.DBFactory).Get(context.Background(), consumer.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(cascadeMetricValue("consumer_delete_cascaded_resources_total", "")).To(Equal(before))
		})
	}
}

// TestConsumerCascadeCommitsAndCleansRecovery checks atomic finalization, recovery cleanup and idempotence.
func TestConsumerCascadeCommitsAndCleansRecovery(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()
	consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	for range 3 {
		cascadeResource(h, consumer.Name, 2*time.Hour)
	}
	other, err := h.CreateConsumer("other-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	cascadeResource(h, other.Name, 2*time.Hour)
	conn := h.DBFactory.New(ctx)
	Expect(conn.Exec("UPDATE resources SET delete_retry_at = clock_timestamp() WHERE consumer_name = ?", consumer.Name).Error).To(Succeed())
	Expect(conn.Exec("INSERT INTO delete_recovery_consumers (consumer_name, next_at) VALUES (?, clock_timestamp())", consumer.Name).Error).To(Succeed())
	before := cascadeMetricValue("consumer_delete_cascaded_resources_total", "")
	service := cascadeConsumerService(h)
	Expect(service.Delete(ctx, consumer.ID)).To(BeNil())
	Expect(service.Delete(ctx, consumer.ID)).To(BeNil(), "repeated deletion must not emit duplicate terminal events")
	var count int64
	for _, query := range []string{
		"SELECT count(*) FROM resources WHERE consumer_name = ?",
		"SELECT count(*) FROM delete_recovery_consumers WHERE consumer_name = ?",
		"SELECT count(*) FROM consumers WHERE name = ?",
	} {
		Expect(conn.Raw(query, consumer.Name).Scan(&count).Error).To(Succeed())
		Expect(count).To(BeZero())
	}
	Expect(conn.Raw("SELECT count(*) FROM status_events WHERE status_event_type = ?", api.StatusDeleteEventType).Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(3)))
	Expect(conn.Raw(`SELECT count(*) FROM events WHERE source = 'Resources' AND event_type = 'Delete'
		AND reconciled_date IS NULL AND source_id IN (SELECT id FROM resources WHERE consumer_name = ?)`, other.Name).Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(1)), "other consumers must not be affected")
	Expect(conn.Raw("SELECT count(*) FROM events WHERE source = 'Resources' AND event_type = 'Delete' AND reconciled_date IS NOT NULL").Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(3)))
	Expect(cascadeMetricValue("consumer_delete_cascaded_resources_total", "") - before).To(Equal(float64(3)))
}

// TestConsumerCascadeRollback injects write and deferred commit failures and verifies a clean retry.
func TestConsumerCascadeRollback(t *testing.T) {
	for _, phase := range []string{"midway", "commit"} {
		t.Run(phase, func(t *testing.T) {
			h, _ := test.RegisterIntegration(t)
			ctx := context.Background()
			consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
			Expect(err).NotTo(HaveOccurred())
			ids := []string{cascadeResource(h, consumer.Name, 2*time.Hour).ID,
				cascadeResource(h, consumer.Name, 2*time.Hour).ID}
			sort.Strings(ids)
			conn := h.DBFactory.New(ctx)
			Expect(conn.Exec("CREATE TABLE consumer_cascade_failure_test (resource_id text PRIMARY KEY)").Error).To(Succeed())
			t.Cleanup(func() {
				Expect(conn.Exec(`DROP TRIGGER IF EXISTS fail_consumer_cascade_test ON status_events;
					DROP FUNCTION IF EXISTS fail_consumer_cascade_test();
					DROP TABLE IF EXISTS consumer_cascade_failure_test`).Error).To(Succeed())
			})
			Expect(conn.Exec("INSERT INTO consumer_cascade_failure_test (resource_id) VALUES (?)", ids[1]).Error).To(Succeed())
			Expect(conn.Exec(`CREATE FUNCTION fail_consumer_cascade_test() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN
					IF EXISTS (SELECT 1 FROM consumer_cascade_failure_test WHERE resource_id = NEW.resource_id) THEN
						RAISE EXCEPTION 'injected cascade failure';
					END IF;
					RETURN NEW;
				END $$`).Error).To(Succeed())
			if phase == "commit" {
				Expect(conn.Exec(`CREATE CONSTRAINT TRIGGER fail_consumer_cascade_test AFTER INSERT ON status_events
					DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_consumer_cascade_test()`).Error).To(Succeed())
			} else {
				Expect(conn.Exec(`CREATE TRIGGER fail_consumer_cascade_test BEFORE INSERT ON status_events
					FOR EACH ROW EXECUTE FUNCTION fail_consumer_cascade_test()`).Error).To(Succeed())
			}
			before := cascadeMetricValue("consumer_delete_cascaded_resources_total", "")
			service := cascadeConsumerService(h)
			Expect(service.Delete(ctx, consumer.ID)).NotTo(BeNil())
			var count int64
			Expect(conn.Raw("SELECT count(*) FROM resources WHERE consumer_name = ?", consumer.Name).Scan(&count).Error).To(Succeed())
			Expect(count).To(Equal(int64(2)))
			Expect(conn.Raw("SELECT count(*) FROM status_events").Scan(&count).Error).To(Succeed())
			Expect(count).To(BeZero())
			Expect(conn.Raw("SELECT count(*) FROM events WHERE event_type = 'Delete' AND reconciled_date IS NULL").Scan(&count).Error).To(Succeed())
			Expect(count).To(Equal(int64(2)))
			_, err = dao.NewConsumerDao(&h.DBFactory).Get(ctx, consumer.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(cascadeMetricValue("consumer_delete_cascaded_resources_total", "")).To(Equal(before))
			Expect(conn.Exec("DROP TRIGGER fail_consumer_cascade_test ON status_events").Error).To(Succeed())
			Expect(service.Delete(ctx, consumer.ID)).To(BeNil())
			Expect(conn.Raw("SELECT count(*) FROM status_events").Scan(&count).Error).To(Succeed())
			Expect(count).To(Equal(int64(2)))
		})
	}
}

// TestConsumerCascadeGRPCWatcher verifies committed finalization reaches the SDK watcher as Deleted.
func TestConsumerCascadeGRPCWatcher(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel(); time.Sleep(time.Second) }()
	consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	resource := cascadeResource(h, consumer.Name, 2*time.Hour)
	logger, err := logging.NewStdLoggerBuilder().Build()
	Expect(err).NotTo(HaveOccurred())
	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(ctx, logger, client,
		&grpcoptions.GRPCOptions{Dialer: &grpcoptions.GRPCDialer{URL: "localhost:" + h.Env().Config.GRPCServer.ServerBindPort}}, resource.Source)
	Expect(err).NotTo(HaveOccurred())
	watcher, err := workClient.ManifestWorks(consumer.Name).Watch(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	defer watcher.Stop()
	Eventually(func() float64 {
		return cascadeMetricValue("grpc_server_registered_source_clients", resource.Source)
	}, 10*time.Second, 50*time.Millisecond).Should(Equal(float64(1)))
	h.StartControllerManager(ctx)
	Eventually(func() int64 {
		var count int64
		Expect(h.DBFactory.New(ctx).Raw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND query = 'LISTEN "status_events"'`).Scan(&count).Error).To(Succeed())
		return count
	}, 5*time.Second, 20*time.Millisecond).Should(BeNumerically(">", 0))
	Expect(cascadeConsumerService(h).Delete(ctx, consumer.ID)).To(BeNil())
	var deleted watch.Event
	Eventually(func() bool {
		select {
		case evt, ok := <-watcher.ResultChan():
			if !ok {
				return false
			}
			if evt.Type == watch.Deleted {
				deleted = evt
				return true
			}
		default:
		}
		return false
	}, 15*time.Second, 50*time.Millisecond).Should(BeTrue(), "a committed cascade must reach the real gRPC work watcher as Deleted")
	deletedWork, ok := deleted.Object.(*workv1.ManifestWork)
	Expect(ok).To(BeTrue())
	Expect(string(deletedWork.UID)).To(Equal(resource.ID))
	Expect(deletedWork.Name).To(Equal("work-" + resource.ID))
	Expect(deletedWork.Namespace).To(Equal(consumer.Name))
	Expect(meta.IsStatusConditionTrue(deletedWork.Status.Conditions, common.ResourceDeleted)).To(BeTrue())
}

// terminalAck builds an agent acknowledgement carrying a ResourceDeleted condition.
func terminalAck(resource *api.Resource, consumerName string) *api.Resource {
	evt := cloudevents.NewEvent()
	evt.SetID(uuid.NewString())
	evt.SetSource("agent")
	evt.SetType((&cetypes.CloudEventsType{CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource: cetypes.SubResourceStatus, Action: cetypes.UpdateRequestAction}).String())
	Expect(evt.SetData(cloudevents.ApplicationJSON, &workpayload.ManifestBundleStatus{Conditions: []metav1.Condition{
		{Type: common.ResourceDeleted, Status: metav1.ConditionTrue, Reason: "Deleted", LastTransitionTime: metav1.Now()},
	}})).To(Succeed())
	status, err := api.CloudEventToJSONMap(&evt)
	Expect(err).NotTo(HaveOccurred())
	return &api.Resource{Meta: api.Meta{ID: resource.ID}, ConsumerName: consumerName, Version: resource.Version, Status: status}
}

// TestConsumerCascadeAcknowledgementRace checks acknowledgement and cascade emit one terminal event.
func TestConsumerCascadeAcknowledgementRace(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for range 10 {
		consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
		Expect(err).NotTo(HaveOccurred())
		resource := cascadeResource(h, consumer.Name, 2*time.Hour)
		ack := terminalAck(resource, consumer.Name)
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			results <- server.HandleStatusUpdate(ctx, ack, h.Env().Services.Resources(), h.Env().Services.StatusEvents(), dao.NewConsumerCascadeDao(h.DBFactory))
		}()
		go func() {
			<-start
			if svcErr := cascadeConsumerService(h).Delete(ctx, consumer.ID); svcErr != nil {
				results <- fmt.Errorf("cascade: %s", svcErr.Error())
			} else {
				results <- nil
			}
		}()
		close(start)
		for range 2 {
			var err error
			Eventually(results, 5*time.Second).Should(Receive(&err))
			Expect(err).NotTo(HaveOccurred())
		}
		var count int64
		Expect(h.DBFactory.New(ctx).Raw("SELECT count(*) FROM status_events WHERE resource_id = ? AND status_event_type = ?",
			resource.ID, api.StatusDeleteEventType).Scan(&count).Error).To(Succeed())
		Expect(count).To(Equal(int64(1)), "acknowledgement and cascade must not emit duplicate terminal events")
		_, svcErr := h.Env().Services.Resources().Get(ctx, resource.ID)
		Expect(svcErr).NotTo(BeNil())
		Expect(svcErr.Is404()).To(BeTrue())
	}
}

// TestTerminalAckSmallPoolContention verifies that concurrent terminal acknowledgements and
// a delete cascade complete when workers outnumber a small connection pool. The consumer
// advisory lock is acquired on the transaction that performs the protected writes, so the
// lock holder never waits for a second pooled connection: pool exhaustion by lock waiters
// cannot starve the holder. With the lock held on a separate connection instead, the holder
// deadlocks here waiting for a query connection and every worker fails on the context deadline.
func TestTerminalAckSmallPoolContention(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	const acks = 8
	resources := make([]*api.Resource, acks)
	for i := range acks {
		resources[i] = cascadeResource(h, consumer.Name, 0)
	}
	// A live resource that is never acknowledged keeps the cascade deterministically
	// refused, so it participates in the consumer lock contention without deleting the consumer.
	blocker := cascadeResource(h, consumer.Name, 0)

	// A dedicated session factory with a small pool over the same database: the workers
	// outnumber the pool while the suite's shared pool and its background users are untouched.
	var smallPool db.SessionFactory = db_session.NewTestFactory(h.Env().Config.Database)
	defer smallPool.Close()
	smallPool.DirectDB().SetMaxOpenConns(3)
	finalizer := dao.NewConsumerCascadeDao(smallPool)
	cascade := services.NewConsumerServiceWithCascade(dao.NewConsumerDao(&smallPool),
		dao.NewConsumerCascadeDao(smallPool), 3600)

	start := make(chan struct{})
	results := make(chan error, acks+1)
	for i := range acks {
		go func(i int) {
			<-start
			results <- server.HandleStatusUpdate(ctx, terminalAck(resources[i], consumer.Name),
				h.Env().Services.Resources(), h.Env().Services.StatusEvents(), finalizer)
		}(i)
	}
	go func() {
		<-start
		if svcErr := cascade.Delete(ctx, consumer.ID); svcErr != nil && svcErr.HttpCode != http.StatusForbidden {
			results <- fmt.Errorf("cascade: %s", svcErr.Error())
		} else {
			results <- nil
		}
	}()
	close(start)
	for range acks + 1 {
		var err error
		Eventually(results, 30*time.Second).Should(Receive(&err))
		Expect(err).NotTo(HaveOccurred())
	}

	// every acknowledged resource was finalized exactly once
	for _, resource := range resources {
		var count int64
		Expect(h.DBFactory.New(ctx).Raw("SELECT count(*) FROM status_events WHERE resource_id = ? AND status_event_type = ?",
			resource.ID, api.StatusDeleteEventType).Scan(&count).Error).To(Succeed())
		Expect(count).To(Equal(int64(1)), "each acknowledgement must emit exactly one terminal event")
		_, svcErr := h.Env().Services.Resources().Get(ctx, resource.ID)
		Expect(svcErr).NotTo(BeNil())
		Expect(svcErr.Is404()).To(BeTrue())
	}
	_, svcErr := h.Env().Services.Resources().Get(ctx, blocker.ID)
	Expect(svcErr).To(BeNil())
	_, err = dao.NewConsumerDao(&h.DBFactory).Get(ctx, consumer.ID)
	Expect(err).NotTo(HaveOccurred())
}

// TestTerminalAckWithoutFinalizerSkipsConsumerLock pins the flag-off contract: with a nil
// finalizer (what NewMessageQueueEventServer leaves in place without --consumer-delete-cascade,
// and what its subscription passes through the variadic), a terminal acknowledgement keeps the
// pre-cascade behaviour and does not block on the per-consumer advisory lock.
func TestTerminalAckWithoutFinalizerSkipsConsumerLock(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	resource := cascadeResource(h, consumer.Name, 0)

	// hold the consumer advisory lock the way an enabled cascade would
	locks := db.NewAdvisoryLockFactory(h.DBFactory)
	owner, err := locks.NewAdvisoryLock(ctx, consumer.Name, db.Consumers)
	Expect(err).NotTo(HaveOccurred())
	defer locks.Unlock(ctx, owner)

	// the message-queue subscription passes its nil finalizer through the variadic, so an
	// explicit nil (not a missing argument) must select the legacy path rather than panic
	var finalizer dao.ConsumerCascadeDao
	ackErr := make(chan error, 1)
	go func() {
		ackErr <- server.HandleStatusUpdate(ctx, terminalAck(resource, consumer.Name),
			h.Env().Services.Resources(), h.Env().Services.StatusEvents(), finalizer)
	}()
	Eventually(ackErr, 5*time.Second).Should(Receive(BeNil()), "the uncoordinated path must not wait on the consumer lock")

	_, svcErr := h.Env().Services.Resources().Get(ctx, resource.ID)
	Expect(svcErr).NotTo(BeNil())
	Expect(svcErr.Is404()).To(BeTrue())
}

// TestConsumerCascadeConcurrentCreate verifies a new resource can pass its FK check under NO KEY UPDATE.
// The final consumer DELETE must then refuse and roll back the complete cascade.
func TestConsumerCascadeConcurrentCreate(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	consumer, err := h.CreateConsumer("cascade-" + uuid.NewString())
	Expect(err).NotTo(HaveOccurred())
	old := cascadeResource(h, consumer.Name, 2*time.Hour)
	conn := h.DBFactory.New(ctx)
	gate := conn.Begin()
	Expect(gate.Error).NotTo(HaveOccurred())
	defer gate.Rollback()
	Expect(gate.Exec("SELECT pg_advisory_xact_lock(1644001)").Error).To(Succeed())
	Expect(conn.Exec(`CREATE FUNCTION pause_consumer_cascade_test() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(1644001); RETURN NEW; END $$;
		CREATE TRIGGER pause_consumer_cascade_test BEFORE INSERT ON status_events
		FOR EACH ROW EXECUTE FUNCTION pause_consumer_cascade_test()`).Error).To(Succeed())
	t.Cleanup(func() {
		Expect(h.DBFactory.New(context.Background()).Exec(`DROP TRIGGER IF EXISTS pause_consumer_cascade_test ON status_events;
			DROP FUNCTION IF EXISTS pause_consumer_cascade_test()`).Error).To(Succeed())
	})
	before := cascadeMetricValue("consumer_delete_cascaded_resources_total", "")
	result := make(chan int, 1)
	go func() {
		if svcErr := cascadeConsumerService(h).Delete(ctx, consumer.ID); svcErr != nil {
			result <- svcErr.HttpCode
		} else {
			result <- 0
		}
	}()
	Eventually(func() bool {
		var waiting bool
		Expect(conn.Raw("SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND objid = 1644001 AND NOT granted)").Scan(&waiting).Error).To(Succeed())
		return waiting
	}, 5*time.Second, 10*time.Millisecond).Should(BeTrue())
	createCtx, createCancel := context.WithTimeout(ctx, 3*time.Second)
	defer createCancel()
	live, err := h.NewResource(uuid.NewString(), consumer.Name, "new-work", "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())
	_, svcErr := h.Env().Services.Resources().Create(createCtx, live)
	Expect(svcErr).To(BeNil(), "a cascade must not deadlock ordinary FK checks")
	Expect(gate.Rollback().Error).To(Succeed())
	Eventually(result, 5*time.Second).Should(Receive(Equal(http.StatusForbidden)))
	var count int64
	Expect(conn.Raw("SELECT count(*) FROM resources WHERE consumer_name = ?", consumer.Name).Scan(&count).Error).To(Succeed())
	Expect(count).To(Equal(int64(2)))
	Expect(conn.Raw("SELECT count(*) FROM status_events").Scan(&count).Error).To(Succeed())
	Expect(count).To(BeZero(), "failed parent deletion must roll back the terminal event")
	_, svcErr = h.Env().Services.Resources().Get(ctx, old.ID)
	Expect(svcErr).To(BeNil())
	Expect(cascadeMetricValue("consumer_delete_cascaded_resources_total", "")).To(Equal(before))
}
