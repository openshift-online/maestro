# Maestro ARO-HCP Env Setup

## Prerequisites

* `az` version >= 2.60, `jq`, `make`, [kubelogin](https://azure.github.io/kubelogin/install.html), `kubectl` version >= 1.30, `helm`
* `az login` with service principal (azure AD user support is WIP)

### Create Service Cluster

Change those flags accordingly and then run the following command. Depending on the selected features, this may take a while:

  ```bash
  AKSCONFIG=svc-cluster make cluster
  ```

### Create Management Cluster

A Management Cluster depends on certain resources found in the resource group of the Service Cluster. Therefore, a standalone Management Cluster can't be created right now and requires a Service Cluster

  ```bash
  AKSCONFIG=mgmt-cluster make cluster
  ```

### Access AKS Clusters

   ```bash
   AKSCONFIG=svc-cluster make aks.admin-access  # one time
   AKSCONFIG=svc-cluster make aks.kubeconfig
   AKSCONFIG=svc-cluster export KUBECONFIG=${HOME}/.kube/${AKSCONFIG}.kubeconfig
   kubectl get ns
   ```

    (Replace `svc` with `mgmt` for management clusters)

### Cleanup

Setting the correct `AKSCONFIG`, this will cleanup all resources created in Azure

   ```bash
   AKSCONFIG=svc-cluster make clean
   ```

    (Replace `svc` with `mgmt` for management clusters)

## Deploy Maestro to AKS Clusters

### Maestro Server

> Make sure your `KUBECONFIG` points to the service cluster!!!

> The service cluster has no ingress. To interact with the services you deploy use `kubectl port-forward`

  ```bash
  AKSCONFIG=svc-cluster make deploy-server
  ```

To validate, have a look at the `maestro` namespace on the service cluster. Some pod restarts are expected in the first 1 minute until the containerized DB is ready.

To access the HTTP and GRPC endpoints of maestro, run

  ```bash
  kubectl port-forward svc/maestro 8001:8000 -n maestro
  kubectl port-forward svc/maestro-grpc 8090 -n maestro
  ```

If you need to restart the maestro server during testing and don't want the port-forward process to be broken, you can install the kubectl relay plugin from [https://github.com/knight42/krelay](https://github.com/knight42/krelay) and perform the port forward using the following steps:


  ```bash
  kubectl relay svc/maestro 8001:8000 -n maestro
  kubectl relay svc/maestro-grpc 8090 -n maestro
  ```

## Maestro Agent

> Make sure your `KUBECONFIG` points to the management cluster!!!

First install the agent

  ```bash
  AKSCONFIG=mgmt-cluster make deploy-agent
  ```

Then register it with the Maestro Server

Make sure your `KUBECONFIG` points to the service cluster, then run

  ```bash
  cd maestro
  AKSCONFIG=svc-cluster make register-agent
  ```
