package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/cert"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

var (
	sourceID            = flag.String("source", "", "The source for manifestwork client")
	consumerName        = flag.String("consumer-name", "", "The Consumer Name")
	maestroServerAddr   = flag.String("maestro-server", "https://127.0.0.1:8000", "The maestro server address")
	grpcServerAddr      = flag.String("grpc-server", "127.0.0.1:8090", "The grpc server address")
	grpcServerCAFile    = flag.String("grpc-server-ca-file", "", "The CA for grpc server")
	grpcClientCertFile  = flag.String("grpc-client-cert-file", "", "The client certificate to access grpc server")
	grpcClientKeyFile   = flag.String("grpc-client-key-file", "", "The client key to access grpc server")
	grpcClientTokenFile = flag.String("grpc-client-token-file", "", "The client token to access grpc server")
)

var (
	workClient workv1client.WorkV1Interface
	works      = make(map[string]*workv1.ManifestWork)
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(*sourceID) == 0 {
		log.Fatalf("the source is required")
	}

	if len(*consumerName) == 0 {
		log.Fatalf("the consumer_name is required")
	}

	maestroAPIClient := openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         *maestroServerAddr,
				Description: "current domain",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			}},
			Timeout: 10 * time.Second,
		},
	})

	grpcOptions := &grpc.GRPCOptions{Dialer: &grpc.GRPCDialer{}}
	grpcOptions.Dialer.URL = *grpcServerAddr

	if *grpcServerCAFile != "" && *grpcClientCertFile != "" && *grpcClientKeyFile != "" {
		certConfig := cert.CertConfig{
			CAFile:         *grpcServerCAFile,
			ClientCertFile: *grpcClientCertFile,
			ClientKeyFile:  *grpcClientKeyFile,
		}
		if err := certConfig.EmbedCerts(); err != nil {
			log.Fatal(err)
		}
		tlsConfig, err := cert.AutoLoadTLSConfig(
			certConfig,
			func() (*cert.CertConfig, error) {
				certConfig := cert.CertConfig{
					CAFile:         *grpcServerCAFile,
					ClientCertFile: *grpcClientCertFile,
					ClientKeyFile:  *grpcClientKeyFile,
				}
				if err := certConfig.EmbedCerts(); err != nil {
					return nil, err
				}
				return &certConfig, nil
			},
			grpcOptions.Dialer,
		)
		if err != nil {
			log.Fatal(err)
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
	}

	if *grpcClientTokenFile != "" {
		token, err := os.ReadFile(*grpcClientTokenFile)
		if err != nil {
			log.Fatal(err)
		}
		grpcOptions.Dialer.Token = string(token)
	}

	logger, err := logging.NewStdLoggerBuilder().Build()
	if err != nil {
		log.Fatal(err)
	}

	workClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		logger,
		maestroAPIClient,
		grpcOptions,
		*sourceID,
	)
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := workClient.ManifestWorks(*consumerName).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to watch manifestworks: %v", err)
	}

	// start to watch the manifestwork status
	log.Printf("Start to watch the manifestworks for source %q and consumer %q", *sourceID, *consumerName)
	go startWatcher(ctx, watcher)

	router := mux.NewRouter()
	router.HandleFunc("/works", getWorks).Methods("GET")
	router.HandleFunc("/works/{name}", getWork).Methods("GET")
	router.HandleFunc("/works", createWork).Methods("POST")
	router.HandleFunc("/works/{name}", updateWork).Methods("PUT")
	router.HandleFunc("/works/{name}", deleteWork).Methods("DELETE")

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func getWorks(w http.ResponseWriter, r *http.Request) {
	var workList workv1.ManifestWorkList
	for _, work := range works {
		workList.Items = append(workList.Items, *work)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workList)
}

func getWork(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	work, ok := works[params["name"]]
	if !ok {
		http.Error(w, fmt.Sprintf("manifestwork not found: %s", params["name"]), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(work)
}

func createWork(w http.ResponseWriter, r *http.Request) {
	var work workv1.ManifestWork
	if err := json.NewDecoder(r.Body).Decode(&work); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	created, err := workClient.ManifestWorks(*consumerName).Create(r.Context(), &work, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	works[created.Name] = created
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func updateWork(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var updating workv1.ManifestWork
	if err := json.NewDecoder(r.Body).Decode(&updating); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, ok := works[params["name"]]; !ok {
		http.Error(w, fmt.Sprintf("manifestwork not found: %s", params["name"]), http.StatusNotFound)
		return
	}

	if updating.Name != params["name"] {
		http.Error(w, "manifestwork name in URL and body do not match", http.StatusBadRequest)
		return
	}

	found, err := workClient.ManifestWorks(*consumerName).Get(r.Context(), updating.Name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newWork := found.DeepCopy()
	newWork.Spec.Workload.Manifests = updating.Spec.Workload.Manifests
	patchData, err := grpcsource.ToWorkPatch(found, newWork)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, err := workClient.ManifestWorks(*consumerName).Patch(r.Context(), found.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	works[params["name"]] = updated
	json.NewEncoder(w).Encode(updated)
}

func deleteWork(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	if _, ok := works[params["name"]]; !ok {
		http.Error(w, fmt.Sprintf("manifestwork not found: %s", params["name"]), http.StatusNotFound)
		return
	}

	err := workClient.ManifestWorks(*consumerName).Delete(r.Context(), params["name"], metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	delete(works, params["name"])
	w.WriteHeader(http.StatusNoContent)
}

func startWatcher(ctx context.Context, watcher watch.Interface) {
	ch := watcher.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}

			switch event.Type {
			case watch.Modified:
				if work, ok := event.Object.(*workv1.ManifestWork); ok {
					works[work.Name] = work
				}
			case watch.Deleted:
				if work, ok := event.Object.(*workv1.ManifestWork); ok {
					if meta.IsStatusConditionTrue(work.Status.Conditions, common.ResourceDeleted) && !work.DeletionTimestamp.IsZero() {
						delete(works, work.Name)
					}
				}
			}
		}
	}
}
