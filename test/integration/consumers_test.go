package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/test"
)

func TestConsumerGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// 401 using no JWT token
	_, _, err := client.DefaultAPI.ApiMaestroV1ConsumersIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	// GET responses per openapi spec: 200 and 404,
	_, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	consumer, err := h.CreateConsumerWithLabels("cluster-"+rand.String(5), map[string]string{"foo": "bar"})
	Expect(err).NotTo(HaveOccurred())

	found, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, consumer.ID).Execute()
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
	consumer, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
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

	_, resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// POST a consumer without name

	// 201 Created
	consumer, resp, err = client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(openapi.Consumer{}).Execute()
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
	consumer, err := h.CreateConsumer("brontosaurus")
	Expect(err).NotTo(HaveOccurred())

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
	patched, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: &labels}).Execute()
	assert(patched, resp, err, openapi.PtrString("brontosaurus"), &labels)

	// no-op patch
	patched, resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{}).Execute()
	assert(patched, resp, err, openapi.PtrString("brontosaurus"), &labels)

	// delete labels
	patched, resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdPatch(ctx, consumer.ID).ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: &map[string]string{}}).Execute()
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
	consumer, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")

	// 200 Got
	got, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*got.Id).To(Equal(*consumer.Id))
	Expect(*got.Name).To(Equal(*consumer.Name))

	// 204 Deleted
	resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

	// 404 Not Found
	_, resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
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
	consumer, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(c).Execute()
	Expect(err).To(Succeed())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*consumer.Id).NotTo(BeEmpty())

	// attach resource to the consumer
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	res, err := h.CreateResource(uuid.NewString(), *consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())
	Expect(res.ID).ShouldNot(BeEmpty())

	// 403 forbid deletion
	resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

	// delete the resource
	err = h.DeleteResource(res.ID)
	Expect(err).NotTo(HaveOccurred())

	// still forbid deletion for the deleting resource
	resp, err = client.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
}

func TestConsumerDeleting(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// create 10 consumers
	consumerNum := 10
	consumerIdToName := map[string]string{}
	for i := 0; i < consumerNum; i++ {
		consumerName := "tom" + fmt.Sprint(i)
		consumer, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(openapi.Consumer{
			Name: openapi.PtrString(consumerName),
		}).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
		Expect(*consumer.Id).NotTo(BeEmpty())
		consumerIdToName[*consumer.Id] = consumerName
	}

	resourceNum := 10
	resourceCreatorNum := 10
	resourceChan := make(chan *Result, resourceCreatorNum*resourceNum*consumerNum)
	consumerChan := make(chan *Result, consumerNum)

	for id, name := range consumerIdToName {

		var wg sync.WaitGroup
		// 10 creator for each consumer
		for i := 0; i < resourceCreatorNum; i++ {
			wg.Add(1)
			// each creator create resources to the consumer constantly
			go func(name, id string) {
				defer wg.Done()
				for i := 0; i < resourceNum; i++ {
					deployName := fmt.Sprintf("nginx-%s", rand.String(5))
					res, err := h.CreateResource(uuid.NewString(), name, deployName, "default", 1)
					resourceChan <- &Result{
						resource:     res,
						consumerName: name,
						consumerId:   id,
						err:          err,
					}
				}
			}(name, id)
		}

		// delete the consumer when creating resources on it
		wg.Add(1)
		go func(name, id string) {
			defer wg.Done()
			resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, id).Execute()
			consumerChan <- &Result{
				consumerName: name,
				consumerId:   id,
				resp:         resp,
				err:          err,
			}
		}(name, id)

		wg.Wait()
	}

	// verify the deleting consumer:
	// 1. success -> no resources is associated with it
	// 2. failed -> resources are associated with it
	for i := 0; i < consumerNum; i++ {
		result := <-consumerChan

		consumerName := result.consumerName
		consumerStatusCode := result.resp.StatusCode
		consumerErr := result.err

		search := fmt.Sprintf("consumer_name = '%s'", consumerName)
		resourceList, resp, err := client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Search(search).Execute()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(err).To(Succeed())

		if consumerStatusCode == http.StatusNoContent {
			// no resource is assocaited with the consumer
			fmt.Println("consumer", consumerName, "deleted successfully!")
			Expect(resourceList.Items).To(BeEmpty())
		} else {
			// at least one resource on the consumer, the statusCode should be 403 or 500
			fmt.Printf("failed to delete consumer(%s), associated with resource(%d), statusCode: %d, err: %v\n", consumerName, len(resourceList.Items), consumerStatusCode, consumerErr)
			Expect(resourceList.Items).NotTo(BeEmpty(), resourceList.Items)
		}
	}
	close(consumerChan)

	// verify the creating resources:
	// 1. success: consumer exists
	// 2. failed: consumer is deleted
	for i := 0; i < consumerNum*resourceNum*resourceCreatorNum; i++ {
		result := <-resourceChan

		resource := result.resource
		resourceErr := result.err
		resourceConsumerId := result.consumerId
		resourceConsumerName := result.consumerName

		// get the consumer
		consumer, resp, err := client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, resourceConsumerId).Execute()

		if resourceErr != nil {
			fmt.Printf("failed to create resource on consumer(%s): %v\n", resourceConsumerName, result.err)
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		} else {
			Expect(resource).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(*consumer.Id).To(Equal(resourceConsumerId))
			Expect(*consumer.Name).To(Equal(resourceConsumerName))
		}
	}
	close(resourceChan)
}

func TestConsumerPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	// Paging
	_, err := h.CreateConsumerList(20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
	Expect(list.Kind).To(Equal("ConsumerList"))
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting consumer list: %v", err)
	Expect(list.Kind).To(Equal("ConsumerList"))
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

type Result struct {
	resource     *api.Resource
	consumerName string
	consumerId   string
	resp         *http.Response
	err          error
}
