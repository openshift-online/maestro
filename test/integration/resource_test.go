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
	"gopkg.in/resty.v1"
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
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

func TestResourceGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource := h.CreateResource(consumer.Name, deployName, 1)

	res, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, resource.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*res.Id).To(Equal(resource.ID), "found object does not match test object")
	Expect(*res.Kind).To(Equal("Resource"))
	Expect(*res.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", resource.ID)))
	Expect(*res.CreatedAt).To(BeTemporally("~", resource.CreatedAt))
	Expect(*res.UpdatedAt).To(BeTemporally("~", resource.UpdatedAt))
	Expect(*res.Version).To(Equal(resource.Version))
}

func TestResourcePost(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
	}()

	clusterName := "cluster-" + rand.String(5)
	consumer := h.CreateConsumer(clusterName)
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.NewAPIResource(consumer.Name, deployName, 1)
	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name, false)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)

	// POST responses per openapi spec: 201, 400, 409, 500

	// 201 Created
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*resource.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", *resource.Id)))

	// 400 bad request. posting junk json is one way to trigger 400.
	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/resources"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	var work *workv1.ManifestWork
	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, *resource.Id, metav1.GetOptions{})
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
	Expect(manifest).To(Equal(res.Manifest))

	statusFeedbackValue := `{"replicas":1,"availableReplicas":1,"readyReplicas":1,"updatedReplicas":1}`
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

	var newRes *openapi.Resource
	Eventually(func() error {
		newRes, _, err = client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resource.Id).Execute()
		if err != nil {
			return err
		}
		if newRes.Status == nil || len(newRes.Status) == 0 ||
			newRes.Status["ReconcileStatus"] == nil || newRes.Status["ContentStatus"] == nil {
			return fmt.Errorf("resource status is empty")
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	Expect(err).NotTo(HaveOccurred(), "Error getting resource: %v", err)
	Expect(newRes.Version).To(Equal(resource.Version))
	Expect(newRes.Status["ReconcileStatus"]).NotTo(BeNil())
	reconcileStatus := newRes.Status["ReconcileStatus"].(map[string]interface{})
	observedVersion, ok := reconcileStatus["ObservedVersion"].(float64)
	Expect(ok).To(BeTrue())
	Expect(int32(observedVersion)).To(Equal(*resource.Version))
	conditions := reconcileStatus["Conditions"].([]interface{})
	Expect(len(conditions)).To(Equal(1))
	condition := conditions[0].(map[string]interface{})
	Expect(condition["type"]).To(Equal("Applied"))
	Expect(condition["status"]).To(Equal("True"))

	contentStatus := newRes.Status["ContentStatus"].(map[string]interface{})
	Expect(contentStatus["replicas"]).To(Equal(float64(1)))
	Expect(contentStatus["availableReplicas"]).To(Equal(float64(1)))
	Expect(contentStatus["readyReplicas"]).To(Equal(float64(1)))
	Expect(contentStatus["updatedReplicas"]).To(Equal(float64(1)))

	if h.Broker != "grpc" {
		time.Sleep(1 * time.Second)
		families := getServerMetrics(t, "http://localhost:8080/metrics")
		labels := []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("create_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 1.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr(clusterName)},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceStatus))},
			{Name: strPtr("action"), Value: strPtr("update_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_received_total", labels, 1.0)
	}
}

func TestResourcePostWithoutName(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	clusterName := "cluster-" + rand.String(5)
	consumer := h.CreateConsumer(clusterName)
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.NewAPIResource(consumer.Name, deployName, 1)
	h.StartControllerManager(ctx)
	resourceService := h.Env().Services.Resources()
	// POST responses per openapi spec: 201, 400, 409, 500

	// 201 Created
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*resource.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*resource.Name).To(Equal(*resource.Id))

	// 201 Created
	resource, resp, err = client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*resource.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*resource.Name).To(Equal(*resource.Id))

	Eventually(func() error {
		newRes, err := resourceService.List(types.ListOptions{ClusterName: clusterName, CloudEventsDataType: payload.ManifestEventDataType})
		if err != nil {
			return err
		}
		if len(newRes) != 2 {
			return fmt.Errorf("should create two resources")
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// make sure controller manager and work agent are stopped
	cancel()
}

func TestResourcePostWithName(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	clusterName := "cluster-" + rand.String(5)
	consumer := h.CreateConsumer(clusterName)
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.NewAPIResource(consumer.Name, deployName, 1)
	h.StartControllerManager(ctx)
	// POST responses per openapi spec: 201, 400, 409, 500

	// 201 Created
	resourceName := "ngix"
	res.Name = &resourceName
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*resource.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*resource.Name).To(Equal(resourceName))

	// 201 Created
	_, _, err = client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).To(HaveOccurred())

	// make sure controller manager and work agent are stopped
	cancel()
}

func TestResourcePatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
	}()
	// use the consumer id as the consumer name
	consumer := h.CreateConsumer("")

	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.ID, false)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.ID)

	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.CreateResource(consumer.ID, deployName, 1)
	Expect(res.Version).To(Equal(int32(1)))

	var work *workv1.ManifestWork
	Eventually(func() error {
		// ensure the work can be get by work client
		var err error
		work, err = agentWorkClient.Get(ctx, res.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// add finalizer to the work
		patchBytes, err := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"uid":             work.GetUID(),
				"resourceVersion": work.GetResourceVersion(),
				"finalizers":      []string{"work-test-finalizer"},
			},
		})
		if err != nil {
			return err
		}

		_, err = agentWorkClient.Patch(ctx, work.Name, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}, 20*time.Second, 2*time.Second).Should(Succeed())

	// 200 OK
	newRes := h.NewAPIResource(consumer.Name, deployName, 2)
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(openapi.ResourcePatchRequest{Version: &res.Version, Manifest: newRes.Manifest}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*resource.Id).To(Equal(res.ID))
	Expect(*resource.CreatedAt).To(BeTemporally("~", res.CreatedAt))
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", *resource.Id)))
	Expect(*resource.Version).To(Equal(res.Version + 1))
	Expect(resource.Manifest).To(Equal(map[string]interface{}(newRes.Manifest)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	// 500 server error. posting junk json is one way to trigger 500.
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/resources/foo"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := dao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)
	Expect(len(events)).To(Equal(2), "expected Create and Update events")
	Expect(contains(api.CreateEventType, events)).To(BeTrue())
	Expect(contains(api.UpdateEventType, events)).To(BeTrue())

	// 409 conflict error. using an out of date resource version
	_, resp, err = client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(
		openapi.ResourcePatchRequest{Version: &res.Version, Manifest: newRes.Manifest}).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusConflict))

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, *resource.Id, metav1.GetOptions{})
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
	manifest := map[string]interface{}{}
	Expect(json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &manifest)).NotTo(HaveOccurred(), "Error unmarshalling manifest:  %v", err)
	Expect(manifest).To(Equal(newRes.Manifest))

	// initialize resource deletion
	_, err = client.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, res.ID).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error deleting object:  %v", err)

	// patch the deleting resource should return 409 conflict
	_, resp, err = client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(
		openapi.ResourcePatchRequest{Version: &res.Version, Manifest: newRes.Manifest}).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusConflict))
}

func TestResourcePaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	_ = h.CreateResourceList(consumer.Name, 20)
	_ = h.CreateResourceBundleList(consumer.Name, 20)

	list, _, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(list.Kind).To(Equal("ResourceList"))
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(list.Kind).To(Equal("ResourceList"))
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestResourceListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	resources := h.CreateResourceList(consumer.Name, 20)

	search := fmt.Sprintf("id in ('%s')", resources[0].ID)
	list, _, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(resources[0].ID))
}

func TestResourceBundleGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resourceBundle := h.CreateResourceBundle(consumer.Name, deployName, 1)

	resBundle, resp, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, resourceBundle.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*resBundle.Id).To(Equal(resourceBundle.ID), "found object does not match test object")
	Expect(*resBundle.Name).To(Equal(resourceBundle.Name))
	Expect(*resBundle.Kind).To(Equal("ResourceBundle"))
	Expect(*resBundle.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", resourceBundle.ID)))
	Expect(*resBundle.CreatedAt).To(BeTemporally("~", resourceBundle.CreatedAt))
	Expect(*resBundle.UpdatedAt).To(BeTemporally("~", resourceBundle.UpdatedAt))
	Expect(*resBundle.Version).To(Equal(resourceBundle.Version))

	families := getServerMetrics(t, "http://localhost:8080/metrics")
	labels := []*prommodel.LabelPair{
		{Name: strPtr("method"), Value: strPtr("GET")},
		{Name: strPtr("path"), Value: strPtr("/api/maestro/v1/resource-bundles/-")},
		{Name: strPtr("code"), Value: strPtr("200")},
	}
	checkServerCounterMetric(t, families, "rest_api_inbound_request_count", labels, 1.0)
}

func TestResourceBundleListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	resourceBundles := h.CreateResourceBundleList(consumer.Name, 20)
	_ = h.CreateResourceList(consumer.Name, 20)

	search := fmt.Sprintf("name = '%s' and consumer_name = '%s'", resourceBundles[0].Name, consumer.Name)
	list, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource bundle list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(resourceBundles[0].ID))
	Expect(*list.Items[0].Name).To(Equal(resourceBundles[0].Name))

	search = fmt.Sprintf("consumer_name = '%s'", consumer.Name)
	list, _, err = client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource bundle list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Total).To(Equal(int32(20)))

	families := getServerMetrics(t, "http://localhost:8080/metrics")
	labels := []*prommodel.LabelPair{
		{Name: strPtr("method"), Value: strPtr("GET")},
		{Name: strPtr("path"), Value: strPtr("/api/maestro/v1/resource-bundles")},
		{Name: strPtr("code"), Value: strPtr("200")},
	}
	checkServerCounterMetric(t, families, "rest_api_inbound_request_count", labels, 2.0)
}

func TestUpdateResourceWithRacingRequests(t *testing.T) {
	h, client := test.RegisterIntegration(t)

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

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.CreateResource(consumer.Name, deployName, 1)
	newRes := h.NewAPIResource(consumer.Name, deployName, 2)

	// starts 20 threads to update this resource at the same time
	threads := 20
	conflictRequests := 0
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			_, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(
				openapi.ResourcePatchRequest{Version: &res.Version, Manifest: newRes.Manifest}).Execute()
			if err != nil && resp.StatusCode == http.StatusConflict {
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
		if e.SourceID == res.ID && e.EventType == api.UpdateEventType {
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
	consumer := h.CreateConsumer(clusterName)
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.NewResource(consumer.Name, deployName, 1, 1)
	res.ID = uuid.NewString()

	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name, false)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)

	// use grpc client to create resource
	h.StartGRPCResourceSourceClient()
	err := h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.CreateRequestAction,
	}, res)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	// for real case, the controller should have a mapping between resource (replicated) in maestro and resource (root) in kubernetes
	// so call subscribe method can return the resource
	// for testing, just list the resource via restful api.
	resources, _, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resources.Items).NotTo(BeEmpty(), "Expected returned resource list is not empty")

	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, *resources.Items[0].Id).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*resource.Id).To(Equal(res.ID))
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", *resource.Id)))
	Expect(*resource.Version).To(Equal(int32(1)))

	// add the resource to the store
	h.Store.Add(res)

	var work *workv1.ManifestWork
	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, res.ID, metav1.GetOptions{})
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
		newRes, err := h.Store.Get(res.ID)
		if err != nil {
			return err
		}
		if newRes.Status == nil || len(newRes.Status) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		resourceStatusJSON, err := json.Marshal(newRes.Status)
		if err != nil {
			return err
		}
		resourceStatus := &api.ResourceStatus{}
		if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
			return err
		}

		if len(resourceStatus.ReconcileStatus.Conditions) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Applied") {
			return fmt.Errorf("resource status is not applied")
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	newRes := h.NewResource(consumer.Name, deployName, 2, 1)
	newRes.ID = *resource.Id
	newRes.Version = *resource.Version
	err = h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.UpdateRequestAction,
	}, newRes)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	resource, resp, err = client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, newRes.ID).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*resource.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", *resource.Id)))
	Expect(*resource.Version).To(Equal(int32(2)))

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, *resource.Id, metav1.GetOptions{})
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
		CloudEventsDataType: payload.ManifestEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.DeleteRequestAction,
	}, newRes)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, newRes.ID, metav1.GetOptions{})
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
		resource, _, err = client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, newRes.ID).Execute()
		if resource != nil {
			return fmt.Errorf("resource %s is not deleted", newRes.ID)
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
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("create_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 2.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("update_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 2.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr("maestro")},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceSpec))},
			{Name: strPtr("action"), Value: strPtr("delete_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_sent_total", labels, 2.0)
		labels = []*prommodel.LabelPair{
			{Name: strPtr("source"), Value: strPtr(clusterName)},
			{Name: strPtr("cluster"), Value: strPtr(clusterName)},
			{Name: strPtr("type"), Value: strPtr("io.open-cluster-management.works.v1alpha1.manifests")},
			{Name: strPtr("subresource"), Value: strPtr(string(types.SubResourceStatus))},
			{Name: strPtr("action"), Value: strPtr("update_request")},
		}
		checkServerCounterMetric(t, families, "cloudevents_received_total", labels, 3.0)
	}
}

func TestResourceBundleFromGRPC(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
	}()
	// create a mock resource
	clusterName := "cluster-" + rand.String(5)
	consumer := h.CreateConsumer(clusterName)
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res := h.NewResource(consumer.Name, deployName, 1, 1)
	res.ID = uuid.NewString()

	h.StartControllerManager(ctx)
	h.StartWorkAgent(ctx, consumer.Name, true)
	clientHolder := h.WorkAgentHolder
	agentWorkClient := clientHolder.ManifestWorks(consumer.Name)

	// use grpc client to create resource bundle
	h.StartGRPCResourceSourceClient()
	time.Sleep(1 * time.Second)

	err := h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.CreateRequestAction,
	}, res)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource bundle with grpc source client: %v", err)

	// add the resource to the store
	h.Store.Add(res)

	var work *workv1.ManifestWork
	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, res.ID, metav1.GetOptions{})
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
		newRes, err := h.Store.Get(res.ID)
		if err != nil {
			return err
		}
		if newRes.Status == nil || len(newRes.Status) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		resourceStatusJSON, err := json.Marshal(newRes.Status)
		if err != nil {
			return err
		}
		resourceStatus := &api.ResourceStatus{}
		if err := json.Unmarshal(resourceStatusJSON, resourceStatus); err != nil {
			return err
		}

		if len(resourceStatus.ReconcileStatus.Conditions) == 0 {
			return fmt.Errorf("resource status is empty")
		}

		if !meta.IsStatusConditionTrue(resourceStatus.ReconcileStatus.Conditions, "Applied") {
			return fmt.Errorf("resource status is not applied")
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	newRes := h.NewResource(consumer.Name, deployName, 2, 1)
	newRes.ID = res.ID
	err = h.GRPCSourceClient.Publish(ctx, types.CloudEventsType{
		CloudEventsDataType: payload.ManifestBundleEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              common.UpdateRequestAction,
	}, newRes)
	Expect(err).NotTo(HaveOccurred(), "Error publishing resource with grpc source client: %v", err)

	Eventually(func() error {
		// ensure the work can be get by work client
		work, err = agentWorkClient.Get(ctx, res.ID, metav1.GetOptions{})
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

	// get the resource bundle with restful API
	resBundle, resp, err := client.DefaultApi.ApiMaestroV1ResourceBundlesIdGet(ctx, res.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*resBundle.Id).To(Equal(res.ID), "found object does not match test object")
	Expect(*resBundle.Name).To(Equal(res.ID))
	Expect(*resBundle.Kind).To(Equal("ResourceBundle"))
	Expect(*resBundle.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resource-bundles/%s", res.ID)))
	Expect(*resBundle.Version).To(Equal(int32(2)))

	// list search resource bundle with restful API
	search := fmt.Sprintf("consumer_name = '%s'", consumer.Name)
	list, _, err := client.DefaultApi.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
	Expect(list.Kind).To(Equal("ResourceBundleList"))
	Expect(err).NotTo(HaveOccurred(), "Error getting resource bundle list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
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

func contains(et api.EventType, events api.EventList) bool {
	for _, e := range events {
		if e.EventType == et {
			return true
		}
	}
	return false
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
