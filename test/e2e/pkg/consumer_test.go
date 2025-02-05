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
		resource := helper.NewAPIResource(*consumerB.Name, fmt.Sprintf("nginx-%s", rand.String(5)), 1)

		AfterAll(func() {
			// delete the consumer
			resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumerA.Id).Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			_, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerA.Id).Execute()
			Expect(err.Error()).To(ContainSubstring("Not Found"))
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			// delete the consumer associated with resource
			resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumerB.Id).Execute()
			Expect(err).To(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden)) // 403 forbid deletion

			// delete the resource on the consumer
			resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, *resource.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			// only if permanently delete the resource, the consumer can be deleted
			resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersIdDelete(ctx, *consumerB.Id).Execute()
			Expect(err).To(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden)) // 403 forbid deletion
		})

		It("create consumer", func() {
			// create a consumer without resource
			created, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumerA).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			consumerA = *created

			got, resp, err := apiClient.DefaultApi.ApiMaestroV1ConsumersIdGet(ctx, *consumerA.Id).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(got).NotTo(BeNil())

			// create a consumer with resource
			created, resp, err = apiClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).Consumer(consumerB).Execute()
			Expect(err).To(Succeed())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(*created.Id).NotTo(BeEmpty())
			consumerB = *created

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
			fmt.Printf("consumer list: %v\n", consumerList.Items)

			got := false
			for _, c := range consumerList.Items {
				if *c.Name == *consumerA.Name {
					got = true
				}
			}
			Expect(got).To(BeTrue())
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
