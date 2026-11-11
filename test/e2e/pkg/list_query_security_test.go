package e2e_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

var _ = Describe("List query security validation", Label("e2e-tests-list-security"), func() {
	It("accepts valid search, ordering, and JSONB label queries", func() {
		suffix := rand.String(6)
		consumerPrefix := "list-query-" + suffix
		consumerLabels := map[string]string{"list-query": suffix}
		consumerNames := []string{consumerPrefix + "-a", consumerPrefix + "-b"}
		consumerIDs := make([]string, 0, len(consumerNames))
		DeferCleanup(func() {
			for _, id := range consumerIDs {
				resp, err := apiClient.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, id).Execute()
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			}
		})
		for _, name := range consumerNames {
			created, resp, err := apiClient.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).
				Consumer(openapi.Consumer{Name: openapi.PtrString(name), Labels: &consumerLabels}).Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			consumerIDs = append(consumerIDs, *created.Id)
		}
		consumerList, resp, err := apiClient.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).
			Search(fmt.Sprintf("name like '%s%%'", consumerPrefix)).OrderBy("name desc").Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(consumerList.Items).To(HaveLen(2))
		Expect(*consumerList.Items[0].Name).To(Equal(consumerNames[1]))
		Expect(*consumerList.Items[1].Name).To(Equal(consumerNames[0]))

		consumerList, resp, err = apiClient.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).
			Search(fmt.Sprintf("labels ->> 'list-query' = '%s'", suffix)).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(consumerList.Items).To(HaveLen(2))

		workPrefix := "list-query-" + suffix
		workNames := []string{workPrefix + "-a", workPrefix + "-b"}
		workClient := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName)
		createdWorkNames := make([]string, 0, len(workNames))
		DeferCleanup(func() {
			for _, name := range createdWorkNames {
				Expect(workClient.Delete(ctx, name, metav1.DeleteOptions{})).To(Succeed())
				Eventually(func() error { return AssertWorkNotFound(name) }).Should(Succeed())
			}
		})
		for _, name := range workNames {
			labels := map[string]string{"list-query": suffix}
			_, err := workClient.Create(ctx, NewManifestWorkWithLabels(name, labels), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			createdWorkNames = append(createdWorkNames, name)
		}
		var resourceNames []string
		Eventually(func() error {
			resourceList, response, listErr := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).
				Search(fmt.Sprintf("name like '%s%%'", workPrefix)).OrderBy("name desc").Execute()
			if listErr != nil {
				return listErr
			}
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected resource list status: %d", response.StatusCode)
			}
			if len(resourceList.Items) != 2 {
				return fmt.Errorf("expected two matching resources, got %d", len(resourceList.Items))
			}
			resourceNames = []string{*resourceList.Items[0].Name, *resourceList.Items[1].Name}
			if resourceNames[0] != workNames[1] || resourceNames[1] != workNames[0] {
				return fmt.Errorf("unexpected descending resource order: %v", resourceNames)
			}
			return nil
		}).WithTimeout(2 * time.Minute).Should(Succeed())

		resourceList, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).
			Search(fmt.Sprintf(`payload -> 'metadata' -> 'labels' @> '{"list-query":"%s"}'`, suffix)).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resourceList.Items).To(HaveLen(2))
	})

	It("rejects SQL-shaped search and ordering input on both list endpoints", func() {
		// These inputs exercise identifier/expression boundaries only. They are
		// deliberately non-destructive. Handler-level recording-DAO tests separately
		// verify that invalid requests are rejected before DAO query calls.
		attackCases := []struct {
			name      string
			search    string
			orderBy   string
			consumers bool
		}{
			{name: "property search key", search: "properties.foo = 'bar'"},
			{name: "quoted identifier", search: `"name' OR 1=1 --" = 'x'`},
			{name: "unknown JSONB search column", search: `unknown_column -> 'k' = 'v'`},
			{name: "function in order by", orderBy: "lower(name)"},
			{name: "comment-separated order by", orderBy: "name/**/desc"},
			{name: "property search key on consumers", search: "properties.foo = 'bar'", consumers: true},
			{name: "quoted identifier on consumers", search: `"name' OR 1=1 --" = 'x'`, consumers: true},
			{name: "unknown JSONB search column on consumers", search: `unknown_column -> 'k' = 'v'`, consumers: true},
			{name: "function in consumer order by", orderBy: "lower(name)", consumers: true},
			{name: "comment-separated consumer order by", orderBy: "name/**/desc", consumers: true},
		}

		for _, tc := range attackCases {
			By("request " + tc.name)
			var resp *http.Response
			var err error
			if tc.consumers {
				request := apiClient.DefaultAPI.ApiMaestroV1ConsumersGet(ctx)
				if tc.search != "" {
					request = request.Search(tc.search)
				}
				if tc.orderBy != "" {
					request = request.OrderBy(tc.orderBy)
				}
				_, resp, err = request.Execute()
			} else {
				request := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx)
				if tc.search != "" {
					request = request.Search(tc.search)
				}
				if tc.orderBy != "" {
					request = request.OrderBy(tc.orderBy)
				}
				_, resp, err = request.Execute()
			}

			Expect(err).To(HaveOccurred(), tc.name)
			Expect(resp).NotTo(BeNil(), tc.name)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), tc.name)
		}

		// Ensure both handlers remain usable after rejecting invalid requests.
		_, resp, err := apiClient.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		_, resp, err = apiClient.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
