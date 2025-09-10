package integration

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestListSyncWorks(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	consumer1, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())

	consumer2, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())

	consumer3, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())

	// source maestro-1 has one works
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	work1, err := h.NewResource(consumer1.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())

	work1.ID = uuid.NewString()
	work1.Source = "maestro-1"

	resourceService := h.Env().Services.Resources()
	_, err = resourceService.Create(ctx, work1)
	Expect(err).NotTo(HaveOccurred())

	// source maestro-2 has two works
	work2, err := h.NewResource(consumer1.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())

	work2.ID = uuid.NewString()
	work2.Source = "maestro-2"

	_, err = resourceService.Create(ctx, work2)
	Expect(err).NotTo(HaveOccurred())

	work3, err := h.NewResource(consumer2.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())

	work3.ID = uuid.NewString()
	work3.Source = "maestro-2"

	_, err = resourceService.Create(ctx, work3)
	Expect(err).NotTo(HaveOccurred())

	work4, err := h.NewResource(consumer3.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())

	work4.ID = uuid.NewString()
	work4.Source = "maestro-2"

	_, err = resourceService.Create(ctx, work4)
	Expect(err).NotTo(HaveOccurred())

	search1 := grpcsource.ToSyncSearch("maestro-1", []string{consumer1.Name})
	works, _, err := grpcsource.PageList(ctx, client, search1, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(works.Items)).To(Equal(1))

	search2 := grpcsource.ToSyncSearch("maestro-2", []string{consumer1.Name, consumer2.Name})
	works, _, err = grpcsource.PageList(ctx, client, search2, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(works.Items)).To(Equal(2))

	// has a watcher that watches all namespaces
	search3 := grpcsource.ToSyncSearch("maestro-2", []string{consumer1.Name, metav1.NamespaceAll})
	works, _, err = grpcsource.PageList(ctx, client, search3, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(works.Items)).To(Equal(3))
}
