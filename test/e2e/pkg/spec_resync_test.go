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
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Spec Resync After Restart", Ordered, Label("e2e-tests-spec-resync-restart"), func() {
	Context("Resource resync resource spec after maestro agent restarts", func() {
		var maestroAgentReplicas int
		var resourceA, resourceB, resourceC *openapi.Resource
		deployA := fmt.Sprintf("nginx-%s", rand.String(5))
		deployB := fmt.Sprintf("nginx-%s", rand.String(5))
		deployC := fmt.Sprintf("nginx-%s", rand.String(5))
		It("post the nginx A resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployA, 1)
			var resp *http.Response
			var err error
			resourceA, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceA.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 1, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx B resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployB, 1)
			var resp *http.Response
			var err error
			resourceB, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceB.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx B deployment %s, expected 1, got %d", deployB, *deploy.Spec.Replicas)
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

		It("patch the nginx A resource", func() {
			newRes := helper.NewAPIResource(consumer.Name, deployA, 2)
			patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resourceA.Id).
				ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resourceA.Version, Manifest: newRes.Manifest}).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*patchedResource.Version).To(Equal(*resourceA.Version + 1))
		})

		It("ensure the nginx A resource is not updated", func() {
			Consistently(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 1, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx B resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceB.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx B resource is not deleted", func() {
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx B deployment %s is deleted", deployB)
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx C resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployC, 1)
			var resp *http.Response
			var err error
			resourceC, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceC.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx C resource is not created", func() {
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx C deployment %s is created", deployC)
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

		It("ensure the nginx A resource is updated", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 2, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx B resource is deleted", func() {
			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx B deployment %s still exists", deployB)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx C resource is created", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx C deployment %s, expected 1, got %d", deployC, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx A and nginx C resources", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceA.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx A deployment %s still exists", deployA)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceC.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx C deployment %s still exists", deployC)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})

var _ = Describe("Spec Resync After Reconnect", Ordered, Label("e2e-tests-spec-resync-reconnect"), func() {
	Context("Resource resync resource spec after maestro agent reconnects", func() {
		var maestroServerReplicas, mqttReplicas int
		var resourceA, resourceB, resourceC *openapi.Resource
		deployA := fmt.Sprintf("nginx-%s", rand.String(5))
		deployB := fmt.Sprintf("nginx-%s", rand.String(5))
		deployC := fmt.Sprintf("nginx-%s", rand.String(5))
		It("post the nginx A resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployA, 1)
			var resp *http.Response
			var err error
			resourceA, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceA.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 1, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx B resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployB, 1)
			var resp *http.Response
			var err error
			resourceB, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceB.Id).ShouldNot(BeEmpty())

			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx B deployment %s, expected 1, got %d", deployB, *deploy.Spec.Replicas)
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

		It("patch the nginx A resource", func() {
			newRes := helper.NewAPIResource(consumer.Name, deployA, 2)
			Eventually(func() error {
				patchedResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, *resourceA.Id).
					ResourcePatchRequest(openapi.ResourcePatchRequest{Version: resourceA.Version, Manifest: newRes.Manifest}).Execute()
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status code, expected 200, got %d", resp.StatusCode)
				}
				if *patchedResource.Version != *resourceA.Version+1 {
					return fmt.Errorf("unexpected version, expected %d, got %d", *resourceA.Version+1, *patchedResource.Version)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx A resource is not updated", func() {
			Consistently(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return nil
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 1, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx B resource", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceB.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("ensure the nginx B resource is not deleted", func() {
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return fmt.Errorf("nginx B deployment %s is deleted", deployB)
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("post the nginx C resource to the maestro api", func() {
			res := helper.NewAPIResource(consumer.Name, deployC, 1)
			var resp *http.Response
			var err error
			resourceC, resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*resourceC.Id).ShouldNot(BeEmpty())
		})

		It("ensure the nginx C resource is not created", func() {
			Consistently(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("nginx C deployment %s is created", deployC)
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

		It("ensure the nginx A resource is updated", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas for nginx A deployment %s, expected 2, got %d", deployA, *deploy.Spec.Replicas)
				}
				return nil
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx B resource is deleted", func() {
			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployB, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx B deployment %s still exists", deployB)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("ensure the nginx C resource is created", func() {
			Eventually(func() error {
				deploy, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas for nginx C deployment %s, expected 1, got %d", deployC, *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the nginx A and nginx C resources", func() {
			resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceA.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployA, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx A deployment %s still exists", deployA)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resourceC.Id).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			Eventually(func() error {
				_, err := consumer.ClientSet.AppsV1().Deployments("default").Get(ctx, deployC, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx C deployment %s still exists", deployC)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})
