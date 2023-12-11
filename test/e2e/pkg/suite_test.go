package e2e_test

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	apiServerAddress string
	kubeconfig       string
	consumer_id      string
	kubeClient       *kubernetes.Clientset
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "End-to-End Test Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	flag.StringVar(&apiServerAddress, "api-server", "", "Maestro API server address")
	flag.StringVar(&consumer_id, "consumer_id", "", "Connsumer ID is used to identify the consumer")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
}

var _ = BeforeSuite(func() {
	// validate the maestro api server is running
	_, err := sendHTTPRequest(http.MethodGet, apiServerAddress+"/api/maestro", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to maestro api server: %v", err))
	}

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
	if consumer_id == "" {
		panic("consumer_id is not provided")
	}
})

var _ = AfterSuite(func() {
	// later...
})

func buildHTTPClient(apiServerAddress string) *http.Client {
	tlsConfig := &tls.Config{
		//nolint:gosec
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}

	return &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
}

func sendHTTPRequest(method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	client := buildHTTPClient(url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			klog.Errorf("unable to close response body: %v", err)
		}
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}
