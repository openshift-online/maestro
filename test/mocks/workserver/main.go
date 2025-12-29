package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	workv1 "open-cluster-management.io/api/work/v1"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test/mocks/workserver/requests"
	"github.com/openshift-online/maestro/test/mocks/workserver/watcher"
)

var serverHealthinessTimeout = 20 * time.Second

const sourceID = "workserver-mock"

// WorkServer provides RESTful APIs for managing ManifestWorks using the grpcWorkClient.
// - Uses `grpcsource.NewMaestroGRPCSourceWorkClient` to create the gRPC work client
// - Watches ManifestWorks for status updates using `Watch()`
// - Stores watched works in `watcher.WorkStore` for quick access
// - GET requests return watched works with the latest status when available
// - All operations are performed through the grpcWorkClient interface
type WorkServer struct {
	apiServerAddress  string
	grpcServerAddress string
	consumerName      string
	bindAddress       string

	apiClient      *openapi.APIClient
	grpcConn       *grpc.ClientConn
	grpcWorkClient *grpcsource.WorkV1ClientWrapper
	watchedWorks   *watcher.WorkStore

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWorkServer() *WorkServer {
	return &WorkServer{}
}

func (s *WorkServer) ParseFlags() {
	flag.StringVar(&s.apiServerAddress, "api-server", "", "Maestro Restful API server address")
	flag.StringVar(&s.grpcServerAddress, "grpc-server", "", "Maestro gRPC server address")
	flag.StringVar(&s.consumerName, "consumer-name", "", "Consumer name is used to identify the consumer")
	flag.StringVar(&s.bindAddress, "bind-address", ":8080", "Address to bind the mock server HTTP API")
	flag.Parse()
}

func (s *WorkServer) Initialize() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Validate required parameters
	if s.apiServerAddress == "" {
		return fmt.Errorf("api-server is required")
	}
	if s.grpcServerAddress == "" {
		return fmt.Errorf("grpc-server is required")
	}
	if s.consumerName == "" {
		return fmt.Errorf("consumer-name is required")
	}

	// Initialize the API client
	cfg := &openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "MockServer/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         s.apiServerAddress,
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
	s.apiClient = openapi.NewAPIClient(cfg)

	// Create insecure gRPC connection
	var err error
	s.grpcConn, err = grpc.NewClient(s.grpcServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create grpc conn: %v", err)
	}

	// Initialize the grpc source options (no TLS)
	grpcOptions := &grpcoptions.GRPCOptions{
		Dialer: &grpcoptions.GRPCDialer{
			URL: s.grpcServerAddress,
		},
		ServerHealthinessTimeout: &serverHealthinessTimeout,
	}

	// Create the grpc work client
	logger, err := logging.NewStdLoggerBuilder().Build()
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	workInterface, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		s.ctx,
		logger,
		s.apiClient,
		grpcOptions,
		sourceID,
	)
	if err != nil {
		return fmt.Errorf("failed to create grpc work client: %v", err)
	}

	// Type assertion to get the wrapper
	wrapper, ok := workInterface.(*grpcsource.WorkV1ClientWrapper)
	if !ok {
		return fmt.Errorf("failed to cast work client to wrapper")
	}
	s.grpcWorkClient = wrapper

	// Start watching ManifestWorks
	watch, err := s.grpcWorkClient.ManifestWorks(s.consumerName).Watch(s.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}
	s.watchedWorks = watcher.StartWatch(s.ctx, watch)

	klog.Infof("Mock server initialized successfully")
	klog.Infof("API Server: %s", s.apiServerAddress)
	klog.Infof("gRPC Server: %s", s.grpcServerAddress)
	klog.Infof("Consumer: %s", s.consumerName)
	klog.Infof("Bind Address: %s", s.bindAddress)

	return nil
}

func (s *WorkServer) Start() error {
	router := mux.NewRouter()

	// Register routes
	router.HandleFunc("/api/v1/works", s.handleCreateWork).Methods("POST")
	router.HandleFunc("/api/v1/works/{name}", s.handleUpdateWork).Methods("PATCH")
	router.HandleFunc("/api/v1/works/{name}", s.handleGetWork).Methods("GET")

	klog.Infof("Starting mock server on %s", s.bindAddress)
	return http.ListenAndServe(s.bindAddress, router)
}

func (s *WorkServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.grpcConn != nil {
		s.grpcConn.Close()
	}
	return nil
}

// HTTP Handlers
func (s *WorkServer) handleCreateWork(w http.ResponseWriter, r *http.Request) {
	var req requests.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// Create ManifestWork from the request
	var work *workv1.ManifestWork
	if err := json.Unmarshal(req.WorkBytes, &work); err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal work from request: %v", err), http.StatusBadRequest)
		return
	}
	if work == nil {
		http.Error(w, "work must not be null", http.StatusBadRequest)
		return
	}

	// Create the work via grpcWorkClient
	createdWork, err := s.grpcWorkClient.ManifestWorks(s.consumerName).Create(s.ctx, work, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create work: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdWork)
}

func (s *WorkServer) handleUpdateWork(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	// Get the existing work first to verify it exists
	_, err = s.grpcWorkClient.ManifestWorks(s.consumerName).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("work %s not found: %v", name, err), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get work: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Patch the work
	patchedWork, err := s.grpcWorkClient.ManifestWorks(s.consumerName).Patch(s.ctx, name, types.MergePatchType, body, metav1.PatchOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to patch work: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(patchedWork)
}

func (s *WorkServer) handleGetWork(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Get from watched works only
	watchedWork := s.watchedWorks.Get(name)
	if watchedWork == nil {
		http.Error(w, fmt.Sprintf("work %s not found in watched works", name), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(watchedWork)
}

func main() {
	server := NewWorkServer()
	server.ParseFlags()

	if err := server.Initialize(); err != nil {
		klog.Fatalf("Failed to initialize server: %v", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		klog.Infof("Shutting down mock server...")
		if err := server.Shutdown(); err != nil {
			klog.Errorf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		klog.Fatalf("Failed to start server: %v", err)
	}
}
