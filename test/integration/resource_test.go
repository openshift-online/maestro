package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/services"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
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

	res := h.NewResource("cluster1")

	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, res.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*resource.Id).To(Equal(res.ID), "found object does not match test object")
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", res.ID)))
	Expect(*resource.CreatedAt).To(BeTemporally("~", res.CreatedAt))
	Expect(*resource.UpdatedAt).To(BeTemporally("~", res.UpdatedAt))
}

func TestResourcePost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500
	res := openapi.Resource{}

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
}

func TestResourcePatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500

	res := h.NewResource("cluster1")

	// 200 OK
	manifest := map[string]interface{}{"data": 1}
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(openapi.ResourcePatchRequest{Manifest: manifest}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*resource.Id).To(Equal(res.ID))
	Expect(*resource.CreatedAt).To(BeTemporally("~", res.CreatedAt))
	Expect(*resource.Kind).To(Equal("Resource"))
	Expect(*resource.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/resources/%s", *resource.Id)))

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
}

func contains(et api.EventType, events api.EventList) bool {
	for _, e := range events {
		if e.EventType == et {
			return true
		}
	}
	return false
}

func TestResourcePaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	_ = h.NewResourceList("cluster1", 20)

	list, _, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestResourceListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	resources := h.NewResourceList("cluster1", 20)

	search := fmt.Sprintf("id in ('%s')", resources[0].ID)
	list, _, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting resource list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(resources[0].ID))
}

func TestUpdateResourceWithRacingRequests(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	res := h.NewResource("cluster1")

	// starts 20 threads to update this resource at the same time
	threads := 20
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			manifests := map[string]interface{}{"data": 1}
			_, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(openapi.ResourcePatchRequest{Manifest: manifests}).Execute()
			Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		}()
	}

	// waits for all goroutines above to complete
	wg.Wait()

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
}

func TestUpdateResourceWithRacingRequests_WithoutLock(t *testing.T) {
	// we disable the advisory lock and try to update the resources
	services.DisableAdvisoryLock = true

	defer func() {
		services.DisableAdvisoryLock = false
	}()

	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	res := h.NewResource("cluster1")

	// starts 20 threads to update this resource at the same time
	threads := 20
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			manifests := map[string]interface{}{"data": 1}
			_, resp, err := client.DefaultApi.ApiMaestroV1ResourcesIdPatch(ctx, res.ID).ResourcePatchRequest(openapi.ResourcePatchRequest{Manifest: manifests}).Execute()
			Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		}()
	}

	// waits for all goroutines above to complete
	wg.Wait()

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := dao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)

	updatedCount := 0
	for _, e := range events {
		if e.SourceID == res.ID && e.EventType == api.UpdateEventType {
			updatedCount = updatedCount + 1
		}
	}

	// the resource patch request is not protected by the advisory lock, so there should be at least one update
	Expect(updatedCount >= 1).To(BeTrue())
}
