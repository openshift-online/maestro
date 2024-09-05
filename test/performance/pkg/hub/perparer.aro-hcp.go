package hub

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"

	"github.com/openshift-online/maestro/test/performance/pkg/hub/store"
	"github.com/openshift-online/maestro/test/performance/pkg/hub/workloads"
	"github.com/openshift-online/maestro/test/performance/pkg/util"
)

const (
	sourceID                     = "maestro-performance-test"
	defaultMaestroServiceAddress = "http://maestro:8000"
	defaultMaestroGRPCAddress    = "maestro-grpc:8090"
)

type AROHCPPreparerOptions struct {
	MaestroServiceAddress string
	GRPCServiceAddress    string

	ClusterBeginIndex int
	ClusterCounts     int

	OnlyClusters bool

	WorkCounts int
}

func NewAROHCPPreparerOptions() *AROHCPPreparerOptions {
	return &AROHCPPreparerOptions{
		MaestroServiceAddress: defaultMaestroServiceAddress,
		GRPCServiceAddress:    defaultMaestroGRPCAddress,
		ClusterBeginIndex:     1,
		ClusterCounts:         1,
		WorkCounts:            1,
	}
}

func (o *AROHCPPreparerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MaestroServiceAddress, "maestro-service-address", o.MaestroServiceAddress, "Address of the Maestro API service")
	fs.StringVar(&o.GRPCServiceAddress, "grpc-service-address", o.GRPCServiceAddress, "Address of the Maestro GRPC service")
	fs.IntVar(&o.ClusterBeginIndex, "cluster-begin-index", o.ClusterBeginIndex, "Begin index of the clusters")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
	fs.BoolVar(&o.OnlyClusters, "only-clusters", o.OnlyClusters, "Only create clusters")
	fs.IntVar(&o.WorkCounts, "work-counts", o.WorkCounts, "Counts of the works")
}

func (o *AROHCPPreparerOptions) Run(ctx context.Context) error {
	if o.OnlyClusters {
		return o.PrepareClusters(ctx)
	}

	// initialize cluster with works
	if err := o.CreateWorks(ctx, "init"); err != nil {
		return err
	}

	return nil
}

func (o *AROHCPPreparerOptions) PrepareClusters(ctx context.Context) error {
	apiClient := util.NewMaestroAPIClient(o.MaestroServiceAddress)

	index := o.ClusterBeginIndex
	startTime := time.Now()
	for i := 0; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(index)

		startTime := time.Now()
		if err := util.CreateConsumer(ctx, apiClient, clusterName); err != nil {
			return err
		}

		klog.Infof("cluster %s is created, time=%dms", clusterName, util.UsedTime(startTime, time.Millisecond))

		index = index + 1
	}
	klog.Infof("Clusters (%d) are created, time=%dms", o.ClusterCounts, util.UsedTime(startTime, time.Millisecond))
	return nil
}

func (o *AROHCPPreparerOptions) CreateWorks(ctx context.Context, phase string) error {
	creator, err := work.NewClientHolderBuilder(&grpc.GRPCOptions{URL: o.GRPCServiceAddress}).
		WithClientID(fmt.Sprintf("%s-client", sourceID)).
		WithSourceID(sourceID).
		WithCodecs(codec.NewManifestBundleCodec()).
		WithWorkClientWatcherStore(store.NewCreateOnlyWatcherStore()).
		WithResyncEnabled(false).
		NewSourceClientHolder(ctx)
	if err != nil {
		return err
	}

	workClient := creator.WorkInterface()

	index := o.ClusterBeginIndex
	total := 0
	startTime := time.Now()
	for i := 0; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(index)

		for j := 0; j < o.WorkCounts; j++ {
			works, err := workloads.ToAROHCPManifestWorks(clusterName)
			if err != nil {
				return err
			}

			startTime := time.Now()
			for _, work := range works {
				startTime := time.Now()
				if _, err := workClient.WorkV1().ManifestWorks(clusterName).Create(
					ctx,
					work,
					metav1.CreateOptions{},
				); err != nil {
					return err
				}

				klog.Infof("the work %s/%s is created, time=%dms",
					work.Namespace, work.Name, util.UsedTime(startTime, time.Millisecond))
				total = total + 1
			}

			klog.Infof("the works are created for cluster %s, time=%dms",
				clusterName, util.UsedTime(startTime, time.Millisecond))
		}
		index = index + 1
	}

	klog.Infof("Works (%d) are created, time=%dms", total, util.UsedTime(startTime, time.Millisecond))

	return nil
}
