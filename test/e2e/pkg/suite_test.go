package e2e_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

	"google.golang.org/grpc"
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
	grpcCertDir       string
	kubeWorkClient    workclientset.Interface
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

	grpcCertDir, err := os.MkdirTemp("/tmp", "maestro-grpc-certs-")
	Expect(err).To(Succeed())

	// validate the consumer kubeconfig and name
	restConfig, err := clientcmd.BuildConfigFromFlags("", consumer.KubeConfig)
	Expect(err).To(Succeed())
	consumer.ClientSet, err = kubernetes.NewForConfig(restConfig)
	Expect(err).To(Succeed())
	kubeWorkClient, err = workclientset.NewForConfig(restConfig)
	Expect(err).To(Succeed())
	Expect(consumer.Name).NotTo(BeEmpty(), "consumer name is not provided")

	grpcCertSrt, err := consumer.ClientSet.CoreV1().Secrets("maestro").Get(ctx, "maestro-grpc-cert", metav1.GetOptions{})
	Expect(err).To(Succeed())
	grpcServerCAFile := fmt.Sprintf("%s/ca.crt", grpcCertDir)
	grpcClientCert := fmt.Sprintf("%s/client.crt", grpcCertDir)
	grpcClientKey := fmt.Sprintf("%s/client.key", grpcCertDir)
	Expect(os.WriteFile(grpcServerCAFile, grpcCertSrt.Data["ca.crt"], 0644)).To(Succeed())
	Expect(os.WriteFile(grpcClientCert, grpcCertSrt.Data["client.crt"], 0644)).To(Succeed())
	Expect(os.WriteFile(grpcClientKey, grpcCertSrt.Data["client.key"], 0644)).To(Succeed())

	grpcClientTokenSrt, err := consumer.ClientSet.CoreV1().Secrets("maestro").Get(ctx, "grpc-client-token", metav1.GetOptions{})
	Expect(err).To(Succeed())
	grpcClientTokenFile := fmt.Sprintf("%s/token", grpcCertDir)
	Expect(os.WriteFile(grpcClientTokenFile, grpcClientTokenSrt.Data["token"], 0644)).To(Succeed())

	grpcOptions = grpcoptions.NewGRPCOptions()
	grpcOptions.URL = grpcServerAddress
	grpcOptions.CAFile = grpcServerCAFile
	grpcOptions.TokenFile = grpcClientTokenFile
	sourceID = "sourceclient-test" + rand.String(5)

	// create the clusterrole for grpc authz
	Expect(helper.CreateGRPCAuthRule(ctx, consumer.ClientSet, "grpc-pub-sub", "source", sourceID, []string{"pub", "sub"})).To(Succeed())
	grpcConn, err = helper.CreateGRPCConn(grpcServerAddress, grpcServerCAFile, grpcClientTokenFile)
	Expect(err).To(Succeed())
	grpcClient = pbv1.NewCloudEventServiceClient(grpcConn)

	workClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		apiClient,
		grpcOptions,
		sourceID,
	)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	// dump debug info
	dumpDebugInfo()
	grpcConn.Close()
	os.RemoveAll(grpcCertDir)
	cancel()
})

type ConsumerOptions struct {
	Name       string
	KubeConfig string
	ClientSet  *kubernetes.Clientset
}

func dumpDebugInfo() {
	// dump the maestro server logs
	dumpPodLogs(ctx, consumer.ClientSet, "app=maestro", "maestro")
	// dump the maestro agent ogs
	dumpPodLogs(ctx, consumer.ClientSet, "app=maestro-agent", "maestro-agent")
}

func dumpPodLogs(ctx context.Context, kubeClient kubernetes.Interface, podSelector, podNamespace string) error {
	// get pods from podSelector
	pods, err := kubeClient.CoreV1().Pods(podNamespace).List(ctx, metav1.ListOptions{LabelSelector: podSelector})
	if err != nil {
		return fmt.Errorf("failed to list pods with pod selector (%s): %v", podSelector, err)
	}

	for _, pod := range pods.Items {
		logReq := kubeClient.CoreV1().Pods(podNamespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		logs, err := logReq.Stream(context.Background())
		if err != nil {
			return fmt.Errorf("failed to open log stream: %v", err)
		}
		defer logs.Close()

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(logs)
		if err != nil {
			return fmt.Errorf("failed to read pod logs: %v", err)
		}

		log.Printf("=========================================== POD LOGS START ===========================================")
		log.Printf("Pod %s/%s phase: %s", pod.Name, podNamespace, string(pod.Status.Phase))
		for _, containerStatus := range pod.Status.ContainerStatuses {
			log.Printf("Container %s status: %v", containerStatus.Name, containerStatus.State)
		}
		log.Printf("Pod %s/%s logs: \n%s", pod.Name, podNamespace, buf.String())
		log.Printf("=========================================== POD LOGS STOP ===========================================")
	}

	return nil
}
