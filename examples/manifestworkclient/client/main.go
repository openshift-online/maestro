package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"fmt"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

var (
	sourceID            = flag.String("source", "grpc", "The source for manifestwork client")
	maestroServerAddr   = flag.String("maestro-server", "https://127.0.0.1:8000", "The maestro server address")
	grpcServerAddr      = flag.String("grpc-server", "127.0.0.1:8090", "The grpc server address")
	grpcServerCAFile    = flag.String("grpc-server-ca-file", "", "The CA for grpc server")
	grpcClientCertFile  = flag.String("grpc-client-cert-file", "", "The client certificate to access grpc server")
	grpcClientKeyFile   = flag.String("grpc-client-key-file", "", "The client key to access grpc server")
	grpcClientTokenFile = flag.String("grpc-client-token-file", "", "The client token to access grpc server")
	consumerName        = flag.String("consumer-name", "", "The Consumer Name")
	manifestworkFile    = flag.String("manifestwork_file", "", "The absolute file path containing the manifestwork json file")
	action              = flag.String("action", "create", "The action executed on the manifestwork, create or delete")
)

func main() {
	flag.Parse()

	if len(*consumerName) == 0 {
		log.Fatalf("the consumer_name is required")
	}

	if len(*manifestworkFile) == 0 {
		log.Fatalf("the manifestwork_file is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	grpcOptions := grpc.NewGRPCOptions()
	grpcOptions.URL = *grpcServerAddr
	if *grpcServerCAFile != "" {
		grpcOptions.CAFile = *grpcServerCAFile
	}
	if *grpcClientCertFile != "" {
		grpcOptions.ClientCertFile = *grpcClientCertFile
	}
	if *grpcClientKeyFile != "" {
		grpcOptions.ClientKeyFile = *grpcClientKeyFile
	}
	if *grpcClientTokenFile != "" {
		grpcOptions.TokenFile = *grpcClientTokenFile
	}

	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		maestroAPIClient,
		grpcOptions,
		*sourceID,
	)
	if err != nil {
		log.Fatal(err)
	}

	workJSON, err := os.ReadFile(*manifestworkFile)
	if err != nil {
		log.Fatalf("failed to read manifestwork file: %v", err)
	}

	manifestwork := &workv1.ManifestWork{}
	if err := json.Unmarshal(workJSON, manifestwork); err != nil {
		log.Fatalf("failed to unmarshal manifestwork: %v", err)
	}

	// use workClient to create/get/patch/delete work
	if *action == "create" {
		_, err = workClient.ManifestWorks(*consumerName).Create(ctx, manifestwork, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}

		<-time.After(2 * time.Second)
		work, err := workClient.ManifestWorks(*consumerName).Get(ctx, manifestwork.Name, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("the work %s/%s (uid=%s) is created", *consumerName, manifestwork.Name, work.UID)
	}

	// newWork := work.DeepCopy()
	// newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(manifestwork.Name)}
	// patchData, err := grpcsource.ToWorkPatch(work, newWork)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// _, err = workClient.ManifestWorks(*consumerName).Patch(ctx, manifestwork.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Printf("the work %s/%s (uid=%s) is updated\n", *consumerName, manifestwork.Name, work.UID)

	// <-time.After(5 * time.Second)

	if *action == "delete" {
		err = workClient.ManifestWorks(*consumerName).Delete(ctx, manifestwork.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("the work %s/%s (uid=%s) is deleted\n", *consumerName, manifestwork.Name, manifestwork.UID)
	}
}
