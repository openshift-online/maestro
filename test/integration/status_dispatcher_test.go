package integration

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
	prommodel "github.com/prometheus/client_model/go"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

func TestStatusDispatcher(t *testing.T) {
	broker := os.Getenv("BROKER")
	if broker == "grpc" {
		t.Skip("StatusDispatcher is not supported with gRPC broker")
	}

	h, _ := test.RegisterIntegration(t)
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
	families := getServerMetrics(t, "http://localhost:8080/metrics")
	labels := []*prommodel.LabelPair{
		{Name: strPtr("source"), Value: strPtr("maestro")},
		{Name: strPtr("original_source"), Value: strPtr("none")},
		{Name: strPtr("cluster"), Value: strPtr(consumer1)},
		{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
		{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceStatus))},
		{Name: strPtr("action"), Value: strPtr("resync_request")},
	}
	checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 1.0)
	labels = []*prommodel.LabelPair{
		{Name: strPtr("source"), Value: strPtr("maestro")},
		{Name: strPtr("original_source"), Value: strPtr("none")},
		{Name: strPtr("cluster"), Value: strPtr(consumer2)},
		{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
		{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceStatus))},
		{Name: strPtr("action"), Value: strPtr("resync_request")},
	}
	checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 2.0)
}
