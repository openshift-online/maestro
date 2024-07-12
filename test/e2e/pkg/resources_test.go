package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// go test -v ./test/e2e/pkg -args -api-server=$api_server -consumer-name=$consumer.Name -consumer-kubeconfig=$consumer_kubeconfig -ginkgo.focus "Resources"
var _ = Describe("Resources", Ordered, Label("e2e-tests-resources"), func() {
	var resource *openapi.Resource

	Context("Resource CRUD Tests", func() {
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

		It("get the nginx resource from the maestro api", func() {
			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(*resource.Id))
			Expect(*gotResource.Version).To(Equal(*resource.Version))
		})

		It("patch the nginx resource with the maestro api", func() {
			newRes := helper.NewAPIResource(consumer.Name, 2)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resource.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource.Version + 1))

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
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

	Context("Resource Delete Option Tests", func() {
		res := helper.NewAPIResource(consumer.Name, 1)
		It("post the nginx resource to the maestro api", func() {
			var resp *http.Response
			var err error
			res.DeleteOption = map[string]interface{}{"propagationPolicy": "Orphan"}
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

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

		It("delete the nginx resource from the maestro api", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			// ensure the "nginx" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx deployment is deleted")
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx deployment", func() {
			err := consumer.ClientSet.AppsV1().Deployments("default").Delete(ctx, "nginx", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

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

	Context("Resource CreateOnly UpdateStrategy Tests", func() {
		It("post the nginx resource to the maestro api with createOnly updateStrategy", func() {
			res := helper.NewAPIResource(consumer.Name, 1)
			var resp *http.Response
			var err error
			res.UpdateStrategy = map[string]interface{}{"type": "CreateOnly"}
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

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

		It("patch the nginx resource", func() {
			newRes := helper.NewAPIResource(consumer.Name, 2)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resource.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource.Version + 1))

			Consistently(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx", metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
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

	Context("Resource ReadOnly UpdateStrategy Tests via restful api", func() {
		It("create a sample deployment in the target cluster", func() {
			nginxDeploy := &appsv1.Deployment{}
			err := json.Unmarshal(helper.GetTestNginxJSON(1), nginxDeploy)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = consumer.ClientSet.AppsV1().Deployments("default").Create(ctx, nginxDeploy, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("post the resource to the maestro api with readonly updateStrategy", func() {
			res := helper.NewReadOnlyAPIResource(consumer.Name)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())
		})

		It("get the resource status back", func() {
			Eventually(func() error {
				res, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
				if err != nil {
					return err
				}

				statusJSON, err := json.Marshal(res.Status)
				if err != nil {
					return err
				}

				resourceStatus := &api.ResourceStatus{}
				err = json.Unmarshal(statusJSON, resourceStatus)
				if err != nil {
					return err
				}

				if resourceStatus.ContentStatus != nil {
					conditions := resourceStatus.ContentStatus["conditions"].([]interface{})
					if len(conditions) > 0 {
						return nil
					}
				}
				return fmt.Errorf("contentStatus should not be empty")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the readonly resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			err = consumer.ClientSet.AppsV1().Deployments("default").Delete(ctx, "nginx", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

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

	Context("Resource ReadOnly UpdateStrategy Tests via gRPC", func() {
		workName := "work-readonly-" + rand.String(5)
		secretName := "auth"
		It("create a sample secret in the target cluster", func() {
			_, err := consumer.ClientSet.CoreV1().Secrets("default").Create(ctx, &corev1.Secret{
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

		It("post the resource bundle via gRPC client", func() {
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "auth",
					},
				},
			}
			objectStr, _ := obj.MarshalJSON()
			manifest := workv1.Manifest{}
			manifest.Raw = objectStr
			_, err := workClient.ManifestWorks(consumer.Name).Create(ctx, &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name: workName,
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"auth\",\"namespace\":\"default\"}}"),
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
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("get the resource via restful API", func() {
			gotResourceBundleList, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(len(gotResourceBundleList.Items)).To(Equal(1))
			resourceBundle := gotResourceBundleList.Items[0]
			Expect(resourceBundle.Metadata["creationTimestamp"]).ShouldNot(BeEmpty())
			gotResourceBundle, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, *resourceBundle.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(gotResourceBundle.Metadata["creationTimestamp"]).ShouldNot(BeEmpty())
		})

		It("get the resource status back", func() {
			Eventually(func() error {
				work, err := workClient.ManifestWorks(consumer.Name).Get(ctx, workName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if work.CreationTimestamp.Time.IsZero() {
					return fmt.Errorf("work creationTimestamp is empty")
				}

				manifest := work.Status.ResourceStatus.Manifests
				if len(manifest) > 0 && len(manifest[0].StatusFeedbacks.Values) != 0 {
					feedback := manifest[0].StatusFeedbacks.Values
					if feedback[0].Name == "credential" && *feedback[0].Value.JsonRaw == "{\"token\":\"dG9rZW4=\"}" {
						return nil
					}
					return fmt.Errorf("the result %v is not expected", feedback[0])
				}

				return fmt.Errorf("manifest should be empty")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the readonly resource", func() {
			err := workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = consumer.ClientSet.CoreV1().Secrets("default").Delete(ctx, secretName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := consumer.ClientSet.CoreV1().Secrets("default").Get(ctx, secretName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("auth secret still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})
