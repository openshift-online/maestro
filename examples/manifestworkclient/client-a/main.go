package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

const sourceID = "mw-client-example"

var (
	maestroServerAddr = flag.String("maestro-server", "https://127.0.0.1:30080", "The Maestro server address")
	grpcServerAddr    = flag.String("grpc-server", "127.0.0.1:30090", "The GRPC server address")
	consumerName      = flag.String("consumer-name", "", "The Consumer Name")
	printWorkDetails  = flag.Bool("print-work-details", false, "Print work details")
)

func main() {
	flag.Parse()

	if len(*consumerName) == 0 {
		log.Fatalf("the consumer_name is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
	}()

	maestroAPIClient := openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         *maestroServerAddr,
				Description: "current domain",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			}},
			Timeout: 10 * time.Second,
		},
	})

	grpcOptions := &grpc.GRPCOptions{Dialer: &grpc.GRPCDialer{}}
	grpcOptions.Dialer.URL = *grpcServerAddr

	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		maestroAPIClient,
		grpcOptions,
		sourceID,
	)
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := workClient.ManifestWorks(metav1.NamespaceAll).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		ch := watcher.ResultChan()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				switch event.Type {
				case watch.Modified:
					Print(event, *printWorkDetails)
				case watch.Deleted:
					Print(event, *printWorkDetails)
				}
			}
		}
	}()

	<-ctx.Done()
}

func Print(event watch.Event, printDetails bool) {
	work := event.Object.(*workv1.ManifestWork)
	fmt.Printf("watched work (uid=%s) is %s\n", work.UID, event.Type)

	if printDetails {
		workJson, err := json.MarshalIndent(work, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", string(workJson))
	}
}
