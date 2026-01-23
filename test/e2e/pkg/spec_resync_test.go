package e2e_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

var _ = Describe("Spec Resync After Restart", Ordered, Label("e2e-tests-spec-resync-restart"), func() {
	Context("Resync resource spec after maestro agent restarts", func() {
		var maestroAgentReplicas int
		deployA := fmt.Sprintf("nginx-a-%s", rand.String(5))
		workNameA := fmt.Sprintf("work-a-%s", rand.String(5))
		workA := helper.NewManifestWork(workNameA, deployA, "default", 1)
		deployB := fmt.Sprintf("nginx-b-%s", rand.String(5))
		workNameB := fmt.Sprintf("work-b-%s", rand.String(5))
		workB := helper.NewManifestWork(workNameB, deployB, "default", 1)
		deployC := fmt.Sprintf("nginx-c-%s", rand.String(5))
		workNameC := fmt.Sprintf("work-c-%s", rand.String(5))
		workC := helper.NewManifestWork(workNameC, deployC, "default", 1)

		BeforeAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create resource A with source work client (op-id: %s)", opID))
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, workA, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create resource B with source work client (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, workB, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro agent", func() {
			deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments(agentTestOpts.agentNamespace).Get(ctx, "maestro-agent", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroAgentReplicas = int(*deploy.Spec.Replicas)

			// patch maestro agent replicas to 0
			deploy, err = agentTestOpts.kubeClientSet.AppsV1().Deployments(agentTestOpts.agentNamespace).Patch(ctx, "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testagentTestOpts.kubeClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro agent pods
			Eventually(func() error {
				pods, err := agentTestOpts.kubeClientSet.CoreV1().Pods(agentTestOpts.agentNamespace).List(ctx, metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) > 0 {
					return fmt.Errorf("maestro-agent pods still running")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("patch the resource A with source work client", func() {
			workA, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workNameA, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWorkA := workA.DeepCopy()
			newWorkA.Spec.Workload.Manifests = []workv1.Manifest{helper.NewManifest(deployA, "default", 2)}

			patchData, err := grpcsource.ToWorkPatch(workA, newWorkA)
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("patch resource A (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(opIDCtx, workNameA, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx A deployment is not updated", func() {
			Consistently(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 1, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the resource B with source work client", func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete resource B (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workNameB, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx B deployment is not deleted", func() {
			Consistently(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx B deployment %s is deleted", deployB)
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("create resource C with source work client", func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create resource C (op-id: %s)", opID))
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, workC, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("ensure the nginx C deployment is not created", func() {
			Consistently(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx C deployment %s is created", deployC)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("restart maestro agent", func() {
			// patch maestro agent replicas back
			deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments(agentTestOpts.agentNamespace).Patch(ctx, "maestro-agent", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroAgentReplicas)), metav1.PatchOptions{
				FieldManager: "testagentTestOpts.kubeClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroAgentReplicas)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := agentTestOpts.kubeClientSet.CoreV1().Pods(agentTestOpts.agentNamespace).List(ctx, metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != maestroAgentReplicas {
					return fmt.Errorf("unexpected maestro-agent pod count, expected %d, got %d", maestroAgentReplicas, len(pods.Items))
				}
				for _, pod := range pods.Items {
					if pod.Status.Phase != "Running" {
						return fmt.Errorf("maestro-agent pod not in running state")
					}
					if pod.Status.ContainerStatuses[0].State.Running == nil {
						return fmt.Errorf("maestro-agent container not in running state")
					}
				}
				return nil
			}, 5*time.Minute, 30*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx A deployment is updated", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 2, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx B deployment is deleted", func() {
			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx B deployment %s still exists", deployB)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx C deployment is created", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx C deployment %s, expected 1, got %d", deployC, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the nginx A, and nginx C resources and B if not deleted (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workNameA, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workNameB, metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				opIDCtx, opID = newOpIDContext(ctx)
				By(fmt.Sprintf("delete resource B (op-id: %s)", opID))
				err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workNameB, metav1.DeleteOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete resource C (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workNameC, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx A deployment %s still exists", deployA)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx B deployment %s still exists", deployB)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx C deployment %s still exists", deployC)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			By("check the resource deletion via source workclient")
			Eventually(func() error {
				if err := AssertWorkNotFound(workNameA); err != nil {
					return err
				}

				if err := AssertWorkNotFound(workNameB); err != nil {
					return err
				}

				if err := AssertWorkNotFound(workNameC); err != nil {
					return err
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})
