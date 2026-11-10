package e2e_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("List query security validation", Label("e2e-tests-list-security"), func() {
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
