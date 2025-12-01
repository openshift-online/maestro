package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test/performance/pkg/util"
	"github.com/openshift-online/ocm-sdk-go/logging"
)

const sourceID = "maestro-performance-test"

var (
	maestroServerAddr = "http://127.0.0.1:8000"
	grpcServerAddr    = "127.0.0.1:8090"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	maestroAPIClient := util.NewMaestroAPIClient(maestroServerAddr)

	logger, err := logging.NewStdLoggerBuilder().Build()
	if err != nil {
		log.Fatal(err)
	}

	grpcOptions := &grpc.GRPCOptions{Dialer: &grpc.GRPCDialer{}}
	grpcOptions.Dialer.URL = grpcServerAddr

	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		logger,
		maestroAPIClient,
		grpcOptions,
		sourceID,
	)
	if err != nil {
		log.Fatal(err)
	}

	totalTime := 0
	for i := 1; i <= 10; i++ {
		startTime := time.Now()
		works, err := workClient.ManifestWorks(util.ClusterName(i)).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}

		usedTime := util.UsedTime(startTime, time.Millisecond)
		totalTime = totalTime + int(usedTime)
		fmt.Printf("lists works %d, time=%d\n", len(works.Items), usedTime)
	}
	fmt.Printf("avg_time=%d\n", totalTime/10)

	for i := 1; i <= 10; i++ {
		startTime := time.Now()
		works, err := workClient.ManifestWorks(util.ClusterName(i)).List(ctx, metav1.ListOptions{
			LabelSelector: "maestro.performance.test=mc",
		})
		if err != nil {
			log.Fatal(err)
		}

		usedTime := util.UsedTime(startTime, time.Millisecond)
		totalTime = totalTime + int(usedTime)
		fmt.Printf("lists works %d, time=%dms\n", len(works.Items), usedTime)
	}
	fmt.Printf("avg_time=%dms\n", totalTime/10)

	for i := 1; i <= 10; i++ {
		startTime := time.Now()
		works, err := workClient.ManifestWorks(util.ClusterName(i)).List(ctx, metav1.ListOptions{
			LabelSelector: "maestro.performance.test=hypershift",
		})
		if err != nil {
			log.Fatal(err)
		}

		usedTime := util.UsedTime(startTime, time.Millisecond)
		totalTime = totalTime + int(usedTime)
		fmt.Printf("lists works %d, time=%dms\n", len(works.Items), usedTime)
	}
	fmt.Printf("avg_time=%dms\n", totalTime/10)

	listWorks(ctx, workClient, getClusterNames(2))
	listWorks(ctx, workClient, getClusterNames(3))
	listWorks(ctx, workClient, getClusterNames(4))
	listWorks(ctx, workClient, getClusterNames(5))
	listWorks(ctx, workClient, getClusterNames(6))
	listWorks(ctx, workClient, getClusterNames(7))
	listWorks(ctx, workClient, getClusterNames(8))
	listWorks(ctx, workClient, getClusterNames(9))
	listWorks(ctx, workClient, getClusterNames(10))

}

func listWorks(ctx context.Context, workClient workv1client.WorkV1Interface, names string) {
	startTime := time.Now()
	works, err := workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("cluster.maestro.performance.test in (%s)", names),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("lists works %d, time=%dms\n", len(works.Items), util.UsedTime(startTime, time.Millisecond))
}

func getClusterNames(total int) string {
	names := []string{}
	for i := 1; i <= total; i++ {
		names = append(names, fmt.Sprintf("maestro-cluster-%d", i))
	}
	return strings.Join(names, ",")
}
