package e2e_test

import (
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

var _ = Describe("Spec Resync After Restart", Ordered, Label("e2e-tests-spec-resync-restart"), func() {
	var resource1, resource2, resource3 *openapi.Resource
	var maestroAgentReplicas int

	Context("Resource resync resource spec after maestro agent restarts", func() {
		It("post the nginx-1 resource to the maestro api", func() {
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 1)
			var resp *http.Response
			var err error
			resource1, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource1.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 2)
			var resp *http.Response
			var err error
			resource2, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource2.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
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
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Get(ctx, "maestro-agent", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroAgentReplicas = int(*deploy.Spec.Replicas)

			// patch maestro agent replicas to 0
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro-agent").Patch(ctx, "maestro-agent", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(0)))

			// ensure no running maestro agent pods
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro-agent").List(ctx, metav1.ListOptions{
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
			newRes := helper.NewAPIResourceWithIndex(consumer.Name, 2, 1)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resource1.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resource1.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resource1.Version + 1))
		})

		It("ensure the nginx-1 resource is not updated", func() {
			// ensure the "nginx-1" deployment in the "default" namespace is not updated
			Consistently(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource2.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx-2 resource is not deleted", func() {
			// ensure the "nginx-2" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx-2 deployment is deleted")
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-3 resource to the maestro api", func() {
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 3)
			var resp *http.Response
			var err error
			resource3, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource3.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx-3 resource is not created", func() {
			// ensure the "nginx-3" deployment in the "default" namespace is not created
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx-3 deployment is created")
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("restart maestro agent", func() {
			// patch maestro agent replicas back
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro-agent").Patch(ctx, "maestro-agent", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroAgentReplicas)), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(maestroAgentReplicas)))

			// ensure maestro agent pod is up and running
			Eventually(func() error {
				pods, err := consumer.ClientSet.CoreV1().Pods("maestro-agent").List(ctx, metav1.ListOptions{
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
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
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
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
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
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource1.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource3.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-1 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
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

var _ = Describe("Spec Resync After Reconnect", Ordered, Label("e2e-tests-spec-resync-reconnect"), func() {
	var resource1, resource2, resource3 *openapi.Resource
	var maestroServerReplicas, mqttReplicas int

	Context("Resource resync resource spec after maestro agent reconnects", func() {
		It("post the nginx-1 resource to the maestro api", func() {
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 1)
			var resp *http.Response
			var err error
			resource1, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource1.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 2)
			var resp *http.Response
			var err error
			resource2, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource2.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx-2 deployment, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the grpc-broker service for agent", func() {
			err := consumer.ClientSet.CoreV1().Services("maestro").Delete(ctx, "maestro-grpc-broker", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("delete the mqtt-broker service for agent", func() {
			err := consumer.ClientSet.CoreV1().Services("maestro").Delete(ctx, "maestro-mqtt-agent", metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("rollout maestro server", func() {
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Get(ctx, "maestro", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			maestroServerReplicas = int(*deploy.Spec.Replicas)
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
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

			// patch maestro server replicas to maestroServerReplicas
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro", types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, maestroServerReplicas)), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
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

		It("rollout the mqtt-broker", func() {
			deploy, err := consumer.ClientSet.AppsV1().Deployments("maestro").Get(ctx, "maestro-mqtt", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			mqttReplicas = int(*deploy.Spec.Replicas)
			deploy, err = consumer.ClientSet.AppsV1().Deployments("maestro").Patch(ctx, "maestro-mqtt", types.MergePatchType, []byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{
				FieldManager: "testconsumer.ClientSet",
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
				FieldManager: "testconsumer.ClientSet",
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

		It("patch the nginx-1 resource", func() {
			newRes := helper.NewAPIResourceWithIndex(consumer.Name, 2, 1)
			Eventually(func() error {
				patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resource1.Id).
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
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource2.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx-2 resource is not deleted", func() {
			// ensure the "nginx-2" deployment in the "default" namespace is not deleted
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx-2 deployment is deleted")
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx-3 resource to the maestro api", func() {
			res := helper.NewAPIResourceWithIndex(consumer.Name, 1, 3)
			var resp *http.Response
			var err error
			resource3, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resource3.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx-3 resource is not created", func() {
			// ensure the "nginx-3" deployment in the "default" namespace is not created
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
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

			_, err := consumer.ClientSet.CoreV1().Services("maestro").Create(ctx, mqttAgentService, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("recreate the grpc-broker service for agent", func() {
			grpcBrokerService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maestro-grpc-broker",
					Namespace: "maestro",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "maestro",
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "grpc-broker",
							Protocol:   corev1.ProtocolTCP,
							Port:       8091,
							TargetPort: intstr.FromInt(8091),
						},
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}
			_, err := consumer.ClientSet.CoreV1().Services("maestro").Create(ctx, grpcBrokerService, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx-1 resource is updated", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
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
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-2", metav1.GetOptions{})
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
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
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
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource1.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource3.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-1", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx-1 deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, "nginx-3", metav1.GetOptions{})
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
