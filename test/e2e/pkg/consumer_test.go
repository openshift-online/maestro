package e2e_test

import (
	"fmt"
	"net/http"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Consumers", Ordered, Label("e2e-tests-consumers"), func() {
	Context("Consumer CRUD Tests", func() {
		consumerA := openapi.Consumer{Name: openapi.PtrString(fmt.Sprintf("consumer-a-%s", rand.String(5)))}
		consumerB := openapi.Consumer{Name: openapi.PtrString(fmt.Sprintf("consumer-b-%s", rand.String(5)))}

		AfterAll(func() {
			// delete the consumer
			resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumerA.Id).Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			_, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerA.Id).Execute()
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			// delete the consumer
			resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumerB.Id).Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			_, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerB.Id).Execute()
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("create consumer", func() {
			// create a consumer
			created, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumerA).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			consumerA = *created

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerA.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())

			// create a consumer
			created, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumerB).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			consumerB = *created

			got, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerB.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())
		})

		It("list consumers", func() {
			consumerList, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersGet(ctx).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(consumerList).NotTo(BeNil())
			Expect(len(consumerList.Items) >= 2).To(BeTrue())

			gotA, gotB := false, false
			for _, c := range consumerList.Items {
				if *c.Name == *consumerA.Name {
					gotA = true
				}
				if *c.Name == *consumerB.Name {
					gotB = true
				}
			}
			Expect(gotA).To(BeTrue())
			Expect(gotB).To(BeTrue())
		})

		It("patch consumer", func() {
			labels := &map[string]string{"hello": "world"}
			patched, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdPatch(ctx, *consumerA.Id).
				ConsumerPatchRequest(openapi.ConsumerPatchRequest{Labels: labels}).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, ok := patched.GetLabelsOk()
			Expect(ok).To(BeTrue())

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerA.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())
			eq := reflect.DeepEqual(*labels, *got.Labels)
			Expect(eq).To(BeTrue())
		})
	})
})
