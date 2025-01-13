#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

index=${index:-1}
counts=${counts:-1}

# work dir
work_dir=${REPO_DIR}/_output/performance/aro
spoke_kube_dir=${work_dir}/clusters
log_dir=${work_dir}/logs
pid_dir=${work_dir}/pids


args="--spoke-kubeconfig=${spoke_kube_dir}/test.kubeconfig"
args="${args} --clusters-index=$index"
args="${args} --clusters-counts=$counts"

(exec "${REPO_DIR}"/maestroperf aro-hcp-watch $args) &> ${log_dir}/watcher.log &
PERF_PID=$!
echo "watcher started: $PERF_PID"
touch $pid_dir/$PERF_PID
