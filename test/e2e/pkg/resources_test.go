package e2e_test

import (
	"fmt"
	"net/http"
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
		})

		It("get the resource via maestro api", func() {
			search := fmt.Sprintf("consumer_name = '%s'", agentTestOpts.consumerName)
			gotResourceList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(len(gotResourceList.Items)).To(Equal(1))
			resourceID = *gotResourceList.Items[0].Id
		})

		It("patch the resource with source work client", func() {
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{helper.NewManifest(deployName, "default", 2)}

			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Version).To(Equal(int32(2)))
		})

		It("delete the resource with source work client", func() {
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			_, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).Should(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
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

	Context("Resource ReadOnly Tests", func() {
		workName := "work-readonly-" + rand.String(5)
		secretName := "auth-" + rand.String(5)
		manifest := fmt.Sprintf("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"%s\",\"namespace\":\"default\"}}", secretName)
		It("create the secret in the target cluster", func() {
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
		})

		It("post the resource with source work client", func() {
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

			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("get the resource via maestro API", func() {
			search := fmt.Sprintf("consumer_name = '%s'", agentTestOpts.consumerName)
			gotResourceList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(len(gotResourceList.Items)).To(Equal(1))
			resource := gotResourceList.Items[0]
			Expect(resource.Metadata["creationTimestamp"]).ShouldNot(BeEmpty())
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the readonly resource with source work client", func() {
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
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
