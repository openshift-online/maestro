package e2e_test

import (
	"context"
	"crypto/tls"
	"flag"
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
	grpcClient        pbv1.CloudEventServiceClient
	workClient        workv1client.WorkV1Interface
	apiClient         *openapi.APIClient
	sourceID          string
	grpcConn          *grpc.ClientConn
	grpcOptions       *grpcoptions.GRPCOptions
	consumer          *ConsumerOptions
	helper            *test.Helper
	cancel            context.CancelFunc
	ctx               context.Context
)

func TestE2E(t *testing.T) {
	helper = &test.Helper{T: t}
	RegisterFailHandler(Fail)
	RunSpecs(t, "End-to-End Test Suite")
}

func init() {
	consumer = &ConsumerOptions{}
	klog.SetOutput(GinkgoWriter)
	flag.StringVar(&apiServerAddress, "api-server", "", "Maestro API server address")
	flag.StringVar(&grpcServerAddress, "grpc-server", "", "Maestro gRPC server address")
	flag.StringVar(&consumer.Name, "consumer-name", "", "Consumer name is used to identify the consumer")
	flag.StringVar(&consumer.KubeConfig, "consumer-kubeconfig", "", "Path to kubeconfig file")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	// initialize the api client
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
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				//nolint:gosec
				InsecureSkipVerify: true,
			}},
			Timeout: 10 * time.Second,
		},
	}
	apiClient = openapi.NewAPIClient(cfg)

	var err error
	grpcConn, err = grpc.Dial(grpcServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("fail to dial grpc server: %v", err)
	}
	grpcClient = pbv1.NewCloudEventServiceClient(grpcConn)

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

	// validate the consumer kubeconfig and name
	restConfig, err := clientcmd.BuildConfigFromFlags("", consumer.KubeConfig)
	Expect(err).To(Succeed())
	consumer.ClientSet, err = kubernetes.NewForConfig(restConfig)
	Expect(err).To(Succeed())
	Expect(consumer.Name).NotTo(BeEmpty(), "consumer name is not provided")
})

var _ = AfterSuite(func() {
	grpcConn.Close()
	cancel()
})

type ConsumerOptions struct {
	Name       string
	KubeConfig string
	ClientSet  *kubernetes.Clientset
}
