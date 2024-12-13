package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

func TestEventServer(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	// insert one existing instances
	_, err := instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance1",
		},
		LastHeartbeat: time.Now(),
	})
	Expect(err).NotTo(HaveOccurred())

	instanceID := &h.Env().Config.MessageBroker.ClientID
	Eventually(func() error {
		instances, err := instanceDao.All(ctx)
		if err != nil {
			return err
		}

		if len(instances) != 2 {
			return fmt.Errorf("expected 1 instance, got %d", len(instances))
		}

		var instance *api.ServerInstance
		for _, i := range instances {
			if i.ID == *instanceID {
				instance = i
			}
		}

		if instance.LastHeartbeat.IsZero() {
			return fmt.Errorf("expected instance.LastHeartbeat to be non-zero")
		}

		if !instance.Ready {
			return fmt.Errorf("expected instance.Ready to be true")
		}

		if instance.ID != *instanceID {
			return fmt.Errorf("expected instance.ID to be %s, got %s", *instanceID, instance.ID)
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// the cluster1 name cannot be changed, because consistent hash makes it allocate to different instance.
	// the case here we want to the new consumer allocate to new instance(cluster1) which is a fake instance.
	// after 3*pulseInterval (3s), it will relocate to maestro instance.
	clusterName := "cluster1"
	consumer := h.CreateConsumer(clusterName)

	// insert a new instance with the same name to consumer name
	// to make sure the consumer is hashed to the new instance firstly.
	// after the new instance is stale after 3*pulseInterval (3s), the current
	// instance will take over the consumer and resync the resource status.
	_, err = instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: clusterName,
		},
		LastHeartbeat: time.Now(),
		Ready:         true,
	})
	Expect(err).NotTo(HaveOccurred())

	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.CreateResource(consumer.Name, deployName, 1)
	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name, false)
	clientHolder := h.WorkAgentHolder
	informer := h.WorkAgentInformer
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)
	resourceService := h.Env().Services.Resources()

	var work *workv1.ManifestWork
	Eventually(func() error {
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

	// after the instance ("cluster") is stale, the current instance ("maestro") will take over the consumer
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
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	newRes, err := resourceService.Get(ctx, res.ID)
	Expect(err).NotTo(HaveOccurred(), "Error getting resource: %v", err)
	Expect(newRes.Version).To(Equal(res.Version))
}
