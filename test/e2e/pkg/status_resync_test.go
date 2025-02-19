package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Status Resync After Restart", Ordered, Label("e2e-tests-status-resync-restart"), func() {
	Context("Resync resource status after maestro server restarts", func() {
		var maestroServerReplicas int
		workName := fmt.Sprintf("work-%s", rand.String(5))
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		work := helper.NewManifestWork(workName, deployName, deployName, 1)
		It("create a resource with source work client", func() {
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if work.CreationTimestamp.Time.IsZero() {
					return fmt.Errorf("work creationTimestamp is empty")
				}

				manifests := work.Status.ResourceStatus.Manifests
				if len(manifests) > 0 && len(manifests[0].StatusFeedbacks.Values) != 0 {
					feedback := manifests[0].StatusFeedbacks.Values
					if feedback[0].Name == "status" {
						feedbackRaw := *feedback[0].Value.JsonRaw
						if strings.Contains(feedbackRaw, "error looking up service account default/nginx") {
							return nil
						}
					}

				}

				return fmt.Errorf("unexpected status, expected error looking up service account default/nginx")
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro server", func() {
			deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Get(ctx, "maestro", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroServerReplicas = int(*deploy.Spec.Replicas)

			// patch maestro server replicas to 0
			deploy, err = serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Patch(ctx, "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "serverTestOpts.kubeClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro server pods
			Eventually(func() error {
				pods, err := serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).List(ctx, metav1.ListOptions{
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

		It("create serviceaccount for deployment", func() {
			_, err := agentTestOpts.kubeClientSet.CoreV1().ServiceAccounts("default").Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: deployName,
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// delete the nginx deployment to tigger recreating
			err = agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Delete(ctx, deployName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("restart maestro server", func() {
			// patch maestro server replicas back
			deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Patch(ctx, "maestro", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroServerReplicas)), metav1.PatchOptions{
				FieldManager: "serverTestOpts.kubeClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroServerReplicas)))

			// ensure maestro server pod is up and running
			Eventually(func() error {
				pods, err := serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).List(ctx, metav1.ListOptions{
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

		It("ensure the resource status is resynced", func() {
			Eventually(func() error {
				work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if work.CreationTimestamp.Time.IsZero() {
					return fmt.Errorf("work creationTimestamp is empty")
				}

				manifests := work.Status.ResourceStatus.Manifests
				if len(manifests) > 0 && len(manifests[0].StatusFeedbacks.Values) != 0 {
					feedback := manifests[0].StatusFeedbacks.Values
					if feedback[0].Name == "status" {
						feedbackRaw := *feedback[0].Value.JsonRaw
						if !strings.Contains(feedbackRaw, "error looking up service account default/nginx") {
							return nil
						}
					}

				}

				return fmt.Errorf("unexpected status")
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the resource with source work client", func() {
			// note: wait some time to ensure source work client is connected to the restarted maestro server
			Eventually(func() error {
				return sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

			err := agentTestOpts.kubeClientSet.CoreV1().ServiceAccounts("default").Delete(ctx, deployName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("check the resource deletion via maestro api", func() {
			Eventually(func() error {
				search := fmt.Sprintf("consumer_name = '%s'", agentTestOpts.consumerName)
				gotResourceList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected http code, got %d, expected %d", resp.StatusCode, http.StatusOK)
				}
				if len(gotResourceList.Items) != 0 {
					return fmt.Errorf("expected no resources returned by maestro api")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})
