# Maestro gRPC server

## Authentication and Authorization

By default, the gRPC server enable server side TLS. To disable that, set `--disable-grpc-tls=true` to the maestro server command. However, if you need Authentication and Authorization, server side TLS must remain enabled.

For authorization, the gRPC server uses a mock authorizer by default. To enable real authorization, set `--grpc-authn-type` to either `mtls` or `token`. Depending on the authorizer type, you will need to create authorization rule resources, which are standard Kubernetes RBAC resources.

1. mTLS-Based Authorization

For mTLS-based authorization, specify the client CA file using `--grpc-client-ca-file`. The server will validate the client certificate against this CA.

Then create authorization rules based on the `CN (Common Name)` or `O (Organization)` in the client certificate, representing the user or group. For example, to allow the user "Alice" to publish and subscribe to the `policy` source, use the following Kubernetes RBAC configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: policy-pub-sub
rules:
- nonResourceURLs:
  - /sources/policy
  verbs:
  - pub
  - sub
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: policy-pub-sub
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: policy-pub-sub
subjects:
- kind: User
  name: Alice
  apiGroup: rbac.authorization.k8s.io
```

On the gRPC client side, configure the [gRPC options](https://pkg.go.dev/open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc#GRPCOptions) with the client certificate and key files, as follows:

```golang
grpcOptions = grpcoptions.NewGRPCOptions()
grpcOptions.URL = grpcServerAddr
grpcOptions.CAFile = grpcServerCAFile
grpcOptions.ClientCertFile = grpcClientCertFile
grpcOptions.ClientKeyFile = grpcClientKeyFile
```

The `grpcClientCertFile` and `grpcClientKeyFile` should contain the certificate signed by the client CA. For the example above, the CN must be "Alice".

2. Token-Based Authorization

For token-based authorization, the gRPC server authenticates the client using a Kubernetes service account token. The service account is supposed created by the gRPC client.

Create authorization rules based on the service account associated with the token. For example, to allow the service account `open-cluster-management/policy-controller` to publish and subscribe to the `policy` source, use the following Kubernetes RBAC configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: policy-pub-sub
rules:
- nonResourceURLs:
  - /sources/policy
  verbs:
  - pub
  - sub
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: policy-pub-sub
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: policy-pub-sub
subjects:
- kind: ServiceAccount
  name: policy-controller
  namespace: open-cluster-management
```

On the gRPC client side, configure the gRPC options with the token file, as follows:

```golang
grpcOptions = grpcoptions.NewGRPCOptions()
grpcOptions.URL = grpcServerAddr
grpcOptions.CAFile = grpcServerCAFile
grpcOptions.TokenFile = grpcClientTokenFile
```

The `grpcClientTokenFile` stores the token for the corresponding service account. In the example above, it holds the token for the `open-cluster-management/policy-controller` service account.

## How to Use gPRC Source Client

### Initliaze the gRPC source client

1. Initialize the gRPC source client by employing the [grpc package](https://pkg.go.dev/open-cluster-management.io/sdk-go@v0.13.0/pkg/cloudevents/generic/options/grpc).

    - Set up the gRPC options, including the gRPC server URL, SSL configuration, and other relevant parameters.
    - Initialize the gRPC source options by utilizing the previously configured gRPC options and specifying the source ID.

    ```golang
    import grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"

    grpcOptions = grpcoptions.NewGRPCOptions()
    grpcOptions.URL = h.Env().Config.GRPCServer.BindAddress
    // set grpc client authentication and authorization configuration
    // if gRPC Server enable authentication and authorization.
    // grpcOptions.CAFile = grpcServerCAFile
    // ClientCertFile = grpcClientCertFile
    // ClientKeyFile = grpcClientKeyFile
	  // grpcOptions.TokenFile = grpcClientTokenFile
    grpcSourceOption = grpcoptions.NewSourceOptions(grpcOptions, "grpc-source-example")
    ```

2. Define the resource codec responsible for encoding and decoding the resource. Ensure that the resource codec adheres to the [generic.Codec](https://pkg.go.dev/open-cluster-management.io/sdk-go@v0.13.0/pkg/cloudevents/generic#Codec) interface, featuring two essential methods: `Encode` for encoding the resource into cloudevents, and `Decode` for decoding cloudevents back into the resource. Refer to the [test/grpc_codec.go](../test/grpc_codec.go) for the example of the resource codec.

3. Define resource lister that implements the [generic.Lister](https://pkg.go.dev/open-cluster-management.io/sdk-go/pkg/cloudevents/generic@v0.13.0#Lister) interface, it is used to list the resource objects on the source when resyncing the resources between sources and agents, for example, a hub controller can list the resources from the resource informers, and a RESTful service can list its resources from a database. Refer to the [test/store.go](../test/store.go) for the example of the resource codec.

4. Define the resource status hash getter method - [generic.StatusHashGetter](https://pkg.go.dev/open-cluster-management.io/sdk-go/pkg/cloudevents/generic@v0.13.0#StatusHashGetter), this method will be used to calculate the resource status hash when resyncing the resource status between sources and agents. Refer to the [test/store.go](../test/store.go#L131) for the example of the resource codec.

5. Then it's ready to call the [CloudEventSourceClient](https://pkg.go.dev/open-cluster-management.io/sdk-go/pkg/cloudevents/generic@v0.13.0#NewCloudEventSourceClient) method to initialize the gRPC source client.

    ```golang
    import generic "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"

    // create the gRPC source client
    grpcSourceCloudEventsClient, err := generic.NewCloudEventSourceClient[*api.Resource](
        context.TODO(),
        grpcSourceOption,
        store,
        resourceStatusHashGetter,
        &ResourceCodec{},
    )
    ```

### Publish the Resource

To publish the resource with cloudevents format, you need to call the `Publish` method of the gRPC source client.

```golang
    // publish the resource in the cloudevents format
    grpcSourceCloudEventsClient.Publish(context.TODO(), types.CloudEventsType{
		  CloudEventsDataType: payload.ManifestEventDataType,
		  SubResource:         types.SubResourceSpec,
		  Action:              config.CreateRequestAction,
	}, res)
```

The `Publish` method takes three arguments:
- the context
- the cloudevents type - deinfe the cloudevent data type in codec implementation
- the resource - the resource you intend to publish, the codec will translate resource to and from cloudevents

see the below for an example of the resource:

```golang
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

### Subscribe to the Resource Status

To subscribe to the resource status, you need to call the `Subscribe` method of the gRPC source client with a callback function that handles the resource status.

```golang
    // start a go routine to sibscribe to the resources status
    grpcSourceCloudEventsClient.Subscribe(ctx, func(action types.ResourceAction, resource *api.Resource) error {
        // check the resource action and handle the resource status
        return nil
    })
```
