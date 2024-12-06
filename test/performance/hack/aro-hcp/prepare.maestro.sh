#!/usr/bin/env bash

ARO_HCP_REPO_PATH="$HOME/go/src/github.com/Azure/ARO-HCP"

pushd $ARO_HCP_REPO_PATH/dev-infrastructure
AKSCONFIG=svc-cluster make cluster
AKSCONFIG=svc-cluster make aks.kubeconfig
KUBECONFIG=${HOME}/.kube/svc-cluster.kubeconfig scripts/maestro-server.sh
popd
