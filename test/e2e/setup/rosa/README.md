## Setup Maestro in ROSA env

This demonstrates how to deploy the Maestro in ROSA env.

### Prerequisites

- Install the CLIs: `oc`, `rosa`, `aws` and `jq`
- Ensue your `aws` CLI is logined with your AWS account and your AWS account should have the permissions to operate AWS IoT and AWS RDS  PostgreSQL in your provided region
- Prepare two ROSA clusters, one is used as Service Cluster and the other is used as Management Cluster, e.g.

```sh
rosa create cluster --cluster-name=service --region=us-west-2 --sts --mode=auto
rosa create cluster --cluster-name=management --region=us-west-2 --sts --mode=auto
```

### Setup Maestro server in your Service Cluster

```sh
export REGION="<your_rosa_cluster_region>" # e.g. us-west-2
export CLUSTER_ID="<your_rosa_cluster_id_or_name>" # e.g. service
export KUBECONFIG="<your_service_cluster_kubeconfig>"

make rosa/setup-maestro
```

This will
- Create AWS IoT client certs and policy for Maestro server in your region
- Create AWS RDS PostgreSQL for Maestro server in your region
- Deploy the Maestro server on the given cluster

After the Maestro server is deployed, you can run following commands to start the Maestro RESTful service and GRPC service in your local host

```sh
oc port-forward svc/maestro 8000 -n maestro
oc port-forward svc/maestro-grpc 8090 -n maestro
```

Then create a consumer in the Maestro, e.g.

```sh
curl -s -X POST -H "Content-Type: application/json" http://127.0.0.1:8000/api/maestro/v1/consumers -d '{"name": "management"}'
```

### Setup Maestro agent in your Management Cluster

```sh
export REGION="<your_rosa_cluster_region>" # e.g. us-west-2
export CONSUMER_ID="<your_created_consumer_id_or_name>" # e.g. management
export KUBECONFIG="<your_management_cluster_kubeconfig>"

make rosa/setup-agent
```

This will
- Create AWS IoT client certs and policy for Maestro agent in your region
- Deploy the Maestro agent on the given cluster

### Cleanup

```sh
export REGION="<your_cluster_region>"

make rosa/teardown

# delete your rosa clusters, e.g.
rosa delete cluster --cluster=service
rosa delete cluster --cluster=management
```

## Run Maestro e2e on a ROSA cluster

### Prepare

1. Install the following CLIs `oc`, `rosa`, `aws`, `jq` and [`krelay` plugin](https://github.com/knight42/krelay)
2. Create a rosa cluster

```sh
rosa create cluster --cluster-name=maestro-e2e --region=us-west-2 --sts --mode=auto
```

### Run e2e

```sh
export KUBECONFIG="<your_rosa_cluster_kubeconfig>"
export REGION="<your_rosa_cluster_region>"
export CLUSTER_ID="<your_rosa_cluster_name_or_id>"

make rosa/e2e-test
```

### Cleanup

```sh
export REGION="<your_rosa_cluster_region>"

make rosa/teardown

# delete your rosa clusters, e.g.
rosa delete cluster --cluster=maestro-e2e 
```
