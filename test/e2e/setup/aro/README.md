# Maestro ARO-HCP Env Setup

## Background

The idea of this directory is to provide means to create a development or testing environment that resemble the (future) production setup in a repeatable way. In order to do so, the creation of all infrastructure resources is based on bicep templates and parameter files.

## Prerequisites

* `az` version >= 2.60, `jq`, `make`, [kubelogin](https://azure.github.io/kubelogin/install.html), `kubectl` version >= 1.30, `helm`
* `az login` with service principal (azure AD user support is WIP)
* Register the needed [AFEC](https://aka.ms/afec) feature flags using `cd cluster && make feature-registration`
    * __NOTE:__ This will take awhile, you will have to wait until they're in a registered state.

## Cluster Creation Procedure

There are a few variants to chose from when creating an AKS cluster:

* Service Cluster: Public AKS cluster with optional params that can be modified to include all Azure resources needed to run a Service cluster
* Management Cluster: Public AKS cluster with optional params that can be modified to include all Azure resources needed to run a Management cluster

When creating a cluster, also supporting infrastructure is created, e.g. managed identities, permissions, databases, keyvaults, ...

### Create Service Cluster

Change those flags accordingly and then run the following command. Depending on the selected features, this may take a while:

  ```bash
  cd cluster
  AKSCONFIG=svc-cluster make cluster
  ```

### Create Management Cluster

A Management Cluster depends on certain resources found in the resource group of the Service Cluster. Therefore, a standalone Management Cluster can't be created right now and requires a Service Cluster

  ```bash
  cd cluster
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
   cd cluster
   AKSCONFIG=svc-cluster make clean
   ```

    (Replace `svc` with `mgmt` for management clusters)

## Deploy Maestro to AKS Clusters

### Maestro Server

> Make sure your `KUBECONFIG` points to the service cluster!!!

> The service cluster has no ingress. To interact with the services you deploy use `kubectl port-forward`

  ```bash
  cd maestro
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
  cd maestro
  AKSCONFIG=mgmt-cluster make deploy-agent
  ```

Then register it with the Maestro Server

Make sure your `KUBECONFIG` points to the service cluster, then run

  ```bash
  cd maestro
  AKSCONFIG=svc-cluster make register-agent
  ```
