package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/ocm-sdk-go/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

var _ = Describe("Status Resync After Restart", Ordered, Label("e2e-tests-status-resync-restart"), func() {
	Context("Resync resource status after maestro server restarts", func() {
		var watcherCtx context.Context
		var watcherCancel context.CancelFunc

		var watcherClient workv1client.WorkV1Interface
		var watchedResult *WatchedResult

		var maestroServerReplicas int
		workName := fmt.Sprintf("work-%s", rand.String(5))
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		work := helper.NewManifestWork(workName, deployName, deployName, 1)

		BeforeAll(func() {
			watcherCtx, watcherCancel = context.WithCancel(ctx)

			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			watcherClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
				ctx,
				logger,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start status watching")
			watcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			watchedResult = StartWatch(watcherCtx, watcher)

			By("create a resource with source work client")
			_, err = watcherClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
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
				work, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
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

			Eventually(func() error {
				if len(watchedResult.WatchedWorks) != 0 {
					return nil
				}
				return fmt.Errorf("no works watched")
			}, 1*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
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
				// (TODO), the maestro pod may be in completed status so we skip the Completed status check here
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodSucceeded {
						continue
					}
					return fmt.Errorf("maestro server pods still running")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			// remove the watched works
			watchedResult.WatchedWorks = []*workv1.ManifestWork{}
		})

		It("create serviceaccount for deployment", func() {
			_, err := agentTestOpts.kubeClientSet.CoreV1().ServiceAccounts("default").Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: deployName,
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// delete the nginx deployment to trigger recreating
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

				// (TODO), the maestro pod may be in completed status so we skip the Completed status check here
				availablePods := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodSucceeded {
						continue
					}
					if pod.Status.Phase == corev1.PodRunning && pod.Status.ContainerStatuses[0].State.Running != nil {
						availablePods++
					}
				}
				if availablePods != maestroServerReplicas {
					return fmt.Errorf("unexpected available maestro server pod count, expected %d, got %d", maestroServerReplicas, availablePods)
				}

				return nil
			}, 5*time.Minute, 30*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the resource status is resynced", func() {
			// the watcher should resynced
			Eventually(func() error {
				if len(watchedResult.WatchedWorks) != 0 {
					return nil
				}
				return fmt.Errorf("no works watched")
			}, 5*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				work, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
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
			},
				10*time.Minute, // timeout is double work resync interval time (4~6 mins)
				2*time.Second).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			By("Startup the maestro server if its shutdown", func() {
				deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Get(ctx, "maestro", metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				if *deploy.Spec.Replicas == 0 {
					deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Patch(ctx, "maestro", types.MergePatchType, fmt.Appendf(nil, `{"spec":{"replicas":%d}}`, maestroServerReplicas), metav1.PatchOptions{
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

						// (TODO), the maestro pod may be in completed status so we skip the Completed status check here
						availablePods := 0
						for _, pod := range pods.Items {
							if pod.Status.Phase == corev1.PodSucceeded {
								continue
							}
							if pod.Status.Phase == corev1.PodRunning && pod.Status.ContainerStatuses[0].State.Running != nil {
								availablePods++
							}
						}
						if availablePods != maestroServerReplicas {
							return fmt.Errorf("unexpected available maestro server pod count, expected %d, got %d", maestroServerReplicas, availablePods)
						}

						return nil
					}, 1*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
				}
			})

			By("delete the resource with source work client")
			// note: wait some time to ensure source work client is connected to the restarted maestro server
			Eventually(func() error {
				return watcherClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
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

			By("check the resource deletion via source workclient")
			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			watcherCancel()
		})
	})
})
