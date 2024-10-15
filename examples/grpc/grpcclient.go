package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
	grpcprotocol "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protocol"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The absolute file path containing the CA root cert file")
	serverAddr         = flag.String("grpc_server", "localhost:8090", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.example.com", "The server name used to verify the hostname returned by the TLS handshake")
	cloudEventFile     = flag.String("cloudevents_json_file", "", "The absolute file path containing the CloudEvent resource")
	subscribeStatus    = flag.Bool("subscribe_status", false, "If true, subscribe to the CloudEvent resource status.")
)

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *tls {
		creds, err := credentials.NewClientTLSFromFile(*caFile, *serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
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

	if *subscribeStatus {
		subReq := &pbv1.SubscriptionRequest{
			Source: "grpc",
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
