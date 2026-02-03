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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

var _ = Describe("Resources", Ordered, Label("e2e-tests-resources"), func() {
	Context("Resource CRUD Tests", func() {
		workName := fmt.Sprintf("work-%s", rand.String(5))
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		work := helper.NewManifestWork(workName, deployName, "default", 1)
		var resourceID string

		BeforeAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create the resource with source work client (op-id: %s)", opID))
			createdWork, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(createdWork.Name).To(Equal(workName))

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}).ShouldNot(HaveOccurred())

			resourceID = string(createdWork.UID)
			gotResource, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Version).To(Equal(int32(1)))
		})

		AfterAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the resource with source work client (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}).ShouldNot(HaveOccurred())
		})

		It("patch the resource with source work client", func() {
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{helper.NewManifest(deployName, "default", 2)}

			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("patch the resource with source work client (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			gotResource, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Version).To(Equal(int32(2)))
		})

		It("delete and create again", func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the resource with source work client (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
				if err == nil {
					return fmt.Errorf("expected resource to be deleted, but got %d", resp.StatusCode)
				}
				if resp.StatusCode != http.StatusNotFound {
					return fmt.Errorf("unexpected http code, got %d, expected %d", resp.StatusCode, http.StatusNotFound)
				}
				return nil
			}).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create the resource again with source work client (op-id: %s)", opID))
			createdWork, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(createdWork.Name).To(Equal(workName))

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}).ShouldNot(HaveOccurred())

			// the resource id should not change
			gotResource, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Version).To(Equal(int32(1)))
		})
	})

	Context("Resource ReadOnly  Tests", func() {
		workName := "work-readonly-" + rand.String(5)
		secretName := "auth-" + rand.String(5)
		manifest := fmt.Sprintf("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"%s\",\"namespace\":\"default\"}}", secretName)

		BeforeAll(func() {
			By("create the auth secret in the target cluster")
			_, err := agentTestOpts.kubeClientSet.CoreV1().Secrets("default").Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			work := &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name: workName,
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte(manifest),
								},
							},
						},
					},
					ManifestConfigs: []workv1.ManifestConfigOption{
						{
							ResourceIdentifier: workv1.ResourceIdentifier{
								Resource:  "secrets",
								Name:      secretName,
								Namespace: "default",
							},
							FeedbackRules: []workv1.FeedbackRule{
								{
									Type: workv1.JSONPathsType,
									JsonPaths: []workv1.JsonPath{
										{
											Name: "credential",
											Path: ".data",
										},
									},
								},
							},
							UpdateStrategy: &workv1.UpdateStrategy{
								Type: workv1.UpdateStrategyTypeReadOnly,
							},
						},
					},
				},
			}

			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create the readonly resource with source work client (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the readonly resource with source work client (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = agentTestOpts.kubeClientSet.CoreV1().Secrets("default").Delete(ctx, secretName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.CoreV1().Secrets("default").Get(ctx, secretName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("auth secret still exists")
			}).ShouldNot(HaveOccurred())

			By("check the resource deletion via source workclient")
			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}).ShouldNot(HaveOccurred())
		})

		It("get the resource status back", func() {
			Eventually(func() error {
				work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if work.CreationTimestamp.Time.IsZero() {
					return fmt.Errorf("work creation timestamp is empty")
				}

				manifests := work.Status.ResourceStatus.Manifests
				if len(manifests) > 0 && len(manifests[0].StatusFeedbacks.Values) != 0 {
					feedback := manifests[0].StatusFeedbacks.Values
					if feedback[0].Name == "credential" && *feedback[0].Value.JsonRaw == "{\"token\":\"dG9rZW4=\"}" {
						return nil
					}
					return fmt.Errorf("the status feedback value %v is not expected", feedback[0])
				}

				return fmt.Errorf("work status manifests are empty")
			}).ShouldNot(HaveOccurred())
		})
	})

	Context("Resource ServerSideApply Tests", func() {
		workName := fmt.Sprintf("ssa-work-%s", rand.String(5))
		nestedWorkName := fmt.Sprintf("nested-work-%s", rand.String(5))
		nestedWorkNamespace := "default"
		BeforeAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create a resource with nested work using SSA with source work client (op-id: %s)", opID))
			work := newNestedManifestWork(workName, nestedWorkName, nestedWorkNamespace)
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the resource with source work client (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("check the resource deletion via source workclient")
			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}).ShouldNot(HaveOccurred())
		})

		It("check the nested work is created and not updated", func() {
			// make sure the nested work is created
			Eventually(func() error {
				_, err := agentTestOpts.workClientSet.WorkV1().ManifestWorks(nestedWorkNamespace).Get(ctx, nestedWorkName, metav1.GetOptions{})
				return err
			}).ShouldNot(HaveOccurred())

			// make sure the nested work is not updated
			Consistently(func() error {
				nestedWork, err := agentTestOpts.workClientSet.WorkV1().ManifestWorks(nestedWorkNamespace).Get(ctx, nestedWorkName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if nestedWork.Generation != 1 {
					return fmt.Errorf("nested work generation is changed to %d", nestedWork.Generation)
				}

				return nil
			}).Should(BeNil())
		})
	})

	Context("Update an obsolete resource", func() {
		var workName string

		BeforeEach(func() {
			workName = "work-" + rand.String(5)
			work := NewManifestWork(workName)
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create the resource with source work client (op-id: %s)", opID))
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)
		})

		AfterEach(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete the resource with source work client (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}).ShouldNot(HaveOccurred())

		})

		It("should return error when updating an obsolete work", func() {
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("patch the resource with source work client (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			obsoleteWork := work.DeepCopy()
			obsoleteWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err = grpcsource.ToWorkPatch(work, obsoleteWork)
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("patch the resource again with source work client (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).Should(HaveOccurred())
			Expect(strings.Contains(err.Error(), "the resource version is not the latest")).Should(BeTrue())

			// wait for few seconds to ensure the update is finished
			<-time.After(5 * time.Second)
		})
	})
})

func newNestedManifestWork(workName, nestedWorkName, nestedWorkNamespace string) *workv1.ManifestWork {
	nestedWork := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "work.open-cluster-management.io/v1",
			Kind:       "ManifestWork",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nestedWorkName,
			Namespace: nestedWorkNamespace,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{{
					RawExtension: runtime.RawExtension{
						Object: &corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								Kind:       "ConfigMap",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cm-test",
								Namespace: "default",
							},
							Data: map[string]string{
								"some": "data",
							},
						},
					},
				}},
			},
		},
	}

	manifest := workv1.Manifest{}
	manifest.Object = nestedWork

	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: workName,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{manifest},
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "work.open-cluster-management.io",
						Resource:  "manifestworks",
						Name:      nestedWorkName,
						Namespace: nestedWorkNamespace,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeServerSideApply,
						ServerSideApply: &workv1.ServerSideApplyConfig{
							Force:        true,
							FieldManager: "maestro-agent",
						},
					},
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: "status",
									Path: ".status",
								},
							},
						},
					},
				},
			},
		},
	}
}
