package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"time"

	"fmt"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

const sourceID = "mw-client-example"

var (
	maestroServerAddr = flag.String("maestro-server", "https://127.0.0.1:30080", "The Maestro server address")
	grpcServerAddr    = flag.String("grpc-server", "127.0.0.1:30090", "The GRPC server address")
	consumerName      = flag.String("consumer-name", "", "The Consumer Name")
)

func main() {
	flag.Parse()

	if len(*consumerName) == 0 {
		log.Fatalf("the consumer_name is required")
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

	// use workClient to create/get/patch/delete work
	workName := "work-" + rand.String(5)
	_, err = workClient.ManifestWorks(*consumerName).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
	if err != nil {
		log.Fatal(err)
	}

	<-time.After(5 * time.Second)

	work, err := workClient.ManifestWorks(*consumerName).Get(ctx, workName, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("the work %s/%s (uid=%s) is created\n", *consumerName, workName, work.UID)

	newWork := work.DeepCopy()
	newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
	patchData, err := grpcsource.ToWorkPatch(work, newWork)
	if err != nil {
		log.Fatal(err)
	}
	_, err = workClient.ManifestWorks(*consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("the work %s/%s (uid=%s) is updated\n", *consumerName, workName, work.UID)

	<-time.After(5 * time.Second)

	err = workClient.ManifestWorks(*consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("the work %s/%s (uid=%s) is deleted\n", *consumerName, workName, work.UID)
}

func NewManifestWork(name string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"work.label": "example",
			},
			Annotations: map[string]string{
				"work.annotations": "example",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					NewManifest(name),
				},
			},
		},
	}
}

func NewManifest(name string) workv1.Manifest {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      name,
			},
			"data": map[string]string{
				"test": rand.String(5),
			},
		},
	}
	objectStr, _ := obj.MarshalJSON()
	manifest := workv1.Manifest{}
	manifest.Raw = objectStr
	return manifest
}
