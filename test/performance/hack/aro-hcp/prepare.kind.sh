#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

ocm_base="https://raw.githubusercontent.com/open-cluster-management-io/api/v0.14.0"

# work dir
crds_dir=${REPO_DIR}/test/performance/hack/crds
work_dir=${REPO_DIR}/_output/performance/aro
spoke_kube_dir=${work_dir}/clusters

mkdir -p ${spoke_kube_dir}

# prepare kind clusters
echo "Start kind cluster aro-test ..."
kind create cluster --name "aro-test" --kubeconfig "${spoke_kube_dir}/test.kubeconfig"
kubectl --kubeconfig ${spoke_kube_dir}/test.kubeconfig apply -f "${ocm_base}/cluster/v1/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml"
kubectl --kubeconfig ${spoke_kube_dir}/test.kubeconfig apply -f "${ocm_base}/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml"
kubectl --kubeconfig ${spoke_kube_dir}/test.kubeconfig apply -f "${ocm_base}/work/v1/0000_01_work.open-cluster-management.io_appliedmanifestworks.crd.yaml"
