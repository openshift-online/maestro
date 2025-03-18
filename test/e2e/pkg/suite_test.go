package e2e_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/cert"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

	"google.golang.org/grpc"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/test"
	"github.com/openshift-online/maestro/test/e2e/pkg/reporter"
)

var log = logger.GetLogger()

type agentTestOptions struct {
	agentNamespace string
	consumerName   string
	kubeConfig     string
	kubeClientSet  kubernetes.Interface
	workClientSet  workclientset.Interface
}

type serverTestOptions struct {
	serverNamespace string
	kubeConfig      string
	kubeClientSet   kubernetes.Interface
}

var (
	serverTestOpts    *serverTestOptions
	agentTestOpts     *agentTestOptions
	apiServerAddress  string
	apiClient         *openapi.APIClient
	grpcServerAddress string
	grpcCertDir       string
	grpcConn          *grpc.ClientConn
	grpcClient        pbv1.CloudEventServiceClient
	grpcOptions       *grpcoptions.GRPCOptions
	sourceID          string
	sourceWorkClient  workv1client.WorkV1Interface
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
	serverTestOpts = &serverTestOptions{}
	agentTestOpts = &agentTestOptions{}
	flag.StringVar(&apiServerAddress, "api-server", "", "Maestro Restful API server address")
	flag.StringVar(&grpcServerAddress, "grpc-server", "", "Maestro gRPC server address")
	flag.StringVar(&serverTestOpts.serverNamespace, "server-namespace", "maestro", "Namespace where the maestro server is running")
	flag.StringVar(&serverTestOpts.kubeConfig, "server-kubeconfig", "", "Path to the kubeconfig file for the maestro server")
	flag.StringVar(&agentTestOpts.agentNamespace, "agent-namespace", "maestro-agent", "Namespace where the maestro agent is running")
	flag.StringVar(&agentTestOpts.consumerName, "consumer-name", "", "Consumer name is used to identify the consumer")
	flag.StringVar(&agentTestOpts.kubeConfig, "agent-kubeconfig", "", "Path to the kubeconfig file for the maestro agent")
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
	grpcCertDir, err = os.MkdirTemp("/tmp", "maestro-grpc-certs-")
	Expect(err).To(Succeed())

	// validate the server kubeconfig and initialize the kube client
	serverRestConfig, err := clientcmd.BuildConfigFromFlags("", serverTestOpts.kubeConfig)
	Expect(err).To(Succeed())
	serverTestOpts.kubeClientSet, err = kubernetes.NewForConfig(serverRestConfig)
	Expect(err).To(Succeed())

	// validate the agent consumer name && kubeconfig and initialize the kube client & work client
	Expect(agentTestOpts.consumerName).NotTo(BeEmpty(), "consumer name is not provided")
	agentRestConfig, err := clientcmd.BuildConfigFromFlags("", agentTestOpts.kubeConfig)
	Expect(err).To(Succeed())
	agentTestOpts.kubeClientSet, err = kubernetes.NewForConfig(agentRestConfig)
	Expect(err).To(Succeed())
	agentTestOpts.workClientSet, err = workclientset.NewForConfig(agentRestConfig)
	Expect(err).To(Succeed())

	// initialize the grpc source options
	grpcOptions = &grpcoptions.GRPCOptions{
		Dialer: &grpcoptions.GRPCDialer{
			URL: grpcServerAddress,
			KeepAliveOptions: grpcoptions.KeepAliveOptions{
				Enable:  true,
				Time:    6 * time.Second,
				Timeout: 1 * time.Second,
			},
		},
	}
	sourceID = "sourceclient-test" + rand.String(5)
	grpcCertSrt, err := serverTestOpts.kubeClientSet.CoreV1().Secrets(serverTestOpts.serverNamespace).Get(ctx, "maestro-grpc-cert", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		// retrieve the grpc cert from the maestro server and write to the grpc cert dir
		grpcServerCAFile := fmt.Sprintf("%s/ca.crt", grpcCertDir)
		grpcClientCert := fmt.Sprintf("%s/client.crt", grpcCertDir)
		grpcClientKey := fmt.Sprintf("%s/client.key", grpcCertDir)
		Expect(os.WriteFile(grpcServerCAFile, grpcCertSrt.Data["ca.crt"], 0644)).To(Succeed())
		Expect(os.WriteFile(grpcClientCert, grpcCertSrt.Data["client.crt"], 0644)).To(Succeed())
		Expect(os.WriteFile(grpcClientKey, grpcCertSrt.Data["client.key"], 0644)).To(Succeed())
		grpcClientTokenSrt, err := serverTestOpts.kubeClientSet.CoreV1().Secrets(serverTestOpts.serverNamespace).Get(ctx, "grpc-client-token", metav1.GetOptions{})
		Expect(err).To(Succeed())
		grpcClientTokenFile := fmt.Sprintf("%s/token", grpcCertDir)
		Expect(os.WriteFile(grpcClientTokenFile, grpcClientTokenSrt.Data["token"], 0644)).To(Succeed())
		// set CAFile and TokenFile for grpc authz
		grpcOptions.Dialer.TLSConfig, err = cert.AutoLoadTLSConfig(grpcServerCAFile, "", "", nil)
		Expect(err).To(Succeed())
		grpcOptions.Dialer.TokenFile = grpcClientTokenFile
		// create the clusterrole for grpc authz
		Expect(helper.CreateGRPCAuthRule(ctx, serverTestOpts.kubeClientSet, "grpc-pub-sub", "source", sourceID, []string{"pub", "sub"})).To(Succeed())

		grpcConn, err = helper.CreateGRPCConn(grpcServerAddress, grpcServerCAFile, grpcClientTokenFile)
		Expect(err).To(Succeed())
	} else {
		grpcConn, err = helper.CreateGRPCConn(grpcServerAddress, "", "")
		Expect(err).To(Succeed())
	}

	grpcClient = pbv1.NewCloudEventServiceClient(grpcConn)
	sourceWorkClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
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
	if grpcConn != nil {
		grpcConn.Close()
	}
	os.RemoveAll(grpcCertDir)
	cancel()
})

var _ = ReportAfterSuite("Maestro e2e Test Report", func(report Report) {
	junitReportFile := os.Getenv("JUNIT_REPORT_FILE")
	if junitReportFile != "" {
		err := reporter.GenerateJUnitReport(report, junitReportFile)
		if err != nil {
			log.Errorf("Failed to generate the report due to: %v", err)
		}
	}
})

func dumpDebugInfo() {
	// dump the maestro server logs
	dumpPodLogs(ctx, serverTestOpts.kubeClientSet, "app=maestro", serverTestOpts.serverNamespace)
	// dump the maestro agent ogs
	dumpPodLogs(ctx, agentTestOpts.kubeClientSet, "app=maestro-agent", agentTestOpts.agentNamespace)
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

		log.Infof("=========================================== POD LOGS START ===========================================")
		log.Infof("Pod %s/%s phase: %s", pod.Name, podNamespace, string(pod.Status.Phase))
		for _, containerStatus := range pod.Status.ContainerStatuses {
			log.Infof("Container %s status: %v", containerStatus.Name, containerStatus.State)
		}
		log.Infof("Pod %s/%s logs: \n%s", pod.Name, podNamespace, buf.String())
		log.Infof("=========================================== POD LOGS STOP ===========================================")
	}

	return nil
}
