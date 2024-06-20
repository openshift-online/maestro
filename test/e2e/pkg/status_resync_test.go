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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Status resync", Ordered, Label("e2e-tests-status-resync"), func() {

	var resource *openapi.Resource
	var maestroServerReplicas int

	Context("Resource resync resource status after maestro server restarts", func() {

		It("post the nginx resource with non-default service account to the maestro api", func() {

			res := helper.NewAPIResourceWithSA(consumer_name, 1, "nginx")
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

			Eventually(func() error {
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), *resource.Id).Execute()
				if err != nil {
					return err
				}
				statusJSON, err := json.Marshal(gotResource.Status)
				if err != nil {
					return err
				}
				if !strings.Contains(string(statusJSON), "error looking up service account default/nginx") {
					return fmt.Errorf("unexpected status, expected error looking up service account default/nginx, got %s", string(statusJSON))
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro server", func() {

			deploy, err := kubeClient.AppsV1().Deployments("maestro").Get(context.Background(), "maestro", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroServerReplicas = int(*deploy.Spec.Replicas)

			// patch maestro server replicas to 0
			deploy, err = kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
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

		It("create default/nginx serviceaccount", func() {

			_, err := kubeClient.CoreV1().ServiceAccounts("default").Create(context.Background(), &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// delete the nginx deployment to tigger recreating
			err = kubeClient.AppsV1().Deployments("default").Delete(context.Background(), "nginx", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("start maestro server", func() {

			// patch maestro server replicas to 1
			deploy, err := kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroServerReplicas)), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroServerReplicas)))

			// ensure maestro server pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro").List(context.Background(), metav1.ListOptions{
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
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), *resource.Id).Execute()
				if err != nil {
					return err
				}
				if _, ok := gotResource.Status["ContentStatus"]; !ok {
					return fmt.Errorf("unexpected status, expected contains ContentStatus, got %v", gotResource.Status)
				}
				statusJSON, err := json.Marshal(gotResource.Status)
				if err != nil {
					return err
				}
				if strings.Contains(string(statusJSON), "error looking up service account default/nginx") {
					return fmt.Errorf("unexpected status, should not contain error looking up service account default/nginx, got %s", string(statusJSON))
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

			err = kubeClient.CoreV1().ServiceAccounts("default").Delete(context.Background(), "nginx", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

	})
})
