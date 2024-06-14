package e2e_test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test"
)

var (
	apiServerAddress  string
	grpcServerAddress string
	kubeconfig        string
	consumer_name     string
	kubeClient        *kubernetes.Clientset
	apiClient         *openapi.APIClient
	helper            *test.Helper
	T                 *testing.T
	workClient        *work.ClientHolder
	grpcOptions       *grpc.GRPCOptions
	cancel            context.CancelFunc
	ctx               context.Context
	sourceID          string
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	T = t
	RunSpecs(t, "End-to-End Test Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	flag.StringVar(&apiServerAddress, "api-server", "", "Maestro API server address")
	flag.StringVar(&grpcServerAddress, "grpc-server", "", "Maestro gRPC server address")
	flag.StringVar(&consumer_name, "consumer_name", "", "Consumer name is used to identify the consumer")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
}

var _ = BeforeSuite(func() {
	// initialize the help
	helper = &test.Helper{
		T: T,
	}
	// initialize the api client
	tlsConfig := &tls.Config{
		//nolint:gosec
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}

	cfg := &openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         apiServerAddress,
				Description: "current domain",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
	}
	apiClient = openapi.NewAPIClient(cfg)

	// validate the kubeconfig file
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(fmt.Sprintf("failed to build kubeconfig: %v", err))
	}
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to create kube client: %v", err))
	}

	// validate the consumer_id
	if consumer_name == "" {
		panic("consumer_id is not provided")
	}

	ctx, cancel = context.WithCancel(context.Background())

	sourceID = "sourceclient-test" + rand.String(5)
	grpcOptions = grpc.NewGRPCOptions()
	grpcOptions.URL = grpcServerAddress

	workClient, err = work.NewClientHolderBuilder(grpcOptions).
		WithClientID(fmt.Sprintf("%s-watcher", sourceID)).
		WithSourceID(sourceID).
		WithCodecs(codec.NewManifestBundleCodec()).
		WithWorkClientWatcherStore(grpcsource.NewRESTFullAPIWatcherStore(apiClient, sourceID)).
		WithResyncEnabled(false).
		NewSourceClientHolder(ctx)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
})
