package e2e_test

import (
	"net/http"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// go test -v ./test/e2e/pkg -args -api-server=$api_server -consumer-name=$consumer_name -consumer-kubeconfig=$consumer_kubeconfig -ginkgo.focus "Consumer"
var _ = Describe("Consumer", Ordered, func() {
	var consumer openapi.Consumer
	var resourceConsumer openapi.Consumer
	var resource openapi.Resource
	BeforeAll(func() {
		consumer = openapi.Consumer{Name: openapi.PtrString("linda")}
		resourceConsumer = openapi.Consumer{Name: openapi.PtrString("susan")}
		resource = helper.NewAPIResource(*resourceConsumer.Name, 1)
	})

	Context("Consumer CRUD Tests", func() {
		It("create consumer", func() {
			// create a consumer without resource
			created, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumer).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			consumer = *created

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())

			// create a consumer associates with resource
			created, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(resourceConsumer).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			resourceConsumer = *created

			res, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(resource).Execute()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*res.Id).ShouldNot(BeEmpty())
			Expect(*res.Version).To(Equal(int32(1)))
			resource = *res
		})

		It("list consumer", func() {
			consumerList, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(consumerList).NotTo(BeNil())
			Expect(len(consumerList.Items) > 0).To(BeTrue())

			got := false
			for _, c := range consumerList.Items {
				if *c.Name == *consumer.Name {
					got = true
				}
			}
			Expect(got).To(BeTrue())
		})

		It("patch consumer", func() {
			labels := &map[string]string{"hello": "world"}
			patched, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, *consumer.Id).
				ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: labels}).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, ok := patched.GetLabelsOk()
			Expect(ok).To(BeTrue())

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())
			eq := reflect.DeepEqual(*labels, *got.Labels)
			Expect(eq).To(BeTrue())
		})

		AfterAll(func() {
			// delete the consumer
			resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumer.Id).Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			_, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumer.Id).Execute()
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			// delete the consumer associated with resource
			resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *resourceConsumer.Id).Execute()
			Expect(err).To(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden)) // 403 forbid deletion

			// delete the resource
			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			// delete the associated consumer
			resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *resourceConsumer.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})
	})
})
