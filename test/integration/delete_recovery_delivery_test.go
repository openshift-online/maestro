package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

type deleteDeliveryAttempt struct {
	eventID string
	err     error
}

type observedDeleteServer struct {
	server.EventServer
	mu       sync.Mutex
	attempts []deleteDeliveryAttempt
}

func (s *observedDeleteServer) OnDelete(ctx context.Context, resourceID string) error {
	err := s.EventServer.OnDelete(ctx, resourceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts = append(s.attempts, deleteDeliveryAttempt{
		eventID: fmt.Sprint(ctx.Value(controllers.EventID)),
		err:     err,
	})
	return err
}

func (s *observedDeleteServer) snapshot() []deleteDeliveryAttempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]deleteDeliveryAttempt(nil), s.attempts...)
}

// subscribeDeleteDelivery receives real broker traffic without the work client's
// automatic resync or ResourceDeleted acknowledgement masking event delivery.
func subscribeDeleteDelivery(t *testing.T, ctx context.Context, h *test.Helper, consumer string) (pbv1.CloudEventService_SubscribeClient, context.CancelFunc) {
	t.Helper()
	subCtx, cancel := context.WithCancel(ctx)
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%s", h.Env().Config.HTTPServer.Hostname, h.Env().Config.GRPCServer.BrokerBindPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		cancel()
		Expect(conn.Close()).To(Succeed())
	})
	stream, err := pbv1.NewCloudEventServiceClient(conn).Subscribe(subCtx, &pbv1.SubscriptionRequest{
		ClusterName: consumer,
		DataType:    workpayload.ManifestBundleEventDataType.String(),
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = stream.Header()
	Expect(err).NotTo(HaveOccurred())
	return stream, cancel
}

func receiveDeleteDelivery(ctx context.Context, stream pbv1.CloudEventService_SubscribeClient, resourceID, consumer string) {
	message, err := stream.Recv()
	Expect(err).NotTo(HaveOccurred())
	evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(message))
	Expect(err).NotTo(HaveOccurred())
	eventType, err := cetypes.ParseCloudEventsType(evt.Type())
	Expect(err).NotTo(HaveOccurred())
	Expect(eventType.Action).To(Equal(cetypes.DeleteRequestAction), "not a reconnect/spec resync response")
	Expect(evt.Extensions()[cetypes.ExtensionResourceID]).To(Equal(resourceID))
	Expect(evt.Extensions()[cetypes.ExtensionClusterName]).To(Equal(consumer))
	Expect(evt.Extensions()).To(HaveKey(cetypes.ExtensionDeletionTimestamp))
}

func startDeleteDeliveryController(t *testing.T, ctx context.Context, h *test.Helper, observed *observedDeleteServer) (*controllers.KindControllerManager, func()) {
	t.Helper()
	controllerCtx, cancel := context.WithCancel(ctx)
	s := server.NewControllersServer(controllerCtx, observed, h.EventFilter)
	Expect(s.StaleDeleteDetector).To(BeNil(), "retirement must actually be disabled in server wiring")
	Expect(s.DeleteRecovery).NotTo(BeNil(), "disabling retirement must not disable recovery")
	// Step scheduler eligibility explicitly; keep the production event startup,
	// listener, filter and rate-limited queue running normally.
	s.DeleteRecovery = nil
	s.UndeliveredDetector = nil
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Start(controllerCtx)
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			<-done
			s.KindControllerManager.Queue().ShutDownWithDrain()
		})
	}
	t.Cleanup(stop)
	return s.KindControllerManager, stop
}

func TestDeleteRecoveryWithoutRetirement(t *testing.T) {
	for _, scenario := range []string{"publication-retry", "controller-restart", "consumer-reconnect"} {
		t.Run(scenario, func(t *testing.T) {
			h, _ := test.RegisterIntegration(t)
			if h.Broker != "grpc" {
				t.Skip("raw gRPC subscription isolates delivery from agent resync and acknowledgement")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			settings := h.Env().Config.EventServer
			oldThreshold := settings.StaleDeleteEventThreshold
			settings.StaleDeleteEventThreshold = 0
			defer func() { settings.StaleDeleteEventThreshold = oldThreshold }()

			consumer, err := h.CreateConsumer("delivery-" + uuid.NewString())
			Expect(err).NotTo(HaveOccurred())
			resource, err := h.CreateResource(uuid.NewString(), consumer.Name, "nginx", "default", 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(h.Env().Services.Resources().MarkAsDeleting(ctx, resource.ID)).To(Succeed())
			conn := h.DBFactory.New(ctx)
			Expect(conn.Exec("UPDATE resources SET deleted_at = ? WHERE id = ?",
				time.Now().Add(-48*time.Hour), resource.ID).Error).To(Succeed())
			// Begin with a scheduler-created event, not the initial API deletion.
			Expect(conn.Exec("DELETE FROM events WHERE source_id = ?", resource.ID).Error).To(Succeed())
			recovery, err := dao.NewDeleteRecovery(h.DBFactory, settings)
			Expect(err).NotTo(HaveOccurred())
			result, err := recovery.Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Published).To(Equal(1))
			var first api.Event
			Expect(conn.Where("source_id = ?", resource.ID).First(&first).Error).To(Succeed())
			Expect(conn.Exec("UPDATE events SET created_at = ? WHERE id = ?",
				time.Now().Add(-48*time.Hour), first.ID).Error).To(Succeed())

			stream, disconnect := subscribeDeleteDelivery(t, ctx, h, consumer.Name)
			Eventually(func() (bool, error) {
				return h.EventServer.PredicateEvent(ctx, first.ID)
			}, 10*time.Second, 10*time.Millisecond).Should(BeTrue())
			if scenario == "consumer-reconnect" {
				disconnect()
				Eventually(func() (bool, error) {
					return h.EventServer.PredicateEvent(ctx, first.ID)
				}, 10*time.Second, 10*time.Millisecond).Should(BeFalse())
			} else {
				// A deterministic encoding failure exercises the real OnDelete
				// error path without timing a network outage.
				Expect(conn.Exec("UPDATE resources SET payload = '{}' WHERE id = ?", resource.ID).Error).To(Succeed())
			}
			observed := &observedDeleteServer{EventServer: h.EventServer}
			manager, stop := startDeleteDeliveryController(t, ctx, h, observed)
			defer stop()
			Eventually(func() int { return manager.Queue().NumRequeues(first.ID) },
				10*time.Second, 10*time.Millisecond).Should(BeNumerically(">=", 2),
				"startup must find and retry the existing event without a new notification")
			if scenario == "consumer-reconnect" {
				Expect(observed.snapshot()).To(BeEmpty(), "broker predicate must prevent publication while disconnected")
			} else {
				attempts := observed.snapshot()
				Expect(len(attempts)).To(BeNumerically(">=", 2))
				for _, attempt := range attempts {
					Expect(attempt.eventID).To(Equal(first.ID))
					Expect(attempt.err).To(MatchError(ContainSubstring("empty jsonmap")))
				}
			}

			for range 3 {
				makeRecoveryDue(conn)
				result, err = recovery.Run(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Consumers).To(Equal(1))
				Expect(result.Published).To(BeZero(), "pending delivery coalesces scheduler republication")
			}
			var pending []api.Event
			Expect(conn.Where("source_id = ?", resource.ID).Find(&pending).Error).To(Succeed())
			Expect(pending).To(HaveLen(1))
			Expect(pending[0].ID).To(Equal(first.ID))
			Expect(pending[0].ReconciledDate).To(BeNil())

			if scenario == "controller-restart" {
				stop()
				observed = &observedDeleteServer{EventServer: h.EventServer}
				manager, stop = startDeleteDeliveryController(t, ctx, h, observed)
				defer stop()
				Eventually(func() int { return manager.Queue().NumRequeues(first.ID) },
					10*time.Second, 10*time.Millisecond).Should(BeNumerically(">=", 2),
					"a fresh controller must discover the durable pending event on startup")
			}
			if scenario == "consumer-reconnect" {
				stream, _ = subscribeDeleteDelivery(t, ctx, h, consumer.Name)
			} else {
				Expect(conn.Model(&api.Resource{}).Unscoped().Where("id = ?", resource.ID).
					Update("payload", resource.Payload).Error).To(Succeed())
			}
			receiveDeleteDelivery(ctx, stream, resource.ID, consumer.Name)
			Eventually(func() *time.Time {
				event, svcErr := h.Env().Services.Events().Get(ctx, first.ID)
				Expect(svcErr).To(BeNil())
				return event.ReconciledDate
			}, 10*time.Second, 10*time.Millisecond).ShouldNot(BeNil())
			attempts := observed.snapshot()
			Expect(attempts).NotTo(BeEmpty())
			for _, attempt := range attempts {
				Expect(attempt.eventID).To(Equal(first.ID), "delivery retries reuse the database event")
			}
			Expect(attempts[len(attempts)-1].err).NotTo(HaveOccurred())

			makeRecoveryDue(conn)
			result, err = recovery.Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Published).To(Equal(1), "successful publication without acknowledgement permits a fresh event")
			var next api.Event
			Expect(conn.Where("source_id = ? AND id <> ?", resource.ID, first.ID).First(&next).Error).To(Succeed())
			receiveDeleteDelivery(ctx, stream, resource.ID, consumer.Name)
			Eventually(func() []deleteDeliveryAttempt { return observed.snapshot() },
				10*time.Second, 10*time.Millisecond).Should(ContainElement(deleteDeliveryAttempt{eventID: next.ID}))
			retained, svcErr := h.Env().Services.Resources().Get(ctx, resource.ID)
			Expect(svcErr).To(BeNil())
			Expect(retained.DeletedAt.Valid).To(BeTrue(), "delivery without ResourceDeleted must preserve the tombstone")
			var acknowledgements int64
			Expect(conn.Model(&api.StatusEvent{}).Where("resource_id = ?", resource.ID).Count(&acknowledgements).Error).To(Succeed())
			Expect(acknowledgements).To(BeZero())
		})
	}
}
