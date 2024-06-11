package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Spec resync", Ordered, Label("e2e-tests-spec-resync"), func() {

	var resource *openapi.Resource

	Context("Resource resync created resource spec", func() {

		It("shut down maestro agent", func() {

			// patch marstro agent replicas to 0
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro agent pods
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
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

		It("post the nginx resource to the maestro api", func() {

			res := helper.NewAPIResource(consumer_name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())
		})

		It("ensure the resource is not created", func() {

			// ensure the "nginx" deployment in the "default" namespace is not created
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx deployment is created")
				}
				return nil
			}, 30*time.Second, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("start maestro agent", func() {

			// patch marstro agent replicas to 1
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":1}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != 1 {
					return fmt.Errorf("unexpected maestro-agent pod count, expected 1, got %d", len(pods.Items))
				}
				if pods.Items[0].Status.Phase != "Running" {
					return fmt.Errorf("maestro-agent pod not in running state")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the resource is created", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

	Context("Resource resync updated resource spec", func() {

		It("post the nginx resource to the maestro api", func() {

			res := helper.NewAPIResource(consumer_name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

			// patch marstro agent replicas to 0
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro agent pods
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
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

		It("patch the nginx resource", func() {

			newRes := helper.NewAPIResource(consumer_name, 2)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(context.Background(), *resource.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource.Version + 1))

		})

		It("ensure the resource is not updated", func() {

			// ensure the "nginx" deployment in the "default" namespace is not updated
			Consistently(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 30*time.Second, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("start maestro agent", func() {

			// patch marstro agent replicas to 1
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":1}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != 1 {
					return fmt.Errorf("unexpected maestro-agent pod count, expected 1, got %d", len(pods.Items))
				}
				if pods.Items[0].Status.Phase != "Running" {
					return fmt.Errorf("maestro-agent pod not in running state")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the resource is updated", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

	Context("Resource resync deleted resource spec", func() {

		It("post the nginx resource to the maestro api", func() {

			res := helper.NewAPIResource(consumer_name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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

			// patch marstro agent replicas to 0
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro agent pods
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
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

		It("delete the nginx resource", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the resource is not deleted", func() {

			// ensure the "nginx" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx deployment is deleted")
					}
				}
				return nil
			}, 30*time.Second, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("start maestro agent", func() {

			// patch marstro agent replicas to 1
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":1}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != 1 {
					return fmt.Errorf("unexpected maestro-agent pod count, expected 1, got %d", len(pods.Items))
				}
				if pods.Items[0].Status.Phase != "Running" {
					return fmt.Errorf("maestro-agent pod not in running state")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the resource is deleted", func() {

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
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
