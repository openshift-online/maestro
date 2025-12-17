package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

var _ = Describe("GRPC", Ordered, Label("e2e-tests-grpc"), func() {
	Context("GRPC PubSub Tests", func() {
		var subscriberCtx context.Context
		var subscriberCancel context.CancelFunc
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resourceID := uuid.NewString()
		subscribedResourceStatus := &SubscribedResourceStatus{}

		BeforeAll(func() {
			subscriberCtx, subscriberCancel = context.WithCancel(ctx)
			// start resource status subscribe
			go startStatusSubscribe(subscriberCtx, sourceID, grpcClient, subscribedResourceStatus)

			By("publish a resource create request with grpc client")
			evt, err := helper.NewEvent(sourceID, "create_request", agentTestOpts.consumerName, resourceID, deployName, 1, 1)
			Expect(err).ShouldNot(HaveOccurred())
			pbEvt := &pbv1.CloudEvent{}
			err = grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			By("publish a resource delete request with grpc client")
			evt, err := helper.NewEvent(sourceID, "delete_request", agentTestOpts.consumerName, resourceID, deployName, 2, 2)
			Expect(err).ShouldNot(HaveOccurred())
			pbEvt := &pbv1.CloudEvent{}
			err = grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())

			By("should subscribe to the resource deleted status with grpc client")
			Eventually(func() error {
				if subscribedResourceStatus == nil {
					return fmt.Errorf("subscribedResourceStatus is nil")
				}
				if subscribedResourceStatus.ResourceID != resourceID {
					return fmt.Errorf("resource status for resource %s not found", resourceID)
				}
				if subscribedResourceStatus.Status == nil {
					return fmt.Errorf("resource status is empty")
				}
				resourceBundleStatus := subscribedResourceStatus.Status
				if resourceBundleStatus.ManifestBundleStatus == nil {
					return fmt.Errorf("resource bundle status is empty")
				}

				if !meta.IsStatusConditionTrue(resourceBundleStatus.ManifestBundleStatus.Conditions, common.ResourceDeleted) {
					return fmt.Errorf("resource is not deleted")
				}

				return nil
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

			_, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).To(HaveOccurred(), "Expected 404 error")
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			// stop the resource status subscriber
			subscriberCancel()
		})

		It("should subscribe to the resource status with grpc client", func() {
			Eventually(func() error {
				return assertResourceStatus(subscribedResourceStatus, resourceID, 1)
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})

		It("get the deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})

		It("get the resource via maestro api", func() {
			gotResource, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(resourceID))
			Expect(*gotResource.Version).To(Equal(int32(1)))
		})

		It("publish a resource update with grpc client", func() {
			evt, err := helper.NewEvent(sourceID, "update_request", agentTestOpts.consumerName, resourceID, deployName, 1, 2)
			Expect(err).ShouldNot(HaveOccurred())
			pbEvt := &pbv1.CloudEvent{}
			err = grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt)
			Expect(err).To(BeNil(), "failed to convert spec from cloudevent to protobuf")
			_, err = grpcClient.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should subscribe to the resource status with grpc client", func() {
			Eventually(func() error {
				return assertResourceStatus(subscribedResourceStatus, resourceID, 2)
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})

		It("get the deployment from cluster", func() {
			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, normalEventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})

		It("get the resource via maestro api", func() {
			gotResource, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceID).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*gotResource.Id).To(Equal(resourceID))
			Expect(*gotResource.Version).To(Equal(int32(2)))
		})
	})
})

type SubscribedResourceStatus struct {
	ResourceID string
	Status     *api.ResourceBundleStatus
}

func startStatusSubscribe(ctx context.Context, sourceID string, grpcClient pbv1.CloudEventServiceClient, subscribedResourceStatus *SubscribedResourceStatus) {
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
		subscribedResourceStatus.ResourceID = resID
		resourceVersion, err := cetypes.ToInteger(evtExtensions[types.ExtensionResourceVersion])
		if err != nil {
			continue
		}
		subscribedResourceStatus.Status = &api.ResourceBundleStatus{
			ObservedVersion:      resourceVersion,
			ManifestBundleStatus: &payload.ManifestBundleStatus{},
		}
		if err := evt.DataAs(subscribedResourceStatus.Status.ManifestBundleStatus); err != nil {
			continue
		}
	}
}

func assertResourceStatus(subscribedResourceStatus *SubscribedResourceStatus, resourceID string, expectedReplicas int32) error {
	if subscribedResourceStatus == nil {
		return fmt.Errorf("subscribedResourceStatus is nil")
	}
	if subscribedResourceStatus.ResourceID != resourceID {
		return fmt.Errorf("resource status for resource %s not found", resourceID)
	}
	if subscribedResourceStatus.Status == nil {
		return fmt.Errorf("resource status is empty")
	}
	resourceBundleStatus := subscribedResourceStatus.Status
	if !meta.IsStatusConditionTrue(resourceBundleStatus.Conditions, "Applied") {
		return fmt.Errorf("resource not applied")
	}

	if !meta.IsStatusConditionTrue(resourceBundleStatus.Conditions, "Available") {
		return fmt.Errorf("resource not Available")
	}

	if len(resourceBundleStatus.ResourceStatus) != 1 {
		return fmt.Errorf("unexpected number of resource status, expected 1, got %d", len(resourceBundleStatus.ResourceStatus))
	}

	resourceStatus := resourceBundleStatus.ResourceStatus[0]
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

	if int32(replicas.(float64)) != expectedReplicas {
		return fmt.Errorf("unexpected replicas, expected %d, got %d", expectedReplicas, int32(replicas.(float64)))
	}

	return nil
}
