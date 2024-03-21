package e2e_test

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/test"
)

var (
	apiServerAddress string
	kubeconfig       string
	consumer_name    string
	kubeClient       *kubernetes.Clientset
	apiClient        *openapi.APIClient
	helper           *test.Helper
	T                *testing.T
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	T = t
	RunSpecs(t, "End-to-End Test Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	flag.StringVar(&apiServerAddress, "api-server", "", "Maestro API server address")
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
})

var _ = AfterSuite(func() {
	// later...
})
