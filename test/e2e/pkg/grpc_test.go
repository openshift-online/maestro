package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

var _ = Describe("GRPC", Ordered, Label("e2e-tests-grpc"), func() {
	Context("GRPC Manifest Tests", func() {
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resourceID := uuid.NewString()
		resourceStatus := &api.ResourceStatus{
			ReconcileStatus: &api.ReconcileStatus{},
		}

		It("subscribe to resource status with grpc client", func() {
			go func() {
				subClient, err := grpcClient.Subscribe(ctx, &pbv1.SubscriptionRequest{Source: sourceID})
				if err != nil {
					return
				}

				for {
					pvEvt, err := subClient.Recv()
					if err == io.EOF {
						return
					}
					if err != nil {
						return
					}
					evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pvEvt))
					if err != nil {
						continue
					}

					evtExtensions := evt.Context.GetExtensions()
					resID, err := cetypes.ToString(evtExtensions[types.ExtensionResourceID])
					if err != nil {
						continue
					}

					if resID != resourceID {
						continue
					}

					resourceVersion, err := cetypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
					if err != nil {
						continue
					}
					resourceStatus.ReconcileStatus.ObservedVersion = resourceVersion

					manifestBundleStatus := &payload.ManifestBundleStatus{}
					if err := evt.DataAs(manifestBundleStatus); err != nil {
						continue
					}

					if len(manifestBundleStatus.ResourceStatus) != 1 {
						return
					}
					resourceStatus.ReconcileStatus.Conditions = manifestBundleStatus.ResourceStatus[0].Conditions
					if meta.IsStatusConditionTrue(manifestBundleStatus.Conditions, common.ManifestsDeleted) {
						deletedCondition := meta.FindStatusCondition(manifestBundleStatus.Conditions, common.ManifestsDeleted)
						resourceStatus.ReconcileStatus.Conditions = append(resourceStatus.ReconcileStatus.Conditions, *deletedCondition)
					}
					for _, value := range manifestBundleStatus.ResourceStatus[0].StatusFeedbacks.Values {
						if value.Name == "status" {
							contentStatus := make(map[string]interface{})
							if err := json.Unmarshal([]byte(*value.Value.JsonRaw), &contentStatus); err != nil {
								continue
							}
							resourceStatus.ContentStatus = contentStatus
						}
					}
				}
			}()
		})

		It("publish a resource spec using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "create_request", agentTestOpts.consumerName, resourceID, deployName, 1, 1)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource status using grpc client", func() {
			Eventually(func() error {
				if resourceStatus.ReconcileStatus == nil {
					return fmt.Errorf("reconcile status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Applied") {
					return fmt.Errorf("resource not applied")
				}

				if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Available") {
					return fmt.Errorf("resource not Available")
				}

				replicas, ok := resourceStatus.ContentStatus["replicas"]
				if !ok {
					return fmt.Errorf("replicas not found in content status")
				}

				if replicas.(float64) != float64(1) {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", replicas)
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource with the maestro api", func() {
			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(resourceID))
			Expect(*gotResource.Version).To(Equal(int32(1)))
		})

		It("publish a resource spec with update request using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "update_request", agentTestOpts.consumerName, resourceID, deployName, 1, 2)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource status using grpc client", func() {
			Eventually(func() error {
				if resourceStatus.ReconcileStatus == nil {
					return fmt.Errorf("reconcile status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Applied") {
					return fmt.Errorf("resource not applied")
				}

				if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Available") {
					return fmt.Errorf("resource not Available")
				}

				replicas, ok := resourceStatus.ContentStatus["replicas"]
				if !ok {
					return fmt.Errorf("replicas not found in content status")
				}

				if replicas.(float64) != float64(2) {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", replicas)
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource with the maestro api", func() {
			gotResource, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(resourceID))
			Expect(*gotResource.Version).To(Equal(int32(2)))
		})

		It("publish a resource spec with delete request using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "delete_request", agentTestOpts.consumerName, resourceID, deployName, 2, 2)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource status using grpc client", func() {
			Eventually(func() error {
				if resourceStatus.ReconcileStatus == nil {
					return fmt.Errorf("reconcile status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, common.ManifestsDeleted) {
					return fmt.Errorf("resource not deleted")
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource with the maestro api", func() {
			_, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, resourceID).Execute()
			Expect(err).To(HaveOccurred(), "Expected 404 error")
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Context("GRPC Manifest Bundle Tests", func() {
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resourceID := uuid.NewString()
		resourceBundleStatus := &api.ResourceBundleStatus{
			ManifestBundleStatus: &payload.ManifestBundleStatus{},
		}

		It("subscribe to resource bundle status with grpc client", func() {
			go func() {
				subClient, err := grpcClient.Subscribe(ctx, &pbv1.SubscriptionRequest{Source: sourceID})
				if err != nil {
					return
				}

				for {
					pvEvt, err := subClient.Recv()
					if err == io.EOF {
						return
					}
					if err != nil {
						return
					}
					evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pvEvt))
					if err != nil {
						continue
					}

					evtExtensions := evt.Context.GetExtensions()
					resID, err := cetypes.ToString(evtExtensions[types.ExtensionResourceID])
					if err != nil {
						continue
					}

					if resID != resourceID {
						continue
					}

					resourceVersion, err := cetypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
					if err != nil {
						continue
					}
					resourceBundleStatus.ObservedVersion = resourceVersion

					if err := evt.DataAs(resourceBundleStatus.ManifestBundleStatus); err != nil {
						continue
					}
				}
			}()
		})

		It("publish a resource bundle spec using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "create_request", agentTestOpts.consumerName, resourceID, deployName, 1, 1)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource bundle status using grpc client", func() {
			Eventually(func() error {
				if resourceBundleStatus.ManifestBundleStatus == nil {
					return fmt.Errorf("resource bundle status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, "Applied") {
					return fmt.Errorf("resource bundle not applied")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, "Available") {
					return fmt.Errorf("resource bundle not Available")
				}

				if len(resourceBundleStatus.ManifestBundleStatus.ResourceStatus) != 1 {
					return fmt.Errorf("unexpected number of resource status, expected 1, got %d", len(resourceBundleStatus.ManifestBundleStatus.ResourceStatus))
				}

				resourceStatus := resourceBundleStatus.ManifestBundleStatus.ResourceStatus[0]
				if len(resourceStatus.StatusFeedbacks.Values) != 1 {
					return fmt.Errorf("unexpected number of status feedbacks, expected 1, got %d", len(resourceStatus.StatusFeedbacks.Values))
				}

				value := resourceStatus.StatusFeedbacks.Values[0]
				contentStatus := make(map[string]interface{})
				if err := json.Unmarshal([]byte(*value.Value.JsonRaw), &contentStatus); err != nil {
					return fmt.Errorf("failed to convert status feedback value to content status: %v", err)
				}

				replicas, ok := contentStatus["replicas"]
				if !ok {
					return fmt.Errorf("replicas not found in content status")
				}

				if replicas.(float64) != float64(1) {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", replicas)
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource bundle with the maestro api", func() {
			gotResourceBundle, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResourceBundle.Id).To(Equal(resourceID))
			Expect(*gotResourceBundle.Version).To(Equal(int32(1)))
		})

		It("publish a resource bundle spec with update request using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "update_request", agentTestOpts.consumerName, resourceID, deployName, 1, 2)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource bundle status using grpc client", func() {
			Eventually(func() error {
				if resourceBundleStatus.ManifestBundleStatus == nil {
					return fmt.Errorf("resource bundle status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, "Applied") {
					return fmt.Errorf("resource bundle not applied")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, "Available") {
					return fmt.Errorf("resource bundle not Available")
				}

				if len(resourceBundleStatus.ManifestBundleStatus.ResourceStatus) != 1 {
					return fmt.Errorf("unexpected number of resource status, expected 1, got %d", len(resourceBundleStatus.ManifestBundleStatus.ResourceStatus))
				}

				resourceStatus := resourceBundleStatus.ManifestBundleStatus.ResourceStatus[0]
				if len(resourceStatus.StatusFeedbacks.Values) != 1 {
					return fmt.Errorf("unexpected number of status feedbacks, expected 1, got %d", len(resourceStatus.StatusFeedbacks.Values))
				}

				value := resourceStatus.StatusFeedbacks.Values[0]
				contentStatus := make(map[string]interface{})
				if err := json.Unmarshal([]byte(*value.Value.JsonRaw), &contentStatus); err != nil {
					return fmt.Errorf("failed to convert status feedback value to content status: %v", err)
				}

				replicas, ok := contentStatus["replicas"]
				if !ok {
					return fmt.Errorf("replicas not found in content status")
				}

				if replicas.(float64) != float64(2) {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", replicas)
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource bundle with the maestro api", func() {
			gotResourceBundle, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResourceBundle.Id).To(Equal(resourceID))
			Expect(*gotResourceBundle.Version).To(Equal(int32(2)))
		})

		It("publish a resource bundle spec with delete request using grpc client", func() {
			evt := helper.NewBundleEvent(sourceID, "delete_request", agentTestOpts.consumerName, resourceID, deployName, 2, 2)
			pbEvt := &pbv1.CloudEvent{}
			err := grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Subscribe to the resource bundle status using grpc client", func() {
			Eventually(func() error {
				if resourceBundleStatus.ManifestBundleStatus == nil {
					return fmt.Errorf("resource bundle status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, common.ManifestsDeleted) {
					return fmt.Errorf("resource bundle not applied")
				}

				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the nginx deployment from cluster", func() {
			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the resource with the maestro api", func() {
			_, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).To(HaveOccurred(), "Expected 404 error")
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
