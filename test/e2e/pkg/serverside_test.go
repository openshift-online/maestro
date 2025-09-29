package e2e_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	workv1 "open-cluster-management.io/api/work/v1"
)

var _ = Describe("ServerSideApply", Ordered, Label("e2e-tests-ssa"), func() {
	// Context("Resource ServerSideApply Tests", func() {
	// 	// The kube-apiserver will set a default selector and label on the Pod of Job if the job does not have
	// 	// spec.Selector, these fields are immutable, if we use update strategy to apply Job, it will report
	// 	// AppliedManifestFailed. The maestro uses the server side strategy to apply a resource with ManifestWork
	// 	// by default, this will avoid this.
	// 	workName := "work-ssa-" + rand.String(5)
	// 	sleepJobName := fmt.Sprintf("sleep-%s", rand.String(5))
	// 	manifest := fmt.Sprintf("{\"apiVersion\":\"batch/v1\",\"kind\":\"Job\",\"metadata\":{\"name\":\"%s\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"sleep\",\"image\":\"busybox:1.36\",\"command\":[\"/bin/sh\",\"-c\",\"sleep 10\"]}],\"restartPolicy\":\"Never\"}},\"backoffLimit\":4}}", sleepJobName)

	// 	BeforeAll(func() {
	// 		By("create the resource with source work client")
	// 		work := &workv1.ManifestWork{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Name: workName,
	// 			},
	// 			Spec: workv1.ManifestWorkSpec{
	// 				Workload: workv1.ManifestsTemplate{
	// 					Manifests: []workv1.Manifest{
	// 						{
	// 							RawExtension: runtime.RawExtension{
	// 								Raw: []byte(manifest),
	// 							},
	// 						},
	// 					},
	// 				},
	// 				ManifestConfigs: []workv1.ManifestConfigOption{
	// 					{
	// 						ResourceIdentifier: workv1.ResourceIdentifier{
	// 							Group:     "batch",
	// 							Resource:  "jobs",
	// 							Name:      sleepJobName,
	// 							Namespace: "default",
	// 						},
	// 						FeedbackRules: []workv1.FeedbackRule{
	// 							{
	// 								Type: workv1.JSONPathsType,
	// 								JsonPaths: []workv1.JsonPath{
	// 									{
	// 										Name: "status",
	// 										Path: ".status",
	// 									},
	// 								},
	// 							},
	// 						},
	// 						UpdateStrategy: &workv1.UpdateStrategy{
	// 							Type: workv1.UpdateStrategyTypeServerSideApply,
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}

	// 		_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
	// 		Expect(err).ShouldNot(HaveOccurred())
	// 	})

	// 	AfterAll(func() {
	// 		By("delete the resource with source work client")
	// 		err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
	// 		Expect(err).ShouldNot(HaveOccurred())

	// 		By("check the resource deletion via maestro api")
	// 		Eventually(func() error {
	// 			search := fmt.Sprintf("consumer_name = '%s'", agentTestOpts.consumerName)
	// 			gotResourceList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	// 			if err != nil {
	// 				return err
	// 			}
	// 			if resp.StatusCode != http.StatusOK {
	// 				return fmt.Errorf("unexpected http code, got %d, expected %d", resp.StatusCode, http.StatusOK)
	// 			}
	// 			if len(gotResourceList.Items) != 0 {
	// 				return fmt.Errorf("expected no resources returned by maestro api")
	// 			}
	// 			return nil
	// 		}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
	// 	})

	// 	It("get the resource via maestro api", func() {
	// 		search := fmt.Sprintf("consumer_name = '%s'", agentTestOpts.consumerName)
	// 		gotResourceList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	// 		Expect(err).ShouldNot(HaveOccurred())
	// 		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	// 		Expect(len(gotResourceList.Items)).To(Equal(1))
	// 		resource := gotResourceList.Items[0]
	// 		Expect(resource.Metadata["creationTimestamp"]).ShouldNot(BeEmpty())
	// 	})

	// 	It("get the resource status back", func() {
	// 		Eventually(func() error {
	// 			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
	// 			if err != nil {
	// 				return err
	// 			}
	// 			if work.CreationTimestamp.Time.IsZero() {
	// 				return fmt.Errorf("work creationTimestamp is empty")
	// 			}

	// 			conditions := work.Status.Conditions
	// 			if meta.IsStatusConditionFalse(conditions, workv1.WorkApplied) {
	// 				return fmt.Errorf("unexpected condition %v", conditions)
	// 			}

	// 			if meta.IsStatusConditionFalse(conditions, workv1.WorkAvailable) {
	// 				return fmt.Errorf("unexpected condition %v", conditions)
	// 			}

	// 			if meta.IsStatusConditionFalse(conditions, "StatusFeedbackSynced") {
	// 				return fmt.Errorf("unexpected condition %v", conditions)
	// 			}

	// 			return nil
	// 		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
	// 	})
	// })

	Context("Nested Work ServerSideApply Tests", func() {
		workName := fmt.Sprintf("ssa-work-%s", rand.String(5))
		nestedWorkName := fmt.Sprintf("nested-work-%s", rand.String(5))
		nestedWorkNamespace := "default"
		BeforeAll(func() {
			By("create a resource with nested work using SSA")
			work := newNestedManifestWork(workName, nestedWorkName, nestedWorkNamespace)
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			By("delete the resource with source work client")
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("check the resource deletion via maestro api")
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
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("check the nested work is created and not updated", func() {
			// make sure the nested work is created
			Eventually(func() error {
				_, err := agentTestOpts.workClientSet.WorkV1().ManifestWorks(nestedWorkNamespace).Get(ctx, nestedWorkName, metav1.GetOptions{})
				return err
			}, 30*time.Second, time.Second).ShouldNot(HaveOccurred())

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
			}, 1*time.Minute, 1*time.Second).Should(BeNil())
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
