#!/bin/bash -ex
#
# Copyright (c) 2023 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

export image=${IMAGE:-"image-registry.testing/maestro/maestro-e2e:latest"}
export agent_namespace=${AGENT_NAMESPACE:-"maestro-agent"}
export consumer_name=${CONSUMER_NAME:-$(cat "${PWD}/test/_output/.consumer_name")}
export service_account_name=${SERVICE_ACCOUNT_NAME:-"default"}

# these kubeconfigs are used in this script to connect the server and agent clusters
server_kubeconfig=${SERVER_KUBECONFIG:-"${PWD}/test/_output/.kubeconfig"}
agent_kubeconfig=${AGENT_KUBECONFIG:-"${PWD}/test/_output/.kubeconfig"}

# these kubeconfigs are used in e2e test job to connect the server and agent clusters
server_in_cluster_kubeconfig=${SERVER_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/_output/.in-cluster.kubeconfig"}
agent_in_cluster_kubeconfig=${AGENT_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/_output/.in-cluster.kubeconfig"}

timeout=${TIMEOUT:-"30m"}

if [ ! -f "$server_kubeconfig" ]; then
   echo "ERROR: server_kubeconfig not found at $server_kubeconfig" >&2
    exit 1
fi

if [ ! -f "$agent_kubeconfig" ]; then
   echo "ERROR: agent_kubeconfig not found at $agent_kubeconfig" >&2
    exit 1
fi

if [ ! -f "$server_in_cluster_kubeconfig" ]; then
    echo "ERROR: server_in_cluster_kubeconfig not found at $server_in_cluster_kubeconfig" >&2
    exit 1
fi

if [ ! -f "$agent_in_cluster_kubeconfig" ]; then
    echo "ERROR: agent_in_cluster_kubeconfig not found at $agent_in_cluster_kubeconfig" >&2
    exit 1
fi

if [ -z "$consumer_name" ]; then
    echo "ERROR: consumer_name not found in test/_output/.consumer_name" >&2
    exit 1
fi

# ensure the maestro server and agent are ready
kubectl --kubeconfig=$server_kubeconfig wait deploy/maestro -n maestro --for condition=Available=True --timeout=300s
kubectl --kubeconfig=$agent_kubeconfig wait deploy/maestro-agent -n ${agent_namespace} --for condition=Available=True --timeout=300s

# ensure the clusters-service namespace exists
kubectl --kubeconfig=$server_kubeconfig create namespace clusters-service || true

# cleanup the e2e test jobs
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete jobs --all --ignore-not-found

# create kubeconfigs for e2e test job
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete secret maestro-e2e-kubeconfig --ignore-not-found
kubectl --kubeconfig=$server_kubeconfig -n clusters-service create secret generic maestro-e2e-kubeconfig \
    --from-file=server.kubeconfig=$server_in_cluster_kubeconfig \
    --from-file=agent.kubeconfig=$agent_in_cluster_kubeconfig

# deploy the e2e test job
envsubst '${image} ${agent_namespace} ${consumer_name} ${service_account_name}' < ${PWD}/test/e2e/istio/job.yaml.template | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 10 # wait for job created

echo "Waiting for job clusters-service/maestro-e2e-tests to complete (timeout: ${timeout})..."
if kubectl --kubeconfig=$server_kubeconfig -n clusters-service wait --for=condition=complete --timeout=${timeout} job/maestro-e2e-tests 2>/dev/null; then
  # Check if job actually succeeded
  succeeded=$(kubectl --kubeconfig=$server_kubeconfig -n clusters-service get job maestro-e2e-tests -o jsonpath='{.status.succeeded}')
  if [ "$succeeded" = "1" ]; then
    echo "Job completed successfully"
    kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs jobs/maestro-e2e-tests
    exit 0
  fi
fi

echo "Job clusters-service/maestro-e2e-tests failed"
kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs jobs/maestro-e2e-tests
exit 1
