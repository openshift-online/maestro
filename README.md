[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/openshift-online/maestro)

# Maestro

Maestro is a system to leverage [CloudEvents](https://cloudevents.io/) to transport Kubernetes resources to the target clusters, and then transport the resource status back. The resources are stored in a database and the status is updated in the database as well. The system is composed of two parts: the Maestro server and the Maestro agent.
- The Maestro server is responsible for storing the resources and their status. And sending the resources to the message broker in the CloudEvents format. The Maestro server provides Resful APIs and gRPC APIs to manage the resources.
- Maestro agent is responsible for receiving the resources and applying them to the target clusters. And reporting back the status of the resources.

## Architecture

Taking MQTT as an example:

![Maestro Architecture](./arch.png)

## Why

Maestro was created to address several critical challenges associated with managing Kubernetes clusters at scale.
Traditional Kubernetes native and custom resources rely heavily on etcd,
which has well-known limitations in terms of scalability and performance.
As the number of clusters increases, the strain on etcd can lead to significant issues,
making it difficult to achieve high-scale Kubernetes cluster management.
To overcome these challenges, Maestro introduces a system that leverages a traditional relational database for storing relevant data,
providing a more scalable and efficient solution.

### Motivations

**Scalability**: Kubernetes-based solutions often face scalability issues due to the limitations of etcd.
Maestro aims to overcome these issues by decoupling resource management from the etcd store,
allowing for more efficient handling of a large number of clusters.
This approach supports the goal of managing up to 200,000+ clusters without linear scaling of the infrastructure.

**Cost Effectiveness**: Running a central orchestrator in a Kubernetes-based environment can be costly.
Maestro reduces infrastructure and maintenance costs by leveraging a relational database and optimized content delivery mechanisms.

**Improve Feedback Loop**: Traditional Kubernetes solutions have limitations in the feedback loop,
making it difficult to observe the state of resources effectively.
Maestro addresses this by providing a robust feedback loop mechanism,
ensuring that resource status is continuously updated and monitored.

**Improve Security Architecture**: Maestro enhances security by eliminating the need for kubeconfigs,
reducing the need for direct access to clusters.

## Run in Local Environment

### Make a build, run postgres and mqtt broker

```shell

# 1. build the project

$ go install gotest.tools/gotestsum@latest  
$ make binary

# 2. run a postgres database locally in docker 

$ make db/setup
$ make db/login
        
    root@f076ddf94520:/# psql -h localhost -U maestro maestro
    psql (14.4 (Debian 14.4-1.pgdg110+1))
    Type "help" for help.
    
    maestro=# \dt
    Did not find any relations.

# 3. run a mqtt broker locally in docker

$ make mqtt/setup
```

### Run database migrations

The initial migration will create the base data model as well as providing a way to add future migrations.

```shell

# Run migrations
$ ./maestro migration

# Verify they ran in the database
$ make db/login

podman exec -it psql-maestro bash -c "psql -h localhost -U maestro maestro"
psql (17.2 (Debian 17.2-1.pgdg120+1))
Type "help" for help.

maestro=# \dt
                 List of relations
 Schema |       Name       | Type  |  Owner
--------+------------------+-------+---------
 public | consumers        | table | maestro
 public | event_instances  | table | maestro
 public | events           | table | maestro
 public | migrations       | table | maestro
 public | resources        | table | maestro
 public | server_instances | table | maestro
 public | status_events    | table | maestro
(7 rows)
```

### Running the Service

```shell
$ make run
```

#### List the consumers

This will be empty if no consumer is ever created

```shell
$ curl http://localhost:8000/api/maestro/v1/consumers
{
  "items": [],
  "kind": "ConsumerList",
  "page": 1,
  "size": 0,
  "total": 0
}
```

#### Create a consumer:

```shell
$ curl -X POST -H "Content-Type: application/json" \
    http://localhost:8000/api/maestro/v1/consumers \
    -d '{
    "name": "cluster1"
  }'
```

#### Get the consumer:

```shell
$ curl http://localhost:8000/api/maestro/v1/consumers
{
  "items": [
    {
      "created_at": "2025-11-26T17:20:14.535108+08:00",
      "href": "/api/maestro/v1/consumers/219ac81e-cd5c-4d22-9e03-e4eaa4f55aa1",
      "id": "219ac81e-cd5c-4d22-9e03-e4eaa4f55aa1",
      "kind": "Consumer",
      "name": "cluster1",
      "updated_at": "2025-11-26T17:20:14.535108+08:00"
    }
  ],
  "kind": "ConsumerList",
  "page": 1,
  "size": 1,
  "total": 1
}
```

#### Create a resource bundle

You can create a resource bundle with manifestwork client based on grpc, check the [document](./examples/manifestworkclient/client/README.md) for more details.

#### List the resource bundle

```shell
curl http://localhost:8000/api/maestro/v1/resource-bundles
{
  "items": [
    {
      "consumer_name": "cluster1",
      "created_at": "2025-11-26T17:23:08.964138+08:00",
      "delete_option": {
        "propagationPolicy": "Foreground"
      },
      "href": "/api/maestro/v1/resource-bundles/916777c0-0950-56c5-bb78-c884a111303b",
      "id": "916777c0-0950-56c5-bb78-c884a111303b",
      "kind": "ResourceBundle",
      "manifest_configs": [
        {
          "feedbackRules": [
            {
              "jsonPaths": [
                {
                  "name": "status",
                  "path": ".status"
                }
              ],
              "type": "JSONPaths"
            }
          ],
          "resourceIdentifier": {
            "group": "apps",
            "name": "nginx",
            "namespace": "default",
            "resource": "deployments"
          },
          "updateStrategy": {
            "type": "ServerSideApply"
          }
        }
      ],
      "manifests": [
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "metadata": {
            "name": "nginx",
            "namespace": "default"
          },
          "spec": {
            "replicas": 1,
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
                    "imagePullPolicy": "IfNotPresent",
                    "name": "nginx"
                  }
                ]
              }
            }
          }
        }
      ],
      "metadata": {
        "creationTimestamp": "2025-11-26T17:23:08+08:00",
        "name": "nginx-work",
        "namespace": "cluster1",
        "resourceVersion": "0",
        "uid": "916777c0-0950-56c5-bb78-c884a111303b"
      },
      "name": "916777c0-0950-56c5-bb78-c884a111303b",
      "updated_at": "2025-11-26T17:23:08.964138+08:00",
      "version": 1
    }
  ],
  "kind": "ResourceBundleList",
  "page": 1,
  "size": 1,
  "total": 1
}
```

## Run in OpenShift

If you are using an OpenShift cluster in the cloud, you need to export Kubeconfig to point to your cluster and skip the CRC login step. If you are using CodeReady Containers (CRC) locally, you need to login to the CRC cluster first.

### Log into CRC

Use OpenShift Local to deploy to a local openshift cluster. Be sure to have CRC running locally:

```shell
$ crc status
CRC VM:          Running
OpenShift:       Running (v4.13.12)
RAM Usage:       7.709GB of 30.79GB
Disk Usage:      23.75GB of 32.68GB (Inside the CRC VM)
Cache Usage:     37.62GB
```

```shell
$ make crc/login
Logging into CRC
Logged into "https://api.crc.testing:6443" as "kubeadmin" using existing credentials.

You have access to 66 projects, the list has been suppressed. You can list all projects with 'oc projects'

Using project "default".
Login Succeeded!
```

### Set external_apps_domain

You need to set the `external_apps_domain` environment variable to point your cluster.
```shell
$ export external_apps_domain=`oc -n openshift-ingress-operator get ingresscontroller default -o jsonpath='{.status.domain}'`
```

### Deploy Maestro

If you want to push the image to your OpenShift cluster default registry and then deploy it to the cluster. You need to follow [this document](https://docs.openshift.com/container-platform/4.13/registry/securing-exposing-registry.html) to expose a default registry manually and login into the registry with podman. Then run `make push` to push the image to the registry.

If you want to use the existing image, you can run `make retrieve-image` to retrieve the image info and run `source .image-env` to set the image environment variables.

```shell
$ make deploy

$ oc get pod -n "maestro-$USER"
NAME                            READY   STATUS      RESTARTS   AGE
maestro-85c847764-4xdt6         1/1     Running     0          62s
maestro-db-5d4c4679f5-r92vg     1/1     Running     0          61s
maestro-mqtt-6cb7bdf46c-kcczm   1/1     Running     0          63s
```

### Create a Consumer

```shell
$ curl -k -X POST -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    https://maestro.${external_apps_domain}/api/maestro/v1/consumers \
    -d '{
    "name": "cluster1"
  }'
```
You should get a response like this:

```shell
{
  "created_at":"2023-12-08T11:35:08.557450505Z",
  "href":"/api/maestro/v1/consumers/3f28c601-5028-47f4-9264-5cc43f2f27fb",
  "id":"3f28c601-5028-47f4-9264-5cc43f2f27fb",
  "kind":"Consumer",
  "name":"cluster1",
  "updated_at":"2023-12-08T11:35:08.557450505Z"
}
```

### Deploy Maestro Agent

```shell
$ export consumer_name=cluster1
$ make deploy-agent
$ oc get pod -n "maestro-agent-$USER"
NAME                             READY   STATUS    RESTARTS   AGE
maestro-agent-5dc9f5b4bf-8jcvq   1/1     Running   0          13s
```

Now you can create a resource bundle with manifestwork client based on grpc, check the [document](./examples/manifestworkclient/client/README.md) for more details.

## Run in KinD Cluster

You can also run the maestro in a KinD cluster locally. The simplest way is to use the provided script to create a KinD cluster and deploy the maestro in the cluster. It creates a KinD cluster with name `maestro`, and deploys the maestro server and agent in the cluster.

```shell
$ make test-env
```
The Kubeconfig of the KinD cluster is in `./test/_output/.kubeconfig`.
```shell
$ export KUBECONFIG=$(pwd)/test/_output/.kubeconfig
$ kubectl -n maestro get svc
NAME                  TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
maestro               NodePort    10.96.143.3     <none>        8000:30080/TCP   7m42s
maestro-db            ClusterIP   10.96.53.61     <none>        5432/TCP         7m59s
maestro-grpc          NodePort    10.96.114.161   <none>        8090:30090/TCP   7m42s
maestro-healthcheck   ClusterIP   10.96.67.145    <none>        8083/TCP         7m42s
maestro-metrics       ClusterIP   10.96.201.253   <none>        8080/TCP         7m42s
maestro-mqtt          ClusterIP   10.96.241.85    <none>        1883/TCP         7m59s
maestro-mqtt-agent    ClusterIP   10.96.215.21    <none>        1883/TCP         7m59s
maestro-mqtt-server   ClusterIP   10.96.72.129    <none>        1883/TCP         7m59s
$ kubectl get pods -n maestro
NAME                             READY   STATUS    RESTARTS   AGE
maestro-85c847764-4xdt6          1/1     Running   0          5m
maestro-db-65f57d978c-c68        1/1     Running   0          5m
maestro-mqtt-6cb7bdf46c-kcczm    1/1     Running   0          5m
$ kubectl get pods -n maestro-agent
NAME                             READY   STATUS    RESTARTS   AGE
maestro-agent-5dc9f5b4bf-8jcvq   1/1     Running   0          3m
```
