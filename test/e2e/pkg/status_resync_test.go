package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Status resync", Ordered, Label("e2e-tests-status-resync"), func() {

	var resource *openapi.Resource

	Context("Resource resync resource status", func() {

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

			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(*resource.Id))
			Expect(*gotResource.Version).To(Equal(*resource.Version))

			statusJSON, err := json.Marshal(gotResource.Status)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(strings.Contains(string(statusJSON), "testKubeClient")).To(BeFalse())
		})

		It("shut down maestro server", func() {

			// patch marstro server replicas to 0
			deploy, err := kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro server pods
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro").List(context.Background(), metav1.ListOptions{
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

		It("patch the resource in the cluster", func() {

			deploy, err := kubeClient.AppsV1().Deployments("default").Patch(context.Background(), "nginx", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))
		})

		It("start maestro server", func() {

			// patch marstro server replicas to 1
			deploy, err := kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":1}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// ensure maestro server pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro").List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=maestro",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) == 0 {
					return fmt.Errorf("unable to find maestro server pod")
				}
				if pods.Items[0].Status.Phase != "Running" {
					return fmt.Errorf("maestro server pod not in running state")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the resource status is resynced", func() {
			Eventually(func() error {
				gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), *resource.Id).Execute()
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status code, expected 200, got %d", resp.StatusCode)
				}
				if *gotResource.Id != *resource.Id {
					return fmt.Errorf("unexpected resource id, expected %s, got %s", *resource.Id, *gotResource.Id)
				}
				if *gotResource.Version != *resource.Version {
					return fmt.Errorf("unexpected resource version, expected %d, got %d", *resource.Version, *gotResource.Version)
				}

				statusJSON, err := json.Marshal(gotResource.Status)
				if err != nil {
					return err
				}
				// TODO: add a better check if the status is resynced
				if !strings.Contains(string(statusJSON), "testKubeClient") {
					return fmt.Errorf("unexpected status, expected testKubeClient, got %s", string(statusJSON))
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

})
