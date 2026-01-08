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

	"k8s.io/klog/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/cert"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test"
	"github.com/openshift-online/maestro/test/e2e/pkg/reporter"
)

var serverHealthinessTimeout = 20 * time.Second

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
	dumpLogs          bool
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
	flag.BoolVar(&dumpLogs, "dump-logs", false, "Dump the pod logs after test")
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
		},
		ServerHealthinessTimeout: &serverHealthinessTimeout,
	}
	sourceID = "sourceclient-test-" + rand.String(5)
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
		// set CAFile and Token for grpc authz
		certConfig := cert.CertConfig{CAFile: grpcServerCAFile}
		err = certConfig.EmbedCerts()
		Expect(err).To(Succeed())
		grpcOptions.Dialer.TLSConfig, err = cert.AutoLoadTLSConfig(certConfig, nil, nil)
		Expect(err).To(Succeed())
		grpcOptions.Dialer.Token = string(grpcClientTokenSrt.Data["token"])
		// create the clusterrole for grpc authz
		Expect(helper.AddGRPCAuthRule(ctx, serverTestOpts.kubeClientSet, "grpc-pub-sub", "source", sourceID)).To(Succeed())

		grpcConn, err = helper.CreateGRPCConn(grpcServerAddress, grpcServerCAFile, string(grpcClientTokenSrt.Data["token"]))
		Expect(err).To(Succeed())
	} else {
		grpcConn, err = helper.CreateGRPCConn(grpcServerAddress, "", "")
		Expect(err).To(Succeed())
	}

	logger, err := logging.NewStdLoggerBuilder().Build()
	Expect(err).ShouldNot(HaveOccurred())

	grpcClient = pbv1.NewCloudEventServiceClient(grpcConn)
	sourceWorkClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		logger,
		apiClient,
		grpcOptions,
		sourceID,
	)
	Expect(err).ShouldNot(HaveOccurred())

	// check the resources left over from previous tests
	Expect(checkResources(ctx)).To(Succeed())
})

var _ = AfterSuite(func() {
	// dump debug info
	if dumpLogs {
		dumpDebugInfo()
	}
	// clean up the resources
	Eventually(func() error {
		return cleanupResources(ctx)
	}, 2*time.Minute, 10*time.Second).ShouldNot(HaveOccurred())

	// close the grpc connection
	if grpcConn != nil {
		grpcConn.Close()
	}
	// remove the grpc cert dir
	os.RemoveAll(grpcCertDir)
	// cancel the context
	cancel()
})

var _ = ReportAfterSuite("Maestro e2e Test Report", func(report Report) {
	junitReportFile := os.Getenv("JUNIT_REPORT_FILE")
	if junitReportFile != "" {
		err := reporter.GenerateJUnitReport(report, junitReportFile)
		if err != nil {
			klog.Errorf("Failed to generate the report due to: %v", err)
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

		klog.Infof("=========================================== POD LOGS START ===========================================")
		klog.Infof("Pod %s/%s phase: %s", pod.Name, podNamespace, string(pod.Status.Phase))
		for _, containerStatus := range pod.Status.ContainerStatuses {
			klog.Infof("Container %s status: %v", containerStatus.Name, containerStatus.State)
		}
		klog.Infof("Pod %s/%s logs: \n%s", pod.Name, podNamespace, buf.String())
		klog.Infof("=========================================== POD LOGS STOP ===========================================")
	}

	return nil
}

func checkResources(ctx context.Context) error {
	By("check the resources left over after previous tests")
	mwGRPCClient := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName)

	workList, err := mwGRPCClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list manifestwork: %v", err)
	}

	if len(workList.Items) != 0 {
		return fmt.Errorf("resource leak detected: %d resources found", len(workList.Items))
	}

	return nil
}

func cleanupResources(ctx context.Context) error {
	By("check the resources left over after test")

	mwGRPCClient := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName)

	workList, err := mwGRPCClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list manifestwork: %v", err)
	}

	if len(workList.Items) == 0 {
		return nil
	}

	works := []string{}
	for _, work := range workList.Items {
		By(fmt.Sprintf("clean up the left over resources %s after test", work.Name))
		if err := mwGRPCClient.Delete(ctx, work.Name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete manifestwork: %v", err)
		}
		works = append(works, work.Name)
	}

	appliedWorks, err := agentTestOpts.workClientSet.WorkV1().AppliedManifestWorks().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list appliedmanifestworks: %v", err)
	}

	for _, appliedWork := range appliedWorks.Items {
		if !contains(appliedWork.Spec.ManifestWorkName, works) {
			continue
		}

		By(fmt.Sprintf("clean up the left over appliedmanifestwork %s for %s after test", appliedWork.Name, appliedWork.Spec.ManifestWorkName))
		err := agentTestOpts.workClientSet.WorkV1().AppliedManifestWorks().Delete(ctx, appliedWork.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete the appliedmanifestwork %s: %v", appliedWork.Name, err)
		}
	}

	return nil
}

func contains(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
