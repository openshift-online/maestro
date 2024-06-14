package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Certificate rotation", Ordered, Label("e2e-tests-spec-resync"), func() {

	var resource *openapi.Resource
	var validSecretData map[string][]byte

	Context("Resource resync resource spec for agent reconnect with validate certificate", func() {

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

		It("replace the agent client certificates to mqtt broker", func() {

			validSecret, err := kubeClient.CoreV1().Secrets("maestro-agent").Get(context.Background(), "maestro-agent-certs", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			invalidSecret, err := kubeClient.CoreV1().Secrets("maestro-agent").Get(context.Background(), "maestro-agent-invalid-certs", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			validSecretData = validSecret.Data
			validSecret.Data = invalidSecret.Data

			// update the secret with invalid data
			_, err = kubeClient.CoreV1().Secrets("maestro-agent").Update(context.Background(), validSecret, metav1.UpdateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// ensure maestro-agent logs have "expired certificate"
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=maestro-agent",
				})
				if err != nil {
					return fmt.Errorf("error in listing maestro-agent pods")
				}
				if len(pods.Items) == 0 {
					return fmt.Errorf("maestro-agent pod not found")
				}

				for _, pod := range pods.Items {
					req := kubeClient.CoreV1().Pods("maestro-agent").GetLogs(pod.Name, &corev1.PodLogOptions{})
					podLogs, err := req.Stream(context.Background())
					if err != nil {
						return fmt.Errorf("error in opening pod logs stream")
					}
					buf := new(bytes.Buffer)
					_, err = io.Copy(buf, podLogs)
					if err != nil {
						podLogs.Close()
						return fmt.Errorf("error in copy information from podLogs to buf")
					}
					if strings.Contains(buf.String(), "expired certificate") {
						podLogs.Close()
						return nil
					}
					podLogs.Close()
				}

				return fmt.Errorf("maestro-agent logs does not have 'expired certificate'")
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
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

		It("recover the agent client certificates to mqtt broker", func() {

			certSecret, err := kubeClient.CoreV1().Secrets("maestro-agent").Get(context.Background(), "maestro-agent-certs", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			certSecret.Data = validSecretData

			// update the secret with invalid data
			_, err = kubeClient.CoreV1().Secrets("maestro-agent").Update(context.Background(), certSecret, metav1.UpdateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
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
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
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
