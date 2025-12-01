package grpcsource

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"k8s.io/client-go/rest"

	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"

	sourceclient "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/source/client"
	sourcecodec "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/source/codec"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

func NewMaestroGRPCSourceWorkClient(
	ctx context.Context,
	logger logging.Logger,
	apiClient *openapi.APIClient,
	opts *grpc.GRPCOptions,
	sourceID string,
) (workv1client.WorkV1Interface, error) {
	if len(sourceID) == 0 {
		return nil, fmt.Errorf("source id is required")
	}

	options, err := generic.BuildCloudEventsSourceOptions(opts, fmt.Sprintf("%s-maestro", sourceID), sourceID)
	if err != nil {
		return nil, err
	}

	watcherStore := newRESTFulAPIWatcherStore(ctx, logger, apiClient, sourceID)

	cloudEventsClient, err := generic.NewCloudEventSourceClient(
		ctx,
		options,
		nil, // resync is disabled, so lister is not required
		nil, // resync is disabled, so status hash is not required
		sourcecodec.NewManifestBundleCodec(),
	)
	if err != nil {
		return nil, err
	}

	cloudEventsClient.Subscribe(ctx, watcherStore.HandleReceivedResource)

	// start a go routine to receive client reconnect signal
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-cloudEventsClient.ReconnectedChan():
				// reconnect happened, sync the works for current watchers
				logger.Info(ctx, "client (source=%s) is reconnected, sync the works for current watchers", sourceID)
				if err := watcherStore.Sync(); err != nil {
					logger.Error(ctx, "failed to sync the works %v", err)
				}
			}
		}
	}()

	manifestWorkClient := sourceclient.NewManifestWorkSourceClient(sourceID, watcherStore, cloudEventsClient)
	return &WorkV1ClientWrapper{ManifestWorkClient: manifestWorkClient}, nil

}

// WorkV1ClientWrapper wraps a ManifestWork client to a WorkV1Interface
type WorkV1ClientWrapper struct {
	ManifestWorkClient *sourceclient.ManifestWorkSourceClient
}

var _ workv1client.WorkV1Interface = &WorkV1ClientWrapper{}

func (c *WorkV1ClientWrapper) ManifestWorks(namespace string) workv1client.ManifestWorkInterface {
	c.ManifestWorkClient.SetNamespace(namespace)
	return c.ManifestWorkClient
}

func (c *WorkV1ClientWrapper) AppliedManifestWorks() workv1client.AppliedManifestWorkInterface {
	return nil
}

func (c *WorkV1ClientWrapper) RESTClient() rest.Interface {
	return nil
}
