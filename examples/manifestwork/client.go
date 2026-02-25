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
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/ocm-sdk-go/logging"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/cert"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	sdkgologging "open-cluster-management.io/sdk-go/pkg/logging"
)

var (
	sourceID                 = flag.String("source", "mw-client-example", "The source for manifestwork client")
	maestroServerAddr        = flag.String("maestro-server", "https://127.0.0.1:30080", "The maestro server address")
	grpcServerAddr           = flag.String("grpc-server", "127.0.0.1:30090", "The grpc server address")
	grpcServerCAFile         = flag.String("grpc-server-ca-file", "", "The CA for grpc server")
	grpcClientCertFile       = flag.String("grpc-client-cert-file", "", "The client certificate to access grpc server")
	grpcClientKeyFile        = flag.String("grpc-client-key-file", "", "The client key to access grpc server")
	grpcClientTokenFile      = flag.String("grpc-client-token-file", "", "The client token to access grpc server")
	consumerName             = flag.String("consumer-name", "", "The Consumer Name")
	serverHealthinessTimeout = flag.Duration("server-healthiness-timeout", 20*time.Second, "The server healthiness timeout")
	printWorkDetails         = flag.Bool("print-work-details", false, "Print detailed work information (for watch command)")
	insecureSkipVerify       = flag.Bool("insecure-skip-verify", false, "Skip TLS verification when using https (INSECURE)")
	verbose                  = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	// Custom argument parsing to allow flags anywhere
	var command string
	var commandArgs []string
	var otherArgs []string

	// Separate command/args from flags
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") {
			// Keep the flag token itself
			otherArgs = append(otherArgs, arg)

			// If this flag has a value (-flag=value or -flag value). Do not consume
			// the next token for bool flags (e.g. -print-work-details).
			if !strings.Contains(arg, "=") && i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
				name := strings.TrimLeft(arg, "-")
				if f := flag.CommandLine.Lookup(name); f != nil {
					type boolFlag interface{ IsBoolFlag() bool }
					if bf, ok := f.Value.(boolFlag); ok && bf.IsBoolFlag() {
						continue
					}
					i++
					otherArgs = append(otherArgs, os.Args[i])
				}
			}
		} else {
			if command == "" {
				command = arg
			} else {
				commandArgs = append(commandArgs, arg)
			}
		}
	}

	// Parse flags
	// check if the klog flag is already registered to avoid duplicate flag define error
	if flag.CommandLine.Lookup("alsologtostderr") == nil {
		klog.InitFlags(nil)
	}
	flag.CommandLine.Parse(otherArgs)

	// Configure klog based on verbose flag
	if *verbose {
		flag.Set("v", "4")
	} else {
		// Completely disable klog output by setting a discard logger
		klog.SetLogger(logr.Discard())
	}

	if command == "" {
		printUsage()
		os.Exit(1)
	}

	if len(*consumerName) == 0 {
		log.Fatalf("the consumer-name is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for watch command
	if command == "watch" {
		stopCh := make(chan os.Signal, 1)
		signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			defer cancel()
			<-stopCh
		}()
	}

	workClient, err := createWorkClient(ctx)
	if err != nil {
		log.Fatalf("failed to create work client: %v", err)
	}

	// Set operation id
	opId := uuid.NewString()
	opCtx := context.WithValue(ctx, sdkgologging.ContextTracingOPIDKey, opId)
	switch command {
	case "get":
		if len(commandArgs) < 1 {
			log.Fatalf("usage: client.go get <work-name>")
		}
		workName := commandArgs[0]
		fmt.Printf("Get manifestwork %s/%s (opid=%s)\n", *consumerName, workName, opId)
		if err := getWork(opCtx, workClient, workName); err != nil {
			log.Fatal(err)
		}
	case "list":
		fmt.Printf("List manifestworks (opid=%s):\n", opId)
		if err := listWorks(opCtx, workClient); err != nil {
			log.Fatal(err)
		}
	case "apply":
		if len(commandArgs) < 1 {
			log.Fatalf("usage: client.go apply <manifestwork-file>")
		}
		manifestworkFile := commandArgs[0]
		fmt.Printf("Apply manifestwork (opid=%s):\n", opId)
		if err := applyWork(opCtx, workClient, manifestworkFile); err != nil {
			log.Fatal(err)
		}
	case "delete":
		if len(commandArgs) < 1 {
			log.Fatalf("usage: client.go delete <work-name>")
		}
		workName := commandArgs[0]
		fmt.Printf("Delete manifestwork %s/%s (opid=%s):\n", *consumerName, workName, opId)
		if err := deleteWork(opCtx, workClient, workName); err != nil {
			log.Fatal(err)
		}
	case "watch":
		fmt.Printf("Watch manifestworks (opid=%s):\n", opId)
		if err := watchWorks(opCtx, workClient); err != nil {
			log.Fatal(err)
		}
	default:
		printUsage()
		log.Fatalf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: maestro-cli <command> [arguments] [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  get <work-name>           Get a specific manifestwork")
	fmt.Println("  list                      List all manifestworks")
	fmt.Println("  apply <manifestwork-file> Create or update a manifestwork from a JSON file")
	fmt.Println("  delete <work-name>        Delete a manifestwork")
	fmt.Println("  watch                     Watch for manifestwork changes")
	fmt.Println()
	fmt.Println("Required Flags:")
	fmt.Println("  --consumer-name string    The Consumer Name")
	fmt.Println()
	fmt.Println("Common Flags:")
	fmt.Println("  --maestro-server string   The maestro server address (default \"https://127.0.0.1:30080\")")
	fmt.Println("  --grpc-server string      The grpc server address (default \"127.0.0.1:30090\")")
	fmt.Println("  --insecure-skip-verify    Skip TLS verification when using https (INSECURE)")
	fmt.Println("  --verbose                 Enable verbose logging")
	fmt.Println()
	fmt.Println("Additional Flags:")
	fmt.Println("  --source string                     The source for manifestwork client (default \"mw-client-example\")")
	fmt.Println("  --grpc-server-ca-file string        The CA for grpc server")
	fmt.Println("  --grpc-client-cert-file string      The client certificate to access grpc server")
	fmt.Println("  --grpc-client-key-file string       The client key to access grpc server")
	fmt.Println("  --grpc-client-token-file string     The client token to access grpc server")
	fmt.Println("  --server-healthiness-timeout duration The server healthiness timeout (default 20s)")
	fmt.Println("  --print-work-details                Print detailed work information (for watch command)")
}

func createWorkClient(ctx context.Context) (workv1client.WorkV1Interface, error) {
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
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: *insecureSkipVerify,
			}},
			Timeout: 10 * time.Second,
		},
	})

	loggerBuilder := logging.NewStdLoggerBuilder().Info(false)
	if *verbose {
		loggerBuilder.Info(true)
		loggerBuilder.Debug(true)
	}
	logger, err := loggerBuilder.Build()
	if err != nil {
		return nil, err
	}

	grpcOptions := &grpc.GRPCOptions{
		Dialer:                   &grpc.GRPCDialer{},
		ServerHealthinessTimeout: serverHealthinessTimeout,
	}
	grpcOptions.Dialer.URL = *grpcServerAddr

	if *grpcServerCAFile != "" && *grpcClientCertFile != "" && *grpcClientKeyFile != "" {
		// Setup TLS if certificates are provided
		certConfig := cert.CertConfig{
			CAFile:         *grpcServerCAFile,
			ClientCertFile: *grpcClientCertFile,
			ClientKeyFile:  *grpcClientKeyFile,
		}
		if err := certConfig.EmbedCerts(); err != nil {
			return nil, err
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
			return nil, err
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
	} else if *grpcServerCAFile != "" && *grpcClientTokenFile != "" {
		// Setup token if provided
		token, err := os.ReadFile(*grpcClientTokenFile)
		if err != nil {
			return nil, err
		}
		grpcOptions.Dialer.Token = strings.TrimSpace(string(token))
		certConfig := cert.CertConfig{
			CAFile: *grpcServerCAFile,
		}
		if err := certConfig.EmbedCerts(); err != nil {
			return nil, err
		}
		tlsConfig, err := cert.AutoLoadTLSConfig(
			certConfig,
			func() (*cert.CertConfig, error) {
				certConfig := cert.CertConfig{
					CAFile: *grpcServerCAFile,
				}
				if err := certConfig.EmbedCerts(); err != nil {
					return nil, err
				}
				return &certConfig, nil
			},
			grpcOptions.Dialer,
		)
		if err != nil {
			return nil, err
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
	}

	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		logger,
		maestroAPIClient,
		grpcOptions,
		*sourceID,
	)
	if err != nil {
		return nil, err
	}

	return workClient, nil
}

func getWork(ctx context.Context, workClient workv1client.WorkV1Interface, workName string) error {
	work, err := workClient.ManifestWorks(*consumerName).Get(ctx, workName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get work: %w", err)
	}

	workJSON, err := json.MarshalIndent(work, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal work: %w", err)
	}

	fmt.Printf("%s\n", string(workJSON))
	return nil
}

func listWorks(ctx context.Context, workClient workv1client.WorkV1Interface) error {
	workList, err := workClient.ManifestWorks(*consumerName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list works: %w", err)
	}

	if len(workList.Items) == 0 {
		fmt.Println("No manifestworks found")
		return nil
	}

	fmt.Printf("%-36s %-15s %-36s %s\n", "Consumer", "Name", "UID", "Created")
	for _, work := range workList.Items {
		fmt.Printf("%-36s %-15s %-36s %s\n",
			*consumerName,
			work.Name,
			work.UID,
			work.CreationTimestamp.Format(time.RFC3339))
	}

	return nil
}

func applyWork(ctx context.Context, workClient workv1client.WorkV1Interface, manifestworkFile string) error {
	workJSON, err := os.ReadFile(manifestworkFile)
	if err != nil {
		return fmt.Errorf("failed to read manifestwork file: %w", err)
	}

	manifestwork := &workv1.ManifestWork{}
	if err := json.Unmarshal(workJSON, manifestwork); err != nil {
		return fmt.Errorf("failed to unmarshal manifestwork: %w", err)
	}

	// Try to get the work first to see if it exists
	existingWork, err := workClient.ManifestWorks(*consumerName).Get(ctx, manifestwork.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Work doesn't exist, create it
		createdWork, err := workClient.ManifestWorks(*consumerName).Create(ctx, manifestwork, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create work: %w", err)
		}

		fmt.Printf("Work %s/%s (uid=%s) created successfully\n", *consumerName, manifestwork.Name, createdWork.UID)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get work before apply: %w", err)
	}

	// Use ToWorkPatch to create a merge patch
	patchData, err := grpcsource.ToWorkPatch(existingWork, manifestwork)
	if err != nil {
		return fmt.Errorf("failed to create patch: %w", err)
	}

	patchedWork, err := workClient.ManifestWorks(*consumerName).Patch(ctx, manifestwork.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch work: %w", err)
	}
	fmt.Printf("Work %s/%s (uid=%s) updated successfully\n", *consumerName, manifestwork.Name, patchedWork.UID)

	return nil
}

func deleteWork(ctx context.Context, workClient workv1client.WorkV1Interface, workName string) error {
	// Get the work first to retrieve UID for better logging
	work, err := workClient.ManifestWorks(*consumerName).Get(ctx, workName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get work before deletion: %w", err)
	}

	err = workClient.ManifestWorks(*consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete work: %w", err)
	}

	fmt.Printf("Work %s/%s (uid=%s) deleted successfully\n", *consumerName, workName, work.UID)
	return nil
}

func watchWorks(ctx context.Context, workClient workv1client.WorkV1Interface) error {
	watcher, err := workClient.ManifestWorks(*consumerName).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Stop()

	fmt.Println("Watching for manifestwork changes... (Press Ctrl+C to stop)")

	ch := watcher.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			work, ok := event.Object.(*workv1.ManifestWork)
			if !ok {
				return fmt.Errorf("unexpected object type: %T\n", event.Object)
			}
			if err := printWatchEvent(event.Type, work); err != nil {
				return err
			}
		}
	}
}

func printWatchEvent(eventType watch.EventType, work *workv1.ManifestWork) error {
	switch eventType {
	case watch.Added:
		fmt.Printf("[ADDED] Work: %s/%s (uid=%s)\n", work.Namespace, work.Name, work.UID)
	case watch.Modified:
		fmt.Printf("[MODIFIED] Work: %s/%s (uid=%s)\n", work.Namespace, work.Name, work.UID)
	case watch.Deleted:
		fmt.Printf("[DELETED] Work: %s/%s (uid=%s)\n", work.Namespace, work.Name, work.UID)
	default:
		return fmt.Errorf("unsupported event type")
	}

	if *printWorkDetails {
		workJSON, err := json.MarshalIndent(work, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal work: %v", err)
		}
		fmt.Printf("%s\n", string(workJSON))
	}
	return nil
}
