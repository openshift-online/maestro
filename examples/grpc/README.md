# CURD Resource/Bundle with gRPC Client

## Preparation

1. Enable gRPC server by passing `--enable-grpc-server=true` to the maestro server start command, for example:

```shell
$ oc -n maestro patch deploy/maestro --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/-", "value": "--enable-grpc-server=true"}]'
```

2. Port-forward the gRPC service to your local machine, for example:

```shell
$ oc -n maestro port-forward svc/maestro-grpc 8090 &
```

## Operate Resource Bundle with gRPC client

```shell
# create
go run ./grpcclient.go -grpc_server localhost:8090 -cloudevents_json_file ./cloudevent-bundle.json

# update
go run ./grpcclient.go -grpc_server localhost:8090 -cloudevents_json_file ./cloudevent-bundle-update.json

# delete
go run ./grpcclient.go -grpc_server localhost:8090 -cloudevents_json_file ./cloudevent-bundle-delete.json
```
