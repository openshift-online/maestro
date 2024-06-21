package e2e_test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"

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
	grpcConn          *grpc.ClientConn
	grpcClient        pbv1.CloudEventServiceClient
	helper            *test.Helper
	T                 *testing.T
	workClient        workv1client.WorkV1Interface
	grpcOptions       *grpcoptions.GRPCOptions
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

	var err error
	grpcConn, err = grpc.Dial(grpcServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("fail to dial grpc server: %v", err)
	}
	grpcClient = pbv1.NewCloudEventServiceClient(grpcConn)

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
	grpcOptions = grpcoptions.NewGRPCOptions()
	grpcOptions.URL = grpcServerAddress

	workClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		apiClient,
		grpcOptions,
		sourceID,
	)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	grpcConn.Close()
	cancel()
})
