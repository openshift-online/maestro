package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/test"
)

func TestConsumerGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer := h.CreateConsumerWithLabels("cluster1", map[string]string{"foo": "bar"})

	found, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, consumer.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*found.Id).To(Equal(consumer.ID), "found object does not match test object")
	Expect(*found.Name).To(Equal(consumer.Name))
	Expect(*found.Labels).To(Equal(*consumer.Labels.ToMap()))
	Expect(*found.Kind).To(Equal("Consumer"))
	Expect(*found.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", consumer.ID)))
	Expect(*found.CreatedAt).To(BeTemporally("~", consumer.CreatedAt))
	Expect(*found.UpdatedAt).To(BeTemporally("~", consumer.UpdatedAt))
}

func TestConsumerPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500
	c := openapi.Consumer{
		Name: openapi.PtrString("foobar"),
		Labels: &map[string]string{
			"foo": "bar",
		},
	}

	// 201 Created
	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*consumer.Kind).To(Equal("Consumer"))
	Expect(*consumer.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", *consumer.Id)))
	Expect(*consumer.Name).To(Equal(*c.Name))
	Expect(*consumer.Labels).To(Equal(*c.Labels))

	// 400 bad request. posting junk json is one way to trigger 400.
	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/consumers"))
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	_, resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// POST a consumer without name

	// 201 Created
	consumer, resp, err = client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(openapi.Consumer{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*consumer.Kind).To(Equal("Consumer"))
	Expect(*consumer.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", *consumer.Id)))
	Expect(*consumer.Name).To(Equal(*consumer.Id), "the name and id are not same")
}

func TestConsumerPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// create a consumer
	consumer := h.CreateConsumer("brontosaurus")

	assert := func(patched *openapi.Consumer, resp *http.Response, err error, name *string, labels *map[string]string) {
		Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(*patched.Id).To(Equal(consumer.ID))
		Expect(*patched.CreatedAt).To(BeTemporally("~", consumer.CreatedAt))
		Expect(*patched.Kind).To(Equal("Consumer"))
		Expect(*patched.Href).To(Equal(fmt.Sprintf("/api/maestro/v1/consumers/%s", consumer.ID)))
		Expect(patched.Name).To(Equal(name))
		Expect(patched.Labels).To(Equal(labels))
	}

	// add labels
	labels := map[string]string{"foo": "bar"}
	patched, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: &labels}).Execute()
	assert(patched, resp, err, openapi.PtrString("brontosaurus"), &labels)

	// no-op patch
	patched, resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{}).Execute()
	assert(patched, resp, err, openapi.PtrString("brontosaurus"), &labels)

	// delete labels
	patched, resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: &map[string]string{}}).Execute()
	assert(patched, resp, err, openapi.PtrString("brontosaurus"), nil)

	// 500 server error. posting junk json is one way to trigger 500.
	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, _ := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/consumers/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestConsumerDelete(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// POST responses per openapi spec: 201, 409, 500
	c := openapi.Consumer{
		Name: openapi.PtrString("bazqux"),
	}

	// 201 Created
	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")

	// 200 Got
	got, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*got.Id).To(Equal(*consumer.Id))
	Expect(*got.Name).To(Equal(*consumer.Name))

	// 204 Deleted
	resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

	// 404 Not Found
	_, resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err.Error()).To(ContainSubstring("Not Found"))
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
}

func TestConsumerDeleteForbidden(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// create a consumser
	c := openapi.Consumer{
		Name: openapi.PtrString("jamie"),
	}
	consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).To(Succeed())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty())

	// attach resource to the consumer
	res := h.NewAPIResource(*consumer.Name, 1)
	resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
	Expect(err).To(Succeed())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*resource.Id).ShouldNot(BeEmpty())
	Expect(*resource.Version).To(Equal(int32(1)))

	// 403 forbid deletion
	resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

	// delete the resource
	resp, err = client.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
	Expect(err).To(Succeed())
	Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

	// still forbid deletion for the deleting resource
	resp, err = client.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
}

// TestConsumerDeleting is to test creating resources when delete the consumer
func TestConsumerDeleting(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// create 20 consumers
	consumerNum := 20
	consumerIdToName := map[string]string{}
	for i := 0; i < consumerNum; i++ {
		consumerName := "tom" + fmt.Sprint(i)
		consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(openapi.Consumer{
			Name: openapi.PtrString(consumerName),
		}).Execute()
		Expect(err).To(Succeed())
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
		Expect(*consumer.Id).NotTo(BeEmpty())
		consumerIdToName[*consumer.Id] = consumerName
	}

	// asynchronously attaching 10 resources to each consumer
	resourceNum := 10
	var wg sync.WaitGroup
	resourceChan := make(chan *Result, resourceNum*consumerNum)
	for id, name := range consumerIdToName {
		wg.Add(1)
		go func(name, id string) {
			defer wg.Done()
			for i := 0; i < resourceNum; i++ {
				// attach resource to the consumer
				res := h.NewAPIResource(name, 1)
				resource, resp, err := client.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
				resourceChan <- &Result{
					resource:     resource,
					resp:         resp,
					consumerName: name,
					consumerId:   id,
					err:          err,
				}
			}
		}(name, id)
	}

	// delete consumer when creating resources on it
	consumerChan := make(chan *Result, consumerNum)
	for id, name := range consumerIdToName {
		wg.Add(1)
		go func(name, id string) {
			defer wg.Done()
			resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, id).Execute()
			consumerChan <- &Result{
				consumerName: name,
				consumerId:   id,
				resp:         resp,
				err:          err,
			}
		}(name, id)
	}

	wg.Wait()

	// verify the deleted consumer:
	// 1. success -> no resources is associated with it
	// 2. failed -> resources are associated with it
	for i := 0; i < consumerNum; i++ {
		result := <-consumerChan

		consumerName := result.consumerName
		consumerStatusCode := result.resp.StatusCode

		search := fmt.Sprintf("consumer_name = '%s'", consumerName)
		resourceList, resp, err := client.DefaultApi.ApiMaestroV1ResourcesGet(ctx).Search(search).Execute()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(err).To(Succeed())

		if consumerStatusCode == http.StatusForbidden {
			// at least one resource on the consumer
			fmt.Println("consumer", consumerName, "associated resources", len(resourceList.Items))
			Expect(resourceList.Items).NotTo(BeEmpty(), resourceList.Items)
		} else if consumerStatusCode == http.StatusNoContent {
			// no resource is assocaited with the consumer
			fmt.Println("consumer", consumerName, "deleted successfully!")
			Expect(resourceList.Items).To(BeEmpty())
		} else {
			fmt.Println("unexpected consumer statusCode", consumerStatusCode)
		}
	}
	close(consumerChan)

	// verify the resources:
	// 1. success: consumer is exist
	// 2. failed: consumer is deleted
	for i := 0; i < consumerNum*resourceNum; i++ {
		result := <-resourceChan

		resourceStatusCode := result.resp.StatusCode
		resourceConsumerId := result.consumerId
		resourceConsumerName := result.consumerName

		// get the consumer
		consumer, resp, err := client.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, resourceConsumerId).Execute()

		if resourceStatusCode == http.StatusCreated {
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(*consumer.Id).To(Equal(resourceConsumerId))
			Expect(*consumer.Name).To(Equal(resourceConsumerName))
		} else {
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		}
	}
	close(resourceChan)
}

func TestConsumerPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	_ = h.CreateConsumerList(20)

	list, _, err := client.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
	Expect(list.Kind).To(Equal("ConsumerList"))
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
	Expect(list.Kind).To(Equal("ConsumerList"))
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

type Result struct {
	resource     *openapi.Resource
	consumerName string
	consumerId   string
	resp         *http.Response
	err          error
}
