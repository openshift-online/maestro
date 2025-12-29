package grpcsource

import (
	"context"
	"fmt"
	"strconv"

	"github.com/openshift-online/ocm-sdk-go/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	maestrologger "github.com/openshift-online/maestro/pkg/logger"
)

// MaxListPageSize is the maximum size of one page, default is 400.
// NOTE: This should be reset carefully, when increasing this value, both maestro server's memory limit
// and the page size of resources need to be considered, if a bigger value is used, it might lead to
// maestro server OOM.
var MaxListPageSize int32 = 400

// PageList assists client code in breaking large list queries into multiple smaller chunks of PageSize or smaller.
func PageList(ctx context.Context, logger logging.Logger, client *openapi.APIClient, search string, opts metav1.ListOptions) (*openapi.ResourceBundleList, string, error) {
	items := []openapi.ResourceBundle{}

	page, err := page(opts)
	if err != nil {
		return nil, "", err
	}

	operationID := maestrologger.GetOperationID(ctx)

	limit := opts.Limit
	if limit < 0 {
		return nil, "", fmt.Errorf("limit cannot be less than 0")
	}

	var total int32 = 0
	nextPage := ""
	pageSize := pageSize(int32(limit))
	offset := (page - 1) * pageSize
	for {
		logger.Debug(ctx, "list works with search=%s, page=%d, size=%d", search, page, pageSize)
		req := client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).
			Search(search).
			Page(page).
			Size(pageSize)

		if len(operationID) > 0 {
			req = req.XOperationID(operationID)
		}

		rbs, _, err := req.Execute()
		if err != nil {
			return nil, "", err
		}
		logger.Debug(ctx, "listed works total=%d, page=%d, size=%d", rbs.Total, rbs.Page, rbs.Size)

		items = append(items, rbs.Items...)
		total = rbs.Size + total
		page = page + 1

		if rbs.Size < pageSize {
			// reaches the last page, stop list
			break
		}

		if limit == 0 {
			// no limit, continue to list the rest of items
			continue
		}

		if total == int32(limit) {
			// reaches the limit, stop list
			if (total + offset) < rbs.Total {
				// the listed items reach the limit size, but there are still items left
				nextPage = fmt.Sprintf("%d", page)
			}

			break
		}
	}

	return &openapi.ResourceBundleList{Items: items}, nextPage, nil
}

func page(opts metav1.ListOptions) (int32, error) {
	if len(opts.Continue) == 0 {
		return 1, nil
	}

	page, err := strconv.Atoi(opts.Continue)
	if err != nil {
		return 0, fmt.Errorf("a page number is required, %v", err)
	}

	if page < 0 {
		return 0, fmt.Errorf("an invalid page number %d", page)
	}

	return int32(page), nil
}

func pageSize(limit int32) int32 {
	if limit > MaxListPageSize {
		return MaxListPageSize
	}

	if limit == 0 {
		return MaxListPageSize
	}

	return limit
}
