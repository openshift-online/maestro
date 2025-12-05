# Resource Bundle CURD with Manifestwork Client

You can CURD resource bundle with a manifestwork client, just like CURD manifestwork in a Kubernetes cluster. The difference is that this client is built on the cloudevents [sdk-go](https://github.com/open-cluster-management-io/sdk-go/tree/main/pkg/cloudevents#work-clients). As an example, check [main.go](./main.go) for how to create the manifestwork client working in maestro.

## Preparation

Do the port-forward for the maestro and maestro-grpc services:

```shell
$ kubectl -n maestro port-forward svc/maestro 8000 &
$ kubectl -n maestro port-forward svc/maestro-grpc 8090 &
```

## How

1. Set the source ID for the manifestwork client and consumer name:

```shell
$ export SOURCE_ID=grpc
$ export CONSUMER_NAME=cluster1
```

2. Review and adjust the manifestwork [JSON](./manifestwork.json)

Before creating a resource bundle with manifestwork client, review the manifestwork JSON file used for creation. You can modify it as needed or leave it unchanged. Keep in mind that while most ManifestWork [API](https://github.com/open-cluster-management-io/api/blob/main/work/v1/types.go) options are supported, not all are guaranteed to work in maestro. Pay special attention to the following:

- `Deletion options`: Only `Foreground` and `Orphan` propagation policies have been verified. Others may not work.
- `Update strategy`: `ServerSideApply`, `Update`, `CreateOnly`, and `ReadOnly` are verified. Others are not guaranteed.

```shell
$ go run ./main.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -manifestwork_file=./manifestwork.json -action=create
```
If your maestro server is running with HTTP, you need to pass `-maestro-server=http://127.0.0.1:8000` to the command above.

Note: If your gRPC server enable authentication and authorization, you'll need to provide the CA file for the server and the client's token. For example, after setting up Maestro with `make test-env`, you can retrieve the gRPC server's CA, client certificate, key, and token using the following command:

```shell
kubectl -n maestro get secret maestro-grpc-cert -o jsonpath="{.data.ca\.crt}" | base64 -d > /tmp/grpc-server-ca.crt
kubectl -n maestro get secret maestro-grpc-cert -o jsonpath="{.data.client\.crt}" | base64 -d > /tmp/grpc-client-cert.crt
kubectl -n maestro get secret maestro-grpc-cert -o jsonpath="{.data.client\.key}" | base64 -d > /tmp/grpc-client-cert.key
kubectl -n maestro get secret grpc-client-token  -o jsonpath="{.data.token}" | base64 -d > /tmp/grpc-client-token
```

You also need to create a cluster role to grant publish & subscribe permissions to the client by running this command:

```shell
$ cat << EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: grpc-pub-sub
rules:
- nonResourceURLs:
  - /sources/${SOURCE_ID}
  verbs:
  - pub
  - sub
EOF
```

then you can create a resource bundle with the following command:

```shell
$ go run ./main.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -manifestwork_file=./manifestwork.json -maestro-server=https://127.0.0.1:30080 -grpc-server=127.0.0.1:30090 -grpc-server-ca-file=/tmp/grpc-server-ca.crt -grpc-client-token-file=/tmp/grpc-client-token -action=create
```

4. Delete the resource bundle:

```shell
$ go run ./main.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -manifestwork_file=./manifestwork.json -maestro-server=https://127.0.0.1:30080 -grpc-server=127.0.0.1:30090 -grpc-server-ca-file=/tmp/grpc-server-ca.crt -grpc-client-token-file=/tmp/grpc-client-token -action=delete
```
