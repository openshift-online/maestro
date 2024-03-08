package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

func TestPulseServer(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx := context.Background()

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	// insert two existing instances
	_, err := instanceDao.UpSert(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance1",
		},
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = instanceDao.UpSert(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	instanceID := &h.Env().Config.MessageBroker.ClientID
	Eventually(func() error {
		instances, err := instanceDao.All(ctx)
		if err != nil {
			return err
		}

		if len(instances) != 1 {
			return fmt.Errorf("expected 1 instance, got %d", len(instances))
		}

		instance := instances[0]
		if instance.UpdatedAt.IsZero() {
			return fmt.Errorf("expected instance.UpdatedAt to be non-zero")
		}

		if instance.ID != *instanceID {
			return fmt.Errorf("expected instance.ID to be %s, got %s", *instanceID, instance.ID)
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	clusterName := "cluster1"
	consumer := h.CreateConsumer(clusterName)
	res := h.CreateResource(consumer.ID, 1)
	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.ID, h.Env().Config.MessageBroker.MQTTOptions)
	clientHolder := h.WorkAgentHolder
	informer := clientHolder.ManifestWorkInformer()
	lister := informer.Lister().ManifestWorks(consumer.ID)
	agentWorkClient := clientHolder.ManifestWorks(consumer.ID)
	resourceService := h.Env().Services.Resources()

	var work *workv1.ManifestWork
	Eventually(func() error {
		list, err := lister.List(labels.Everything())
		if err != nil {
			return err
		}

		// ensure there is only one work was synced on the cluster
		if len(list) != 1 {
			return fmt.Errorf("unexpected work list %v", list)
		}

		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, res.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return nil
	}, 3*time.Second, 1*time.Second).Should(Succeed())

	Expect(work).NotTo(BeNil())
	Expect(work.Spec.Workload).NotTo(BeNil())
	Expect(len(work.Spec.Workload.Manifests)).To(Equal(1))

	newWork := work.DeepCopy()
	newWork.Status = workv1.ManifestWorkStatus{
		ResourceStatus: workv1.ManifestResourceStatus{
			Manifests: []workv1.ManifestCondition{
				{
					Conditions: []metav1.Condition{
						{
							Type:   "Applied",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	// only update the status on the agent local part
	Expect(informer.Informer().GetStore().Update(newWork)).NotTo(HaveOccurred())

	// afther the two instances are stale, the current instance will take over the consumer
	// and resync status, then the resource status will be updated finally
	Eventually(func() error {
		newRes, err := resourceService.Get(ctx, res.ID)
		if err != nil {
			return err
		}
		if newRes.Status == nil || len(newRes.Status) == 0 {
			return fmt.Errorf("resource status is empty")
		}
		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	newRes, err := resourceService.Get(ctx, res.ID)
	Expect(err).NotTo(HaveOccurred(), "Error getting resource: %v", err)
	Expect(newRes.Version).To(Equal(res.Version))
}
