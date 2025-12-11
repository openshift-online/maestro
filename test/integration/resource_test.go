package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cemetrics "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/metrics"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/test"
)

func TestResourceBundleGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())

	res, resp, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, resource.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*res.Id).To(Equal(resource.ID), "found object does not match test object")
	Expect(*res.Name).To(Equal(resource.Name))
	Expect(*res.Kind).To(Equal("ResourceBundle"))
	Expect(*res.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", resource.ID)))
	Expect(*res.CreatedAt).To(BeTemporally("~", resource.CreatedAt))
	Expect(*res.UpdatedAt).To(BeTemporally("~", resource.UpdatedAt))
	Expect(*res.Version).To(Equal(resource.Version))

	// expectedMetric := `
	// # HELP rest_api_inbound_request_count Number of requests served.
	// # TYPE rest_api_inbound_request_count counter
	// rest_api_inbound_request_count{code="200",method="GET",path="/api/maestro/v1/resource-bundles/-"} 1
	// rest_api_inbound_request_count{code="404",method="GET",path="/api/maestro/v1/resource-bundles/-"} 1
	// `

	// if err := testutil.GatherAndCompare(prometheus.DefaultGatherer,
	// 	strings.NewReader(expectedMetric), "rest_api_inbound_request_count"); err != nil {
	// 	t.Errorf("unexpected metrics: %v", err)
	// }
}

func TestResourcePaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	_, err = h.CreateResourceList(consumer.Name, 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestResourceListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	resources, err := h.CreateResourceList(consumer.Name, 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("name = '%s' and consumer_name = '%s'", resources[0].Name, consumer.Name)
	list, _, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(resources[0].ID))
	Expect(*list.Items[0].Name).To(Equal(resources[0].Name))

	search = fmt.Sprintf("consumer_name = '%s'", consumer.Name)
	list, _, err = client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Total).To(Equal(int32(20)))
}

func TestUpdateResourceWithRacingRequests(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	var objids []string
	rows, err := h.DBFactory.DirectDB().Query("SELECT objid FROM pg_locks WHERE locktype='advisory'")
	Expect(err).NotTo(HaveOccurred(), "Error querying pg_locks: %v", err)
	for rows.Next() {
		var objid string
		Expect(rows.Scan(&objid)).NotTo(HaveOccurred(), "Error scanning pg_locks value: %v", err)
		objids = append(objids, objid)
	}
	rows.Close()
	time.Sleep(time.Second)

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())
	newResource, err := h.NewResource(resource.ID, consumer.Name, deployName, "default", 2, resource.Version)
	Expect(err).NotTo(HaveOccurred())
	newResource.ID = resource.ID

	// starts 20 threads to update this resource at the same time
	threads := 20
	conflictRequests := 0
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			_, err := h.UpdateResource(newResource)
			if err != nil && strings.Contains(err.Error(), fmt.Sprintf("%d", errors.ErrorConflict)) {
				conflictRequests = conflictRequests + 1
			}
		}()
	}

	// waits for all goroutines above to complete
	wg.Wait()

	// there should only be one thread successful update request
	Expect(conflictRequests).To(Equal(threads - 1))

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := dao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)

	updatedCount := 0
	for _, e := range events {
		if e.SourceID == resource.ID && e.EventType == api.UpdateEventType {
			updatedCount = updatedCount + 1
		}
	}

	// the resource patch request is protected by the advisory lock, so there should only be one update
	Expect(updatedCount).To(Equal(1))

	// ensure the locks for current test are released
	query := fmt.Sprintf("select count(*) from pg_locks where locktype='advisory' and objid not in (%s)", strings.Join(objids, ","))
	if len(objids) == 0 {
		query = "select count(*) from pg_locks where locktype='advisory'"
	}

	// ensure the locks for current test are released finally
	Eventually(func() error {
		var count int
		err := h.DBFactory.DirectDB().
			QueryRow(query).
			Scan(&count)
		Expect(err).NotTo(HaveOccurred(), "Error querying pg_locks:  %v", err)

		if count != 0 {
			return fmt.Errorf("there are %d unreleased advisory lock", count)
		}
		return nil
	}, 20*time.Second, 1*time.Second).Should(Succeed())
}

func TestResourceFromGRPC(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()

	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
		// give one second to terminate the work agent
		time.Sleep(1 * time.Second)
	}()
	// create a mock resource
	clusterName := "cluster-" + rand.String(5)
	consumer, err := h.CreateConsumer(clusterName)
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.NewResource(uuid.NewString(), consumer.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())
	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)

	time.Sleep(3 * time.Second)
	// reset metrics to avoid interference from other tests
	cemetrics.ResetSourceCloudEventsMetrics()
	server.ResetGRPCMetrics()

	// use grpc client to create resource
	h.StartGRPCResourceSourceClient()
	err = h.GRPCSourceClient.Publish(ctx, cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.CreateRequestAction,
	}, resource)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	// for real case, the controller should have a mapping between resource (replicated) in maestro and resource (root) in kubernetes
	// so call subscribe method can return the resource
	// for testing, just list the resource via restful api.
	resources, _, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resources.Items).NotTo(BeEmpty(), "Expected returned resource list is not empty")

	res, resp, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, *resources.Items[0].Id).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*res.Id).To(Equal(resource.ID))
	Expect(*res.Kind).To(Equal("ResourceBundle"))
	Expect(*res.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", *res.Id)))
	Expect(*res.Version).To(Equal(int32(1)))

	// add the resource to the store
	h.Store.Add(resource)

	var work *workv1.ManifestWork
	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, resource.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	Expect(work).NotTo(BeNil())
	Expect(work.Spec.Workload).NotTo(BeNil())
	Expect(len(work.Spec.Workload.Manifests)).To(Equal(1))
	manifest := map[string]interface{}{}
	Expect(json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &manifest)).NotTo(HaveOccurred(), "Error unmarshalling manifest:  %v", err)

	// update the resource
	statusFeedbackValue := `{"observedGeneration":1,"replicas":1,"availableReplicas":1,"readyReplicas":1,"updatedReplicas":1}`
	newWorkStatus := workv1.ManifestWorkStatus{
		ResourceStatus: workv1.ManifestResourceStatus{
			Manifests: []workv1.ManifestCondition{
				{
					Conditions: []metav1.Condition{
						{
							Type:   "Applied",
							Status: metav1.ConditionTrue,
						},
					},
					StatusFeedbacks: workv1.StatusFeedbackResult{
						Values: []workv1.FeedbackValue{
							{
								Name: "status",
								Value: workv1.FieldValue{
									Type:    workv1.JsonRaw,
									JsonRaw: &statusFeedbackValue,
								},
							},
						},
					},
				},
			},
		},
	}

	// update the work status
	Expect(updateWorkStatus(ctx, agentWorkClient, work, newWorkStatus)).NotTo(HaveOccurred())

	Eventually(func() error {
		foundResource, err := h.Store.Get(resource.ID)
		if err != nil {
			return err
		}
		if len(foundResource.Status) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		evt, err := api.JSONMAPToCloudEvent(foundResource.Status)
		if err != nil {
			return fmt.Errorf("failed to convert jsonmap to cloudevent")
		}

		manifestStatus := &workpayload.ManifestBundleStatus{}
		if err := evt.DataAs(manifestStatus); err != nil {
			return fmt.Errorf("failed to unmarshal event payload: %v", err)
		}

		resourceStatus := manifestStatus.ResourceStatus
		if len(resourceStatus) != 1 {
			return fmt.Errorf("unexpected length of resourceStatus")
		}

		if !meta.IsStatusConditionTrue(resourceStatus[0].Conditions, "Applied") {
			return fmt.Errorf("resource status is not applied")
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	newResource, err := h.NewResource(resource.ID, consumer.Name, deployName, "default", 2, 1)
	Expect(err).NotTo(HaveOccurred())
	newResource.ID = *res.Id
	newResource.Version = *res.Version
	err = h.GRPCSourceClient.Publish(ctx, cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.UpdateRequestAction,
	}, newResource)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	res, resp, err = client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, newResource.ID).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*res.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*res.Kind).To(Equal("ResourceBundle"))
	Expect(*res.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", *res.Id)))
	Expect(*res.Version).To(Equal(int32(2)))

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, *res.Id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// ensure the work version is updated
		if work.GetGeneration() != 2 {
			return fmt.Errorf("unexpected work version %v", work.GetGeneration())
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	Expect(work).NotTo(BeNil())
	Expect(work.Spec.Workload).NotTo(BeNil())
	Expect(len(work.Spec.Workload.Manifests)).To(Equal(1))
	manifest = map[string]interface{}{}
	Expect(json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &manifest)).NotTo(HaveOccurred(), "Error unmarshalling manifest:  %v", err)
	Expect(manifest["spec"].(map[string]interface{})["replicas"]).To(Equal(float64(2)))

	err = h.GRPCSourceClient.Publish(ctx, cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.DeleteRequestAction,
	}, newResource)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, newResource.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if work.GetDeletionTimestamp() == nil {
			return fmt.Errorf("work %s is not deleted", work.Name)
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// no real kubernete environment, so need to update the resource status manually
	deletingWorkStatus := workv1.ManifestWorkStatus{
		Conditions: []metav1.Condition{
			{
				Type:   common.ResourceDeleted,
				Status: metav1.ConditionTrue,
			},
		},
	}

	// update the work status
	Expect(updateWorkStatus(ctx, agentWorkClient, work, deletingWorkStatus)).NotTo(HaveOccurred())

	Eventually(func() error {
		res, _, err = client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, newResource.ID).Execute()
		if res != nil {
			return fmt.Errorf("resource %s is not deleted", newResource.ID)
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	time.Sleep(3 * time.Second)

	expectedMetrics := `
	# HELP grpc_server_registered_source_clients Number of registered source clients on the grpc server.
    # TYPE grpc_server_registered_source_clients gauge
	grpc_server_registered_source_clients{source="maestro"} 1
	# HELP grpc_server_called_total Total number of RPCs called on the server.
	# TYPE grpc_server_called_total counter
	grpc_server_called_total{code="OK",source="maestro",type="Publish"} 3
	grpc_server_called_total{code="OK",source="maestro",type="Subscribe"} 1
	# HELP grpc_server_message_received_total Total number of messages received on the server from agent and client.
	# TYPE grpc_server_message_received_total counter
	grpc_server_message_received_total{source="maestro",type="Publish"} 3
	grpc_server_message_received_total{source="maestro",type="Subscribe"} 1
	# HELP grpc_server_processed_total Total number of RPCs processed on the server, regardless of success or failure.
	# TYPE grpc_server_processed_total counter
	grpc_server_processed_total{code="OK",source="maestro",type="Publish"} 3
	`

	if h.Broker != "grpc" {
		expectedMetrics += fmt.Sprintf(`
		# HELP cloudevents_sent_total The total number of CloudEvents sent from source.
		# TYPE cloudevents_sent_total counter
		cloudevents_sent_total{action="create_request",consumer="%s",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 2
		cloudevents_sent_total{action="delete_request",consumer="%s",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 2
		cloudevents_sent_total{action="update_request",consumer="%s",source="maestro",subresource="spec",type="io.open-cluster-management.works.v1alpha1.manifestbundles"} 2
		`, clusterName, clusterName, clusterName)
	}

	if err := testutil.GatherAndCompare(prometheus.DefaultGatherer,
		strings.NewReader(expectedMetrics), "cloudevents_sent_total", "grpc_server_registered_source_clients", "grpc_server_message_received_total", "grpc_server_processed_total"); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestMarkAsDeletingThenUpdate(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Create a consumer and resource
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())

	resourceService := h.Env().Services.Resources()
	err = resourceService.MarkAsDeleting(ctx, resource.ID)
	Expect(err).NotTo(HaveOccurred())

	statusRes := &api.Resource{
		Meta: api.Meta{
			ID: resource.ID,
		},
		Version: resource.Version,
		Status:  createStatusWithSequenceID(t, resource.ID, fmt.Sprintf("%d", 1)),
	}
	_, updated, svcErr := resourceService.UpdateStatus(ctx, statusRes)
	Expect(svcErr).NotTo(HaveOccurred())
	Expect(updated).Should(Equal(true))

	res, err := resourceService.Get(ctx, resource.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(res.Status)).ShouldNot(Equal(0))
}

func updateWorkStatus(ctx context.Context, workClient workv1client.ManifestWorkInterface, work *workv1.ManifestWork, newStatus workv1.ManifestWorkStatus) error {
	// update the work status
	newWork := work.DeepCopy()
	newWork.Status = newStatus

	oldData, err := json.Marshal(work)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newWork)
	if err != nil {
		return err
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return err
	}

	_, err = workClient.Patch(ctx, work.Name, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
	}

	return nil
}

// TestUpdateAndUpdateStatusIsolation ensures that Update and UpdateStatus operations
// work independently without affecting each other. Specifically:
// 1. Create a resource
// 2. Update the status
// 3. Update the resource payload
// 4. Verify the status from step 2 is preserved and not affected by the payload update
func TestUpdateAndUpdateStatusIsolation(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Step 1: Create a resource
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())
	Expect(resource.Version).To(Equal(int32(1)))
	Expect(len(resource.Status)).To(Equal(0), "Initial resource should have no status")

	resourceService := h.Env().Services.Resources()

	// Step 2: Update the status with sequence ID "1"
	statusRes1 := &api.Resource{
		Meta: api.Meta{
			ID: resource.ID,
		},
		Version: resource.Version,
		Status:  createStatusWithSequenceID(t, resource.ID, "1"),
	}
	_, updated, svcErr := resourceService.UpdateStatus(ctx, statusRes1)
	Expect(svcErr).NotTo(HaveOccurred())
	Expect(updated).Should(BeTrue(), "Status should be updated")

	// Verify status was set correctly
	updatedRes, svcErr := resourceService.Get(ctx, resource.ID)
	Expect(svcErr).NotTo(HaveOccurred())
	Expect(len(updatedRes.Status)).ShouldNot(Equal(0), "Status should not be empty after update")
	statusEvt, err := api.JSONMAPToCloudEvent(updatedRes.Status)
	Expect(err).NotTo(HaveOccurred())
	statusPayload := &workpayload.ManifestBundleStatus{}
	Expect(statusEvt.DataAs(statusPayload)).NotTo(HaveOccurred())
	Expect(statusPayload.Conditions).To(HaveLen(1))
	Expect(statusPayload.Conditions[0].Type).To(Equal("Applied"))
	Expect(statusPayload.Conditions[0].Status).To(Equal(metav1.ConditionStatus("True")))
	initialStatusMessage := statusPayload.Conditions[0].Message

	// Version should still be 1 (UpdateStatus doesn't increment version)
	Expect(updatedRes.Version).To(Equal(int32(1)))

	// Step 3: Update the resource payload (change replicas from 1 to 3)
	newResource, err := h.NewResource(resource.ID, consumer.Name, deployName, "default", 3, updatedRes.Version)
	Expect(err).NotTo(HaveOccurred())
	newResource.ID = updatedRes.ID
	newResource.Status = createStatusWithSequenceID(t, resource.ID, "2")

	updatedPayloadRes, svcErr := resourceService.Update(ctx, newResource)
	Expect(svcErr).NotTo(HaveOccurred())
	Expect(updatedPayloadRes.Version).To(Equal(int32(2)), "Version should be incremented after Update")

	// Step 4: Verify that the status is preserved and unchanged
	finalRes, svcErr := resourceService.Get(ctx, resource.ID)
	Expect(svcErr).NotTo(HaveOccurred())

	// Status should still be present and unchanged
	Expect(len(finalRes.Status)).ShouldNot(Equal(0), "Status should be preserved after Update")

	finalStatusEvt, err := api.JSONMAPToCloudEvent(finalRes.Status)
	Expect(err).NotTo(HaveOccurred())
	finalStatusPayload := &workpayload.ManifestBundleStatus{}
	Expect(finalStatusEvt.DataAs(finalStatusPayload)).NotTo(HaveOccurred())
	Expect(finalStatusPayload.Conditions).To(HaveLen(1))
	Expect(finalStatusPayload.Conditions[0].Type).To(Equal("Applied"))
	Expect(finalStatusPayload.Conditions[0].Status).To(Equal(metav1.ConditionStatus("True")))
	Expect(finalStatusPayload.Conditions[0].Message).To(Equal(initialStatusMessage), "Status message should be unchanged")

	// Verify the payload was actually updated (replicas changed from 1 to 3)
	payloadEvt, err := api.JSONMAPToCloudEvent(finalRes.Payload)
	Expect(err).NotTo(HaveOccurred())
	payloadData := &workpayload.ManifestBundle{}
	Expect(payloadEvt.DataAs(payloadData)).NotTo(HaveOccurred())
	Expect(payloadData.Manifests).To(HaveLen(1))

	var manifest map[string]interface{}
	Expect(json.Unmarshal(payloadData.Manifests[0].Raw, &manifest)).NotTo(HaveOccurred())
	spec := manifest["spec"].(map[string]interface{})
	Expect(spec["replicas"]).To(Equal(float64(3)), "Replicas should be updated to 3")

	// Additional test: Update status again to verify it still works after a payload update
	statusRes2 := &api.Resource{
		Meta: api.Meta{
			ID: resource.ID,
		},
		Version: finalRes.Version, // Now version 2
		Status:  createStatusWithSequenceID(t, resource.ID, "2"),
	}
	updatedRes2, updated2, svcErr := resourceService.UpdateStatus(ctx, statusRes2)
	Expect(svcErr).NotTo(HaveOccurred())
	Expect(updated2).Should(BeTrue(), "Status should be updated again")
	Expect(updatedRes2.Version).To(Equal(int32(2)), "Version should remain 2 after UpdateStatus")

	// Verify the new status
	newStatusEvt, err := api.JSONMAPToCloudEvent(updatedRes2.Status)
	Expect(err).NotTo(HaveOccurred())
	newStatusPayload := &workpayload.ManifestBundleStatus{}
	Expect(newStatusEvt.DataAs(newStatusPayload)).NotTo(HaveOccurred())
	Expect(newStatusPayload.Conditions[0].Message).To(ContainSubstring("sequence 2"), "Status should reflect new update")
}

// createStatusWithSequenceID creates a resource status CloudEvent with the given sequence ID
func createStatusWithSequenceID(t *testing.T, resourceID, sequenceID string) map[string]interface{} {
	source := "test-agent"
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceStatus,
		Action:              cetypes.UpdateRequestAction,
	}

	// Create a cloud event with status data
	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithResourceID(resourceID).
		WithResourceVersion(1).
		WithStatusUpdateSequenceID(sequenceID)

	evt := evtBuilder.NewEvent()

	// Create a simple status payload
	statusPayload := &workpayload.ManifestBundleStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "Applied",
				Status:  "True",
				Message: fmt.Sprintf("Test status update with sequence %s", sequenceID),
			},
		},
	}

	// Set the event data
	if err := evt.SetData(cloudevents.ApplicationJSON, statusPayload); err != nil {
		t.Fatalf("failed to set cloud event data: %v", err)
	}

	// Convert cloudevent to JSONMap
	statusMap, err := api.CloudEventToJSONMap(&evt)
	if err != nil {
		t.Fatalf("failed to convert cloudevent to status map: %v", err)
	}

	return statusMap
}
