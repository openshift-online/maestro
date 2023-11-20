package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/services"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/test"
)

func TestDinosaurGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultApi.ApiMaestroV1DinosaursIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultApi.ApiMaestroV1DinosaursIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	dino := h.NewDinosaur(h.NewID())

	dinosaur, resp, err := client.DefaultApi.ApiMaestroV1DinosaursIdGet(ctx, dino.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*dinosaur.Id).To(Equal(dino.ID), "found object does not match test object")
	Expect(*dinosaur.Species).To(Equal(dino.Species), "species mismatch")
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/dinosaurs/%s", dino.ID)))
	Expect(*dinosaur.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
	Expect(*dinosaur.UpdatedAt).To(BeTemporally("~", dino.UpdatedAt))
}

func TestDinosaurPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500
	dino := openapi.Dinosaur{
		Species: openapi.PtrString(time.Now().String()),
	}

	// 201 Created
	dinosaur, resp, err := client.DefaultApi.ApiMaestroV1DinosaursPost(ctx).Dinosaur(dino).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*dinosaur.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*dinosaur.Species).To(Equal(*dino.Species), "species mismatch")
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/dinosaurs/%s", *dinosaur.Id)))

	// 400 bad request. posting junk json is one way to trigger 400.
	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/dinosaurs"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestDinosaurPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500

	dino := h.NewDinosaur("Brontosaurus")

	// 200 OK
	species := "Dodo"
	dinosaur, resp, err := client.DefaultApi.ApiMaestroV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &species}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*dinosaur.Id).To(Equal(dino.ID))
	Expect(*dinosaur.Species).To(Equal(species), "species mismatch")
	Expect(*dinosaur.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/dinosaurs/%s", *dinosaur.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	// 500 server error. posting junk json is one way to trigger 500.
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/dinosaurs/foo"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	// species can not be empty in request body
	restyResp, err = resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{"species":""}`).
		Patch(h.RestURL(fmt.Sprintf("/dinosaurs/%s", *dinosaur.Id)))
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
	Expect(restyResp.String()).To(ContainSubstring("species cannot be empty"))

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

func TestDinosaurPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	_ = h.NewDinosaurList("Bronto", 20)

	list, _, err := client.DefaultApi.ApiMaestroV1DinosaursGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultApi.ApiMaestroV1DinosaursGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestDinosaurListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dinosaurs := h.NewDinosaurList("bronto", 20)

	search := fmt.Sprintf("id in ('%s')", dinosaurs[0].ID)
	list, _, err := client.DefaultApi.ApiMaestroV1DinosaursGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(dinosaurs[0].ID))
}

func TestUpdateDinosaurWithRacingRequests(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dino := h.NewDinosaur("Stegosaurus")

	// starts 20 threads to update this dinosaur at the same time
	threads := 20
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			species := "Pterosaur"
			updated, resp, err := client.DefaultApi.ApiMaestroV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &species}).Execute()
			Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*updated.Species).To(Equal(species), "species mismatch")
		}()
	}

	// waits for all goroutines above to complete
	wg.Wait()

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := dao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)

	updatedCount := 0
	for _, e := range events {
		if e.SourceID == dino.ID && e.EventType == api.UpdateEventType {
			updatedCount = updatedCount + 1
		}
	}

	// the dinosaur patch request is protected by the advisory lock, so there should only be one update
	Expect(updatedCount).To(Equal(1))
}

func TestUpdateDinosaurWithRacingRequests_WithoutLock(t *testing.T) {
	// we disable the advisory lock and try to update the dinosaurs
	services.DisableAdvisoryLock = true

	defer func() {
		services.DisableAdvisoryLock = false
	}()

	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dino := h.NewDinosaur("Tyrannosaurus")

	// starts 20 threads to update this dinosaur at the same time
	threads := 20
	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			species := "Triceratops"
			updated, resp, err := client.DefaultApi.ApiMaestroV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &species}).Execute()
			Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*updated.Species).To(Equal(species), "species mismatch")
		}()
	}

	// waits for all goroutines above to complete
	wg.Wait()

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := dao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)

	updatedCount := 0
	for _, e := range events {
		if e.SourceID == dino.ID && e.EventType == api.UpdateEventType {
			updatedCount = updatedCount + 1
		}
	}

	// the dinosaur patch request is not protected by the advisory lock, so there should be at least one update
	Expect(updatedCount >= 1).To(BeTrue())
}
