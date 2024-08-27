package e2e_test

import (
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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Status Resync After Restart", Ordered, Label("e2e-tests-status-resync-restart"), func() {
	Context("Resource resync resource status after maestro server restarts", func() {
		var maestroServerReplicas int
		var resource *openapi.Resource
		name := fmt.Sprintf("nginx-%s", rand.String(5))
		It("post the nginx resource with non-default service account to the maestro api", func() {
			res := helper.NewAPIResourceWithSA(consumer.Name, name, name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx deployment %s, expected 1, got %d", name, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(*resource.Id))
			Expect(*gotResource.Version).To(Equal(*resource.Version))

			Eventually(func() error {
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
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
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("shut down maestro server", func() {
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Get(ctx, "maestro", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroServerReplicas = int(*deploy.Spec.Replicas)

			// patch maestro server replicas to 0
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro server pods
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
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

		It("create serviceaccount for nginx deployment", func() {
			_, err := consumer.ClientSet.CoreV1().ServiceAccounts("default").Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// delete the nginx deployment to tigger recreating
			err = consumer.ClientSet.AppsV1().Deployments("default").Delete(ctx, name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("restart maestro server", func() {
			// patch maestro server replicas back
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroServerReplicas)), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroServerReplicas)))

			// ensure maestro server pod is up and running
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
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
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
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
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

			err = consumer.ClientSet.CoreV1().ServiceAccounts("default").Delete(ctx, name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

var _ = Describe("Status Resync After Reconnect", Ordered, Label("e2e-tests-status-resync-reconnect"), func() {
	Context("Resource resync resource status after maestro server reconnects", func() {
		var mqttReplicas int
		var resource *openapi.Resource
		name := fmt.Sprintf("nginx-%s", rand.String(5))
		It("post the nginx resource with non-default service account to the maestro api", func() {
			res := helper.NewAPIResourceWithSA(consumer.Name, name, name, 1)
			var resp *http.Response
			var err error
			resource, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx deployment %s, expected 1, got %d", name, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(*resource.Id))
			Expect(*gotResource.Version).To(Equal(*resource.Version))

			Eventually(func() error {
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
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
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the mqtt-broker service for server", func() {
			err := consumer.ClientSet.CoreV1().Services("maestro").Delete(ctx, "maestro-mqtt-server", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("create serviceaccount for nginx deployment", func() {
			_, err := consumer.ClientSet.CoreV1().ServiceAccounts("default").Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// delete the nginx deployment to tigger recreating
			err = consumer.ClientSet.AppsV1().Deployments("default").Delete(ctx, name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Rollout the mqtt-broker", func() {
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Get(ctx, "maestro-mqtt", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			mqttReplicas = int(*deploy.Spec.Replicas)
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro-mqtt", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running mqtt-broker pods
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
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
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro-mqtt", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, mqttReplicas)), metav1.PatchOptions{
				FieldManager: "testConsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(mqttReplicas)))

			// ensure mqtt-broker pod is up and running
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro").List(ctx, metav1.ListOptions{
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

		It("recreate the mqtt-broker service for server", func() {
			mqttServerService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maestro-mqtt-server",
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

			_, err := consumer.ClientSet.CoreV1().Services("maestro").Create(ctx, mqttServerService, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("ensure the resource status is resynced", func() {
			Eventually(func() error {
				gotResource, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
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
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

			err = consumer.ClientSet.CoreV1().ServiceAccounts("default").Delete(ctx, name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
