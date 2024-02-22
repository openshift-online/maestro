## gRPC server
gPRC server is disabled by default. You can enable it by passing `--enable-grpc-server=true` to the maestro server start command.

### How to use gRPC server
You need to initialize the gRPC client with the server address and port. You can use the `grpcoptions` package to initialize the gRPC client. 

Before initializing the client, you need to define the resource codec. The resource codec is used to encode and decode the resource. The resource codec should implement the `ResourceCodec` interface. The `ResourceCodec` interface has two methods: `Encode` and `Decode`. The `Encode` method is used to encode the resource to the cloudevents, and the `Decode` method is used to decode the cloudevents to the resource. Refer to the `test/grpc_codec.go` for the example of the resource codec.

Once the resource codec is defined, you can initialize the gRPC client using the `grpcoptions` package. The `grpcoptions` package has the `NewGRPCOptions` method to initialize the gRPC options. The `NewGRPCOptions` method takes the server address and port as the input and returns the gRPC options. The gRPC options are used to initialize the gRPC client. The `grpcoptions` package also has the `NewSourceOptions` method to initialize the source options. The `NewSourceOptions` method takes the gRPC options and the source name as the input and returns the source options. The source options are used to initialize the source client. The source client is used to publish the cloudevents:
```
    import grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	...
    grpcOptions := grpcoptions.NewGRPCOptions()
	grpcOptions.URL = h.Env().Config.GRPCServer.BindAddress
	grpcSourceCloudEventsClient, err := generic.NewCloudEventSourceClient[*api.Resource](
		ctx,
		grpcoptions.NewSourceOptions(grpcOptions, "grpc-example"),
		nil,
		nil,
		&ResourceCodec{},
	)
```
Once the client is initialized, you can use it to publish the cloudevents:
```
    grpcSourceCloudEventsClient.Publish(context.TODO(), types.CloudEventsType{
		CloudEventsDataType: payload.ManifestEventDataType,
		SubResource:         types.SubResourceSpec,
		Action:              config.CreateRequestAction,
	}, res)
```
The `res` is the resource you want to publish. The format of the resource is defined:
```
    resource := &api.Resource{
		ConsumerID: consumerID,
		Manifest:   testManifest,
	}
    ...
    testManifest := map[string]interface{}{}
	json.Unmarshal(`{
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": {
        "name": "nginx",
        "namespace": "default"
        },
        "spec": {
        "replicas": %d,
        "selector": {
            "matchLabels": {
            "app": "nginx"
            }
        },
        "template": {
            "metadata": {
            "labels": {
                "app": "nginx"
            }
            },
            "spec": {
            "containers": [
                {
                "image": "nginxinc/nginx-unprivileged",
                "name": "nginx"
                }
            ]
            }
        }
        }
    }`, &testManifest);
	
```