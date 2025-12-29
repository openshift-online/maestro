package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	cemetrics "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/metrics"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

func TestStatusDispatcher(t *testing.T) {
	broker := os.Getenv("BROKER")
	if broker == "grpc" {
		t.Skip("StatusDispatcher is not supported with gRPC broker")
	}

	h, _ := test.RegisterIntegration(t)

	// reset metrics to avoid interference from other tests
	cemetrics.ResetSourceCloudEventsMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	// create 2 consumers
	consumer1 := "xyzzy"
	consumer2 := "thud"
	_, err := h.CreateConsumer(consumer1)
	Expect(err).NotTo(HaveOccurred())
	_, err = h.CreateConsumer(consumer2)
	Expect(err).NotTo(HaveOccurred())

	// should dispatch to all consumers for current instance
	Eventually(func() bool {
		return h.StatusDispatcher.Dispatch(consumer1) &&
			h.StatusDispatcher.Dispatch(consumer2)
	}, 6*time.Second, 1*time.Second).Should(BeTrue())

	// insert a new instance and healthcheck server will mark it as ready and then add it to the hash ring
	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	_, err = instanceDao.Replace(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance1",
		},
		LastHeartbeat: time.Now(),
		Ready:         true,
	})
	Expect(err).NotTo(HaveOccurred())

	// should dispatch consumer based on the new hash ring
	Eventually(func() bool {
		return h.StatusDispatcher.Dispatch(consumer1) &&
			!h.StatusDispatcher.Dispatch(consumer2)
	}, 5*time.Second, 1*time.Second).Should(BeTrue())

	// finally should dispatch to all consumers for current instance
	// as instance1 will be unready and removed from the hash ring
	Eventually(func() bool {
		return h.StatusDispatcher.Dispatch(consumer1) &&
			h.StatusDispatcher.Dispatch(consumer2)
	}, 6*time.Second, 1*time.Second).Should(BeTrue())

	// check metrics for status resync
	time.Sleep(1 * time.Second)
	expectedMetrics := fmt.Sprintf(`
	# HELP cloudevents_sent_total The total number of CloudEvents sent from source.
	# TYPE cloudevents_sent_total counter
	cloudevents_sent_total{action="resync_request",consumer="%s",source="maestro",subresource="status",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 1
	cloudevents_sent_total{action="resync_request",consumer="%s",source="maestro",subresource="status",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 2
	`, consumer1, consumer2)

	if err := testutil.GatherAndCompare(prometheus.DefaultGatherer,
		strings.NewReader(expectedMetrics), "cloudevents_sent_total"); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}
