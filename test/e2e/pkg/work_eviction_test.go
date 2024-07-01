package e2e_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// go test -v ./test/e2e/pkg -args -api-server=$api_server -consumer-name=$consumer.Name -consumer-kubeconfig=$consumer_kubeconfig -ginkgo.focus "Applied Manifestwork Eviction"
var _ = Describe("Applied Manifestwork Eviction", Ordered, Label("e2e-tests-work-eviction"), func() {
	var resource *openapi.Resource
	var maestroServerReplicas int

	Context("Agent Appliedmanifestwork Eviction Grace Period Tests", func() {
		It("post the nginx resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())
			Expect(*resource.Version).To(Equal(int32(1)))

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro server", func() {
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Get(ctx, "maestro", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroServerReplicas = int(*deploy.Spec.Replicas)

			// patch maestro server replicas to 0
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro server pods
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
					LabelSelector: "app=maestro",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) > 0 {
					return fmt.Errorf("maestro server pods still running")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("restart maestro agent", func() {
			// patch maestro agent to restart it
			restartPatchData := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Patch(ctx, "maestro-agent", types.StrategicMergePatchType, []byte(restartPatchData), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro agent is ready
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Get(ctx, "maestro-agent", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deploy != nil && deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
					return nil
				}
				return fmt.Errorf("maestro agent deploy not ready")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx resource is evicted", func() {
			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("start maestro server", func() {
			// patch maestro server replicas to 1
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroServerReplicas)), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroServerReplicas)))

			// ensure maestro server pod is up and running
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
					LabelSelector: "app=maestro",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != maestroServerReplicas {
					return fmt.Errorf("unexpected maestro server pod count, expected %d, got %d", maestroServerReplicas, len(pods.Items))
				}
				for _, pod := range pods.Items {
					if pod.Status.Phase != "Running" {
						return fmt.Errorf("maestro server pod not in running state")
					}
					if pod.Status.ContainerStatuses[0].State.Running == nil {
						return fmt.Errorf("maestro server container not in running state")
					}
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("restart maestro agent again", func() {
			// patch maestro agent to restart it
			restartPatchData := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Patch(ctx, "maestro-agent", types.StrategicMergePatchType, []byte(restartPatchData), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro agent is ready
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Get(ctx, "maestro-agent", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deploy != nil && deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
					return nil
				}
				return fmt.Errorf("maestro agent deploy not ready")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx resource is resynced", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})

