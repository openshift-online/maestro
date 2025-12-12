# Overview 

Maestro leverages [CloudEvents](https://cloudevents.io/) to transport Kubernetes resources to target clusters and relay the resource status back. These resources and their statuses are stored and updated in a database. The system is composed of two main parts: the Maestro server and the Maestro agent.

The Maestro Server includes various components to fulfill its functions, as illustrated in the diagram below.

![maestro-overview](./images/maestro-overview.jpg)

## Maestro Resource Flow

### gRPC

1. [Resource create flow with gRPC](https://swimlanes.io/#hZBBDoIwEEX3PcVcwAuwMNGC0QUJQi9QYYKNTWumBa8vBayCJq6aTP+beflCeY0J5BKdJwslOttRjcAJpUc4aPuAXkloy4IzxsIDXCs0HjbbiI3jCqlHSr52cG27BrJ+YBj7QYRFkQkjVQ9GM/z6YGwdWWCptAkUSE45/556C+n+DxkPnoxD8kDRvsx2IgOcvLk1g7bWg24ujWwn7WqOjoUkcJSm0WtykRlLOwsBe7K3UFbRXbRy1w+fO9ZTZWNjTw==)

    ![maestro-resource-create-flow-grpc](./images/maestro-resource-create-flow-grpc.png)

2. [Resource patch flow with gRPC](https://swimlanes.io/#hZDRDoIgGIXveYr/BXoBLtpKbXXhRmoPQPpPWQwcoL1+KEVpbV2xwfkO304lnEQKOUfrjIYCrR5MjcC4qzs4SH2HUXBoC5YQQqYDEilQOdhsIzVfl2hGNPRdcekb7tDH9dBANnqGkB/EVBSZ6UrUXugJvx4IWUcWWMo1BYbGCuu+BJyGdP+nIP57UhaNAxM7WLqrMsCgn2jl7aX01jlXvA32ZYiGXSgcuWrkmlxk5u3OVQV7o2/TZmy4SmG7D58e67DcPNwD)

    ![maestro-resource-patch-flow-grpc](./images/maestro-resource-patch-flow-grpc.png)

3. [Resource delete flow with gRPC](https://swimlanes.io/#hVBbDoIwEPzvKfYCXoAPPwSMJpoocIEKKzauXbMteH2BxvpM/GoynZmdmcp4wgS2Gp0XhgIdd1IjZEjoEZbEN+iNhrbYpUqp8YGUDFoPs3mUTXCJ0qMkXx4pcddA3g8apX4oRqOoGSFTT4nk/IS1C27Gtkp9kt8MMs0JlHz0j/Px5yh8gWzxRx8DrK1D8SDRON/kVQ4YeqRshxpEQ/yttroNNcpADQMlsNK2oU/lG2cacV9VsBA+j+PtugMZd3rJc8UampclpyHv)

    ![maestro-resource-delete-flow-grpc](./images/maestro-resource-delete-flow-grpc.png)

## Maestro Resource Status Flow


[maestro-resource-status-flow](https://swimlanes.io/#lVTLUuNADLzPV+gDeNw5bBXYBraKRxbCeWtia+MphhmvpCG/v5rYJinHhMU3S61WS2pbnHi8gHuLLBThCTkmqhGexUpiuPZxY4yp3jEIXFG0TW1ZkOD0BxwEL3IVCUiE1RgFGhl5y2jM2OrmaVHAM9J7zzYT3uPjHrfOScK/SbHGDBqLGLTQ+57nIDiybCK9KkTJWhsaPwr6jXmMnaxF8owzuvbjA+XnRYOI0nFnpW53Ig4SxjxE0QNcR4L7X8slbJy0wK0lbIDTimtynbgY9KXrIskJJG0lLUKIp7HjYQxoPijP4FHTtHGMO/ABqm9Ux8BOb6eHbC23LqzzghQl5FZJPupS11hBzklKIWTc2zC7C4oJNfLZkX30Fii80066i36uFWa+qeWUJe9BffU6WzzFFz6mpjfjtw5Solr2zQUE9weKRJSX8HMYRi8Qk2/gtrfKpOd33fKy3d55iV577siUG2pCTcHLorxcVudldVctq4kz/9fmt3O2ni2eV3n19Tc7HXfmF3CEZfYvMlKv97/7T0myYbZIHi1ESjx5/gE=)

![maestro-resource-status-flow](./images/maestro-resource-status-flow.png)

## Maestro Resource Data Flow

### Maestro Publish Resource with MQTT

![maestro-mqtt-pub-dataflow](./images/maestro-mqtt-pub-dataflow.png)

### Maestro Subscribe Resource Status with MQTT

![maestro-mqtt-sub-dataflow](./images/maestro-mqtt-sub-dataflow.png)
