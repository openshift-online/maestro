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
)

var _ = Describe("Resources", Ordered, Label("e2e-tests-resources"), func() {

	var resource *openapi.Resource

	Context("Resource CRUD Tests", func() {

		It("post the nginx resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer_name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
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

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas == 2 {
					return nil
				}
				return fmt.Errorf("replicas is not 2")
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

	Context("Resource Delete Option Tests", func() {
		res := helper.NewAPIResource(consumer_name, 1)
		It("post the nginx resource to the maestro api", func() {
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("patch the nginx resource with orphan delete option", func() {

			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(context.Background(), *resource.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource.Version, Manifest: res.Manifest, DeleteOption: map[string]interface{}{"propagationPolicy": "Orphan"}}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource.Version + 1))

		})

		It("delete the nginx resource from the maestro api", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			retry := 0
			Eventually(func() error {
				// Attempt to retrieve the "nginx" deployment in the "default" namespace
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})

				// If an error occurs
				if err != nil {
					// Return any other errors directly
					return err
				}

				// Increment the retry counter
				retry++

				// If the retry count reaches 30, consider it successful
				if retry == 30 {
					return nil
				}

				// Otherwise, indicate that another retry is needed
				return fmt.Errorf("need to retry again")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx deployment", func() {
			err := kubeClient.AppsV1().Deployments("default").Delete(context.Background(), "nginx", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

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

	Context("Resource Update Strategy Tests", func() {

		It("post the nginx resource to the maestro api with createOnly updateStrategy", func() {
			res := helper.NewAPIResource(consumer_name, 1)
			var resp *http.Response
			var err error
			res.UpdateStrategy = map[string]interface{}{"type": "CreateOnly"}
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
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

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas == 1 {
					return nil
				}
				return fmt.Errorf("replicas is not 1")
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

})
