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
	var testConsumerName string
	var testConsumerID string
	BeforeAll(func() {
		testConsumerName = "test-consumer"
	})

	Context("Consumer CRUD Tests", func() {
		It("create consumer", func() {
			consumer := openapi.Consumer{Name: &testConsumerName}
			created, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumer).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			testConsumerID = *created.Id

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, testConsumerID).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())
		})

		It("list consumer", func() {
			consumerList, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(consumerList).NotTo(BeNil())
			Expect(len(consumerList.Items) > 0).To(BeTrue())

			got := false
			for _, c := range consumerList.Items {
				if *c.Name == testConsumerName {
					got = true
				}
			}
			Expect(got).To(BeTrue())
		})

		It("patch consumer", func() {
			labels := &map[string]string{"hello": "world"}
			patched, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, testConsumerID).
				ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: labels}).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, ok := patched.GetLabelsOk()
			Expect(ok).To(BeTrue())

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, testConsumerID).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())
			eq := reflect.DeepEqual(*labels, *got.Labels)
			Expect(eq).To(BeTrue())
		})

		AfterAll(func() {
			// TODO: add the consumer deletion
		})
	})
})
