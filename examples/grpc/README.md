# Resource Bundle CURD with gRPC Client

## Preparation

Do the port-forward the maestro-grpc service:

```shell
$ kubectl -n maestro port-forward svc/maestro-grpc 8090 &
```

## How

1. Set the source ID for the manifestwork client and consumer name:

```shell
$ export SOURCE_ID=grpc
$ export CONSUMER_NAME=cluster1
```

2. Create a resource bundle:

```shell
# create
$ go run ./grpcclient.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -cloudevent-file ./cloudevent.json
```

Note: If your gRPC server enable authentication and authorization, you'll need to provide the CA file for the server and the client's token. For example, after setting up Maestro with `make e2e-test/setup`, you can retrieve the gRPC server's CA, client certificate, key, and token using the following command:

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
$ go run ./grpcclient.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -cloudevent-file ./cloudevent.json -grpc-server-tls=true -grpc-server=127.0.0.1:30090 -grpc-server-ca-file=/tmp/grpc-server-ca.crt -grpc-client-token-file=/tmp/grpc-client-token
```

2. Update the resource bundle:

```shell
$ go run ./grpcclient.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -cloudevent-file ./cloudevent-update.json -grpc-server-tls=true -grpc-server=127.0.0.1:30090 -grpc-server-ca-file=/tmp/grpc-server-ca.crt -grpc-client-token-file=/tmp/grpc-client-token
```

3. Delete the resource bundle:

```shell
$ go run ./grpcclient.go -source=$SOURCE_ID -consumer-name=$CONSUMER_NAME -cloudevent-file ./cloudevent-delete.json -grpc-server-tls=true -grpc-server=127.0.0.1:30090 -grpc-server-ca-file=/tmp/grpc-server-ca.crt -grpc-client-token-file=/tmp/grpc-client-token
```
