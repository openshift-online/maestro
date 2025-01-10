#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

# the counts of agents that are running at one kind cluster
counts=${counts:-1}

# work dir
work_dir=${REPO_DIR}/_output/performance/aro
spoke_kube_dir=${work_dir}/clusters
agent_config_dir=${work_dir}/cert
agent_log_dir=${work_dir}/logs
pid_dir=${work_dir}/pids

mkdir -p ${agent_log_dir}
mkdir -p ${pid_dir}


# start agents
echo "Start ${counts} agents ..."
echo "The kind cluster ${spoke_kube_dir}/test.kubeconfig is used"

args="--agent-config-dir=${agent_config_dir}"
args="${args} --spoke-kubeconfig=${spoke_kube_dir}/test.kubeconfig"
args="${args} --cluster-begin-index=1"
args="${args} --cluster-counts=${counts}"

(exec "${REPO_DIR}"/maestroperf aro-hcp-spoke $args) &> ${agent_log_dir}/agents.log &
PERF_PID=$!
echo "$counts agents started: $PERF_PID"
touch $pid_dir/$PERF_PID
