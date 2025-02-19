package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

var (
	sourceID            = flag.String("source", "grpc", "The source for manifestwork client")
	grpcServerAddr      = flag.String("grpc-server", "127.0.0.1:8090", "The grpc server address")
	grpcServerTLS       = flag.Bool("grpc-server-tls", false, "Connect grpc server with TLS if true")
	grpcServerCAFile    = flag.String("grpc-server-ca-file", "", "The CA for grpc server")
	grpcClientCertFile  = flag.String("grpc-client-cert-file", "", "The client certificate to access grpc server")
	grpcClientKeyFile   = flag.String("grpc-client-key-file", "", "The client key to access grpc server")
	grpcClientTokenFile = flag.String("grpc-client-token-file", "", "The client token to access grpc server")
	consumerName        = flag.String("consumer-name", "", "The Consumer Name")
	cloudEventFile      = flag.String("cloudevent-file", "", "The absolute file path containing the CloudEvent resource")
	enableSubscribing   = flag.Bool("enable-subscribing", false, "If true, subscribe to the CloudEvent resource status.")
)

func main() {
	flag.Parse()

	if len(*cloudEventFile) == 0 {
		log.Fatalf("the cloudevent file is required")
	}

	if *grpcServerTLS {
		if len(*grpcServerCAFile) == 0 {
			log.Fatalf("the grpc server CA file is required when TLS enabled")
		}

		if len(*grpcClientTokenFile) == 0 {
			log.Fatalf("the grpc client token file is required when TLS enabled")
		}
	}

	var opts []grpc.DialOption
	if *grpcServerTLS {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			log.Fatalf("failed to load system cert pool: %v", err)
		}

		caPEM, err := os.ReadFile(*grpcServerCAFile)
		if err != nil {
			log.Fatalf("failed to read grpc server CA file: %v", err)
		}

		if ok := certPool.AppendCertsFromPEM(caPEM); !ok {
			log.Fatalf("failed to append grpc server CA certificate")
		}

		tlsConfig := &tls.Config{
			RootCAs:    certPool,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}

		clientToken, err := os.ReadFile(*grpcClientTokenFile)
		if err != nil {
			log.Fatalf("failed to read grpc client token file: %v", err)
		}
		perRPCCred := oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: string(clientToken)})}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithPerRPCCredentials(perRPCCred))
	} else {
		// no TLS and authz
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(*grpcServerAddr, opts...)
	if err != nil {
		log.Fatalf("failed to create grpc connection: %v", err)
	}
	defer conn.Close()

	client := pbv1.NewCloudEventServiceClient(conn)
	cloudeventJSON, err := os.ReadFile(*cloudEventFile)
	if err != nil {
		log.Fatalf("failed to read cloudevent file: %v", err)
	}

	evt := &cloudevents.Event{}
	if err := json.Unmarshal(cloudeventJSON, evt); err != nil {
		log.Fatalf("failed to unmarshal cloudevent: %v", err)
	}
	// override the consumer name
	evt.SetExtension(types.ExtensionClusterName, *consumerName)

	ctx := context.TODO()
	pbEvt := &pbv1.CloudEvent{}
	if err = grpcprotocol.WritePBMessage(ctx, binding.ToMessage(evt), pbEvt); err != nil {
		log.Fatalf("failed to convert spec from cloudevent to protobuf: %v", err)
	}

	if _, err = client.Publish(ctx, &pbv1.PublishRequest{Event: pbEvt}); err != nil {
		log.Fatalf("failed to publish: %v", err)
	}

	log.Printf("=======================================")
	log.Printf("Published spec with cloudevent:\n%v\n\n", evt)
	log.Printf("=======================================")

	if *enableSubscribing {
		subReq := &pbv1.SubscriptionRequest{
			Source: *sourceID,
		}

		subClient, err := client.Subscribe(ctx, subReq)
		if err != nil {
			log.Fatalf("failed to subscribe: %v", err)
		}

		for {
			pvEvt, err := subClient.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("failed to receive cloudevent: %v", err)
			}
			evt, err := binding.ToEvent(ctx, grpcprotocol.NewMessage(pvEvt))
			if err != nil {
				log.Fatalf("failed to convert status from protobuf to cloudevent: %v", err)
			}

			log.Printf("=======================================")
			log.Printf("Received status with cloudevent:\n%v\n", evt)
			log.Printf("=======================================")
		}
	}
}
