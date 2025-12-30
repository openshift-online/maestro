package upgrade_test

import (
	"context"
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"

	"github.com/openshift-online/maestro/test/e2e/pkg/reporter"
	"github.com/openshift-online/maestro/test/mocks/workserver/client"
)

var (
	ctx               context.Context
	cancel            context.CancelFunc
	workServerAddress string
	kubeConfig        string
	workServerClient  *client.WorkServerClient
	kubeClientSet     kubernetes.Interface
	workClientSet     workclientset.Interface
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upgrade Test Suite")
}

func init() {
	flag.StringVar(&workServerAddress, "work-server", "http://workserver:8080", "Maestro work server address")
	flag.StringVar(&kubeConfig, "agent-kubeconfig", "", "Path to the kubeconfig file for the maestro agent")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	agentRestConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	Expect(err).To(Succeed())
	kubeClientSet, err = kubernetes.NewForConfig(agentRestConfig)
	Expect(err).To(Succeed())
	workClientSet, err = workclientset.NewForConfig(agentRestConfig)
	Expect(err).To(Succeed())

	workServerClient = client.NewWorkServerClient(workServerAddress)
})

var _ = AfterSuite(func() {
	// cancel the context
	cancel()
})

var _ = ReportAfterSuite("Maestro Upgrade Test Report", func(report Report) {
	junitReportFile := os.Getenv("JUNIT_REPORT_FILE")
	if junitReportFile != "" {
		err := reporter.GenerateJUnitReport(report, junitReportFile)
		if err != nil {
			klog.Errorf("Failed to generate the report due to: %v", err)
		}
	}
})
