package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Spec resync", Ordered, Label("e2e-tests-spec-resync"), func() {

	var resource1, resource2, resource3 *openapi.Resource
	var mqttReplicas, maestroAgentReplicas int

	Context("Resource resync resource spec after maestro agent restarts", func() {

		It("post the nginx-1 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 1)
			var resp *http.Response
			var err error
			resource1, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource1.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-2 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 2)
			var resp *http.Response
			var err error
			resource2, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource2.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-2 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro agent", func() {

			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Get(context.Background(), "maestro-agent", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroAgentReplicas = int(*deploy.Spec.Replicas)

			// patch maestro agent replicas to 0
			deploy, err = kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
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

		It("patch the nginx-1 resource", func() {

			newRes := helper.NewAPIResourceWithIndex(consumer_name, 2, 1)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(context.Background(), *resource1.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource1.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource1.Version + 1))
		})

		It("ensure the nginx-1 resource is not updated", func() {

			// ensure the "nginx-1" deployment in the "default" namespace is not updated
			Consistently(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx-2 resource", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource2.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx-2 resource is not deleted", func() {

			// ensure the "nginx-2" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx-2 deployment is deleted")
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-3 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 3)
			var resp *http.Response
			var err error
			resource3, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource3.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx-3 resource is not created", func() {

			// ensure the "nginx-3" deployment in the "default" namespace is not created
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx-3 deployment is created")
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("start maestro agent", func() {

			// patch maestro agent replicas to maestroAgentReplicas
			deploy, err := kubeClient.AppsV1().Deployments("maestro-agent").Patch(context.Background(), "maestro-agent", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroAgentReplicas)), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroAgentReplicas)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro-agent").List(context.Background(), metav1.ListOptions{
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-1 resource is updated", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-2 resource is deleted", func() {

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-2 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-3 resource is created", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx-1 and nginx-3 resource", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource1.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource3.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-1 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-3 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

	})

	Context("Resource resync resource spec after maestro agent reconnects", func() {

		It("post the nginx-1 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 1)
			var resp *http.Response
			var err error
			resource1, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource1.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-2 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 2)
			var resp *http.Response
			var err error
			resource2, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource2.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-2 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the mqtt-broker service for agent", func() {

			err := kubeClient.CoreV1().Services("maestro").Delete(context.Background(), "maestro-mqtt-agent", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Rollout the mqtt-broker", func() {

			deploy, err := kubeClient.AppsV1().Deployments("maestro").Get(context.Background(), "maestro-mqtt", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			mqttReplicas = int(*deploy.Spec.Replicas)
			deploy, err = kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro-mqtt", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running mqtt-broker pods
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro").List(context.Background(), metav1.ListOptions{
					LabelSelector: "name=maestro-mqtt",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) > 0 {
					return fmt.Errorf("maestro-mqtt pods still running")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			// patch mqtt-broker replicas to mqttReplicas
			deploy, err = kubeClient.AppsV1().Deployments("maestro").Patch(context.Background(), "maestro-mqtt", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, mqttReplicas)), metav1.PatchOptions{
				FieldManager: "testKubeClient",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(mqttReplicas)))

			// ensure mqtt-broker pod is up and running
			Eventually(func() error {
				pods, err := kubeClient.CoreV1().Pods("maestro").List(context.Background(), metav1.ListOptions{
					LabelSelector: "name=maestro-mqtt",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != mqttReplicas {
					return fmt.Errorf("unexpected maestro-mqtt pod count, expected %d, got %d", mqttReplicas, len(pods.Items))
				}
				for _, pod := range pods.Items {
					if pod.Status.Phase != "Running" {
						return fmt.Errorf("maestro-mqtt pod not in running state")
					}
					if pod.Status.ContainerStatuses[0].State.Running == nil {
						return fmt.Errorf("maestro-mqtt container not in running state")
					}
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("patch the nginx-1 resource", func() {

			newRes := helper.NewAPIResourceWithIndex(consumer_name, 2, 1)
			Eventually(func() error {
				patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(context.Background(), *resource1.Id).
					ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource1.Version, Manifest: newRes.Manifest}).Execute()
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status code, expected 200, got %d", resp.StatusCode)
				}
				if *patchedResource.Version != *resource1.Version+1 {
					return fmt.Errorf("unexpected version, expected %d, got %d", *resource1.Version+1, *patchedResource.Version)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-1 resource is not updated", func() {

			// ensure the "nginx-1" deployment in the "default" namespace is not updated
			Consistently(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx-2 resource", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource2.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx-2 resource is not deleted", func() {

			// ensure the "nginx-2" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx-2 deployment is deleted")
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-3 resource to the maestro api", func() {

			res := helper.NewAPIResourceWithIndex(consumer_name, 1, 3)
			var resp *http.Response
			var err error
			resource3, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource3.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx-3 resource is not created", func() {

			// ensure the "nginx-3" deployment in the "default" namespace is not created
			Consistently(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx-3 deployment is created")
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("recreate the mqtt-broker service for agent", func() {

			mqttAgentService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maestro-mqtt-agent",
					Namespace: "maestro",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"name": "maestro-mqtt",
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "mosquitto",
							Protocol:   corev1.ProtocolTCP,
							Port:       1883,
							TargetPort: intstr.FromInt(1883),
						},
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}

			_, err := kubeClient.CoreV1().Services("maestro").Create(context.Background(), mqttAgentService, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-1 resource is updated", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas for nginx-1 deployment, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-2 resource is deleted", func() {

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-2 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-3 resource is created", func() {

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx-1 and nginx-3 resource", func() {

			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource1.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(context.Background(), *resource3.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-1", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-1 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx-3", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-3 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

	})
})
