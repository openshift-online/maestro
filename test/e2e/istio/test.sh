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
export consumer_name=${CONSUMER_NAME:-$(cat "${PWD}/test/e2e/.consumer_name")}

# these kubeconfigs are used in this script to connect the server and agent clusters
server_kubeconfig=${SERVER_KUBECONFIG:-"${PWD}/test/e2e/.kubeconfig"}
agent_kubeconfig=${AGENT_KUBECONFIG:-"${PWD}/test/e2e/.kubeconfig"}

# these kubeconfigs are used in e2e test job to connect the server and agent clusters
server_in_cluster_kubeconfig=${SERVER_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/e2e/.in-cluster.kubeconfig"}
agent_in_cluster_kubeconfig=${AGENT_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/e2e/.in-cluster.kubeconfig"}

timeout=${TIMEOUT:-"30m"}

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
envsubst '${image} ${agent_namespace} ${consumer_name}' < ${PWD}/test/e2e/istio/job.yaml.template | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 5

echo "Waiting for job clusters-service/maestro-e2e-tests to complete (timeout: ${timeout})..."
kubectl --kubeconfig=$server_kubeconfig -n clusters-service wait --for=condition=complete --timeout=${timeout} job/maestro-e2e-tests 2>/dev/null && {
    echo "Job completed successfully"
    exit 0
}

echo "Job failed"
echo "-----------------------------"
kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs -l job-name=maestro-e2e-tests
exit 1
