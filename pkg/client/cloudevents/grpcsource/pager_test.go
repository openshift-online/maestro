package grpcsource

import (
	"context"
	"testing"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPageList(t *testing.T) {
	getter := &mock.ResourceBundlesStore{}
	maestroServer := mock.NewMaestroMockServer(getter)
	maestroServer.Start()
	defer maestroServer.Stop()

	client := mock.NewMaestroAPIClient(maestroServer.URL())
	cases := []struct {
		name             string
		resourceBundles  []openapi.ResourceBundle
		listOpts         metav1.ListOptions
		expectedItemsLen int
		expectedNext     string
	}{
		{
			name:             "no items",
			resourceBundles:  resourceBundles(0),
			listOpts:         metav1.ListOptions{},
			expectedItemsLen: 0,
			expectedNext:     "",
		},
		{
			name:             "list all items (items < MaxListPageSize)",
			resourceBundles:  resourceBundles(200),
			listOpts:         metav1.ListOptions{},
			expectedItemsLen: 200,
			expectedNext:     "",
		},
		{
			name:             "list all items (items = MaxListPageSize)",
			resourceBundles:  resourceBundles(400),
			listOpts:         metav1.ListOptions{},
			expectedItemsLen: 400,
			expectedNext:     "",
		},
		{
			name:             "list all items (items > MaxListPageSize)",
			resourceBundles:  resourceBundles(429),
			listOpts:         metav1.ListOptions{},
			expectedItemsLen: 429,
			expectedNext:     "",
		},
		{
			name:            "list items (limit > total items)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit: 500,
			},
			expectedItemsLen: 429,
			expectedNext:     "",
		},
		{
			name:            "list items (limit < total items)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit: 400,
			},
			expectedItemsLen: 400,
			expectedNext:     "2",
		},
		{
			name:            "list items (limit < total items)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit: 40,
			},
			expectedItemsLen: 40,
			expectedNext:     "2",
		},
		{
			name:            "list items with continue (from last page - 1)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit:    100,
				Continue: "4",
			},
			expectedItemsLen: 100,
			expectedNext:     "5",
		},
		{
			name:            "list items with continue (from page last page)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit:    100,
				Continue: "5",
			},
			expectedItemsLen: 29,
			expectedNext:     "",
		},
		{
			name:            "list items with continue (from page last page + 1)",
			resourceBundles: resourceBundles(429),
			listOpts: metav1.ListOptions{
				Limit:    100,
				Continue: "6",
			},
			expectedItemsLen: 0,
			expectedNext:     "",
		},
		{
			name:            "list items with continue and max limit",
			resourceBundles: resourceBundles(1229),
			listOpts: metav1.ListOptions{
				Limit:    400,
				Continue: "3",
			},
			expectedItemsLen: 400,
			expectedNext:     "4",
		},
		{
			name:            "list items with continue and max limit",
			resourceBundles: resourceBundles(1229),
			listOpts: metav1.ListOptions{
				Limit:    400,
				Continue: "4",
			},
			expectedItemsLen: 29,
			expectedNext:     "",
		},
		{
			name:            "list items with continue and max limit",
			resourceBundles: resourceBundles(1229),
			listOpts: metav1.ListOptions{
				Limit:    400,
				Continue: "5",
			},
			expectedItemsLen: 0,
			expectedNext:     "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getter.Set(c.resourceBundles)

			list, next, err := pageList(context.Background(), client, "", c.listOpts)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}

			if len(list.Items) != c.expectedItemsLen {
				t.Errorf("expected items length %v, but got %v", c.expectedItemsLen, len(list.Items))
			}

			if next != c.expectedNext {
				t.Errorf("expected next %v, but got %v", c.expectedNext, next)
			}
		})

	}
}

func resourceBundles(total int) []openapi.ResourceBundle {
	items := []openapi.ResourceBundle{}
	for i := 0; i < total; i++ {
		items = append(items, openapi.ResourceBundle{})
	}
	return items
}
