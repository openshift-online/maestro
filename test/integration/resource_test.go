package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	prommodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"

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
	_, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource, err := h.CreateResource(consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())

	res, resp, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resource.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*res.Id).To(Equal(resource.ID), "found object does not match test object")
	Expect(*res.Name).To(Equal(resource.Name))
	Expect(*res.Kind).To(Equal("ResourceBundle"))
	Expect(*res.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", resource.ID)))
	Expect(*res.CreatedAt).To(BeTemporally("~", resource.CreatedAt))
	Expect(*res.UpdatedAt).To(BeTemporally("~", resource.UpdatedAt))
	Expect(*res.Version).To(Equal(resource.Version))

	families := getServerMetrics(t, "http://localhost:8080/metrics")
	labels := []*prommodel.LabelPair{
		{Name: strPtr("method"), Value: strPtr("GET")},
		{Name: strPtr("path"), Value: strPtr("/api/maestro/v1/resource-bundles/-")},
		{Name: strPtr("code"), Value: strPtr("200")},
	}
	checkServerCounterMetric(t, families, "rest_api_inbound_request_count", labels, 1.0)
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

	list, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Page(2).Size(5).Execute()
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
	list, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(resources[0].ID))
	Expect(*list.Items[0].Name).To(Equal(resources[0].Name))

	search = fmt.Sprintf("consumer_name = '%s'", consumer.Name)
	list, _, err = client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
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
	resource, err := h.CreateResource(consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())
	newResource, err := h.NewResource(consumer.Name, deployName, "default", 2, resource.Version)
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
	resource, err := h.NewResource(consumer.Name, deployName, "default", 1, 1)
	Expect(err).NotTo(HaveOccurred())
	resource.ID = uuid.NewString()

	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)

	// use grpc client to create resource
	h.StartGRPCResourceSourceClient()
	err = h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.CreateRequestAction,
	}, resource)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	// for real case, the controller should have a mapping between resource (replicated) in maestro and resource (root) in kubernetes
	// so call subscribe method can return the resource
	// for testing, just list the resource via restful api.
	resources, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resources.Items).NotTo(BeEmpty(), "Expected returned resource list is not empty")

	res, resp, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, *resources.Items[0].Id).Execute()
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
		if foundResource.Status == nil || len(foundResource.Status) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		evt, err := api.JSONMAPToCloudEvent(foundResource.Status)
		if err != nil {
			return fmt.Errorf("failed to convert jsonmap to cloudevent")
		}

		manifestStatus := &payload.ManifestBundleStatus{}
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

	newResource, err := h.NewResource(consumer.Name, deployName, "default", 2, 1)
	Expect(err).NotTo(HaveOccurred())
	newResource.ID = *res.Id
	newResource.Version = *res.Version
	err = h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.UpdateRequestAction,
	}, newResource)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	res, resp, err = client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, newResource.ID).Execute()
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
		if work.GetResourceVersion() != "2" {
			return fmt.Errorf("unexpected work version %v", work.GetResourceVersion())
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	Expect(work).NotTo(BeNil())
	Expect(work.Spec.Workload).NotTo(BeNil())
	Expect(len(work.Spec.Workload.Manifests)).To(Equal(1))
	manifest = map[string]interface{}{}
	Expect(json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &manifest)).NotTo(HaveOccurred(), "Error unmarshalling manifest:  %v", err)
	Expect(manifest["spec"].(map[string]interface{})["replicas"]).To(Equal(float64(2)))

	err = h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.DeleteRequestAction,
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
				Type:   common.ManifestsDeleted,
				Status: metav1.ConditionTrue,
			},
		},
	}

	// update the work status
	Expect(updateWorkStatus(ctx, agentWorkClient, work, deletingWorkStatus)).NotTo(HaveOccurred())

	Eventually(func() error {
		res, _, err = client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, newResource.ID).Execute()
		if res != nil {
			return fmt.Errorf("resource %s is not deleted", newResource.ID)
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	time.Sleep(1 * time.Second)
	families := getServerMetrics(t, "http://localhost:8080/metrics")
	labels := []*prommodel.LabelPair{
		{Name: strPtr("type"), Value: strPtr("Publish")},
		{Name: strPtr("source"), Value: strPtr("maestro")},
	}
	checkServerCounterMetric(t, families, "grpc_server_called_total", labels, 3.0)
	checkServerCounterMetric(t, families, "grpc_server_message_received_total", labels, 3.0)
	checkServerCounterMetric(t, families, "grpc_server_message_sent_total", labels, 3.0)

	labels = []*prommodel.LabelPair{
		{Name: strPtr("type"), Value: strPtr("Subscribe")},
		{Name: strPtr("source"), Value: strPtr("maestro")},
	}
	checkServerCounterMetric(t, families, "grpc_server_called_total", labels, 1.0)
	checkServerCounterMetric(t, families, "grpc_server_message_received_total", labels, 1.0)
	//checkServerCounterMetric(t, families, "maestro_grpc_server_msg_sent_total", labels, 2.0)

	labels = []*prommodel.LabelPair{
		{Name: strPtr("type"), Value: strPtr("Publish")},
		{Name: strPtr("source"), Value: strPtr("maestro")},
		{Name: strPtr("code"), Value: strPtr("OK")},
	}
	checkServerCounterMetric(t, families, "grpc_server_processed_total", labels, 3.0)

	labels = []*prommodel.LabelPair{
		{Name: strPtr("type"), Value: strPtr("Subscribe")},
		{Name: strPtr("source"), Value: strPtr("maestro")},
		{Name: strPtr("code"), Value: strPtr("OK")},
	}
	checkServerCounterMetric(t, families, "grpc_server_processed_total", labels, 0.0)

	if h.Broker != "grpc" {
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("create_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_received_total", labels, 1.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("update_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_received_total", labels, 1.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("delete_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_received_total", labels, 1.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr(clusterName)},
			{Name: strPtr("original_source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifestbundles")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceStatus))},
			{Name: strPtr("action"), Value: strPtr("update_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 2.0)
	}
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

func getServerMetrics(t *testing.T, url string) map[string]*prommodel.MetricFamily {
	// gather metrics from metrics server from url /metrics
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("Error getting metrics:  %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Error getting metrics with status code:  %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Error reading metrics:  %v", err)
	}
	parser := expfmt.TextParser{}
	// Ensure EOL
	reader := strings.NewReader(strings.ReplaceAll(string(body), "\r\n", "\n"))
	families, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		t.Errorf("Error parsing metrics:  %v", err)
	}

	return families
}

func checkServerCounterMetric(t *testing.T, families map[string]*prommodel.MetricFamily, name string, labels []*prommodel.LabelPair, value float64) {
	family, ok := families[name]
	if !ok {
		t.Errorf("Metric %s not found", name)
	}
	metricValue := 0.0
	metrics := family.GetMetric()
	for _, metric := range metrics {
		metricLabels := metric.GetLabel()
		if !compareMetricLabels(labels, metricLabels) {
			continue
		}
		metricValue += *metric.Counter.Value
	}
	if metricValue != value {
		t.Errorf("Counter metric %s value is %f, expected %f", name, metricValue, value)
	}
}

func compareMetricLabels(labels []*prommodel.LabelPair, metricLabels []*prommodel.LabelPair) bool {
	if len(labels) != len(metricLabels) {
		return false
	}
	for _, label := range labels {
		match := false
		for _, metricLabel := range metricLabels {
			if *label.Name == *metricLabel.Name && *label.Value == *metricLabel.Value {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func strPtr(s string) *string {
	return &s
}
