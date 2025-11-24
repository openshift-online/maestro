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

timeout=${timeout:-"30m"}

export image=${IMAGE:-"image-registry.testing/maestro/maestro-e2e:latest"}
export consumer_name=${CONSUMER_NAME:-$(cat "${PWD}/test/e2e/.consumer_name")}
export service_account_name=${SERVICE_ACCOUNT_NAME:-"default"}

export server_kubeconfig=${SERVER_KUBECONFIG:-"${PWD}/test/e2e/.kubeconfig"}
export agent_in_cluster_kubeconfig=${AGENT_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/e2e/.in-cluster.kubeconfig"}

# ensure the clusters-service namespace exists
kubectl --kubeconfig=$server_kubeconfig create namespace clusters-service || true

# deploy the mock work-server
envsubst '${image} ${consumer_name} ${service_account_name}' < ${PWD}/test/upgrade/script/resources/deployment.yaml | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 10

# cleanup the test jobs
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete jobs --all --ignore-not-found

# create kubeconfigs for test job
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete secret maestro-upgrade-test-kubeconfig --ignore-not-found
kubectl --kubeconfig=$server_kubeconfig -n clusters-service create secret generic maestro-upgrade-test-kubeconfig \
    --from-file=agent.kubeconfig=$agent_in_cluster_kubeconfig

# deploy the upgrade test job
envsubst '${image} ${service_account_name}' < ${PWD}/test/upgrade/script/resources/job.yaml | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 10

# Wait for pod to exist and be running
echo "Waiting for job clusters-service/maestro-upgrade-tests to complete (timeout: ${timeout})..."
kubectl --kubeconfig=$server_kubeconfig -n clusters-service wait --for=condition=complete --timeout=${timeout} job/maestro-upgrade-tests 2>/dev/null && {
    echo "Job completed successfully"
    kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs -l app=maestro-upgrade
    exit 0
}

echo "Job failed"
echo "-----------------------------"
kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs -l app=maestro-upgrade
exit 1
