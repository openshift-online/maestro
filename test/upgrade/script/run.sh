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

timeout=${timeout:-"5m"}

export image=${IMAGE:-"image-registry.testing/maestro/maestro-e2e:latest"}
export consumer_name=${CONSUMER_NAME:-$(cat "${PWD}/test/_output/.consumer_name")}
export service_account_name=${SERVICE_ACCOUNT_NAME:-"default"}

export server_kubeconfig=${SERVER_KUBECONFIG:-"${PWD}/test/_output/.kubeconfig"}
export agent_in_cluster_kubeconfig=${AGENT_IN_CLUSTER_KUBECONFIG:-"${PWD}/test/_output/.in-cluster.kubeconfig"}

if [ ! -f "$server_kubeconfig" ]; then
  echo "ERROR: server_kubeconfig not found at $server_kubeconfig" >&2
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

# ensure the maestro server is ready
kubectl --kubeconfig=$server_kubeconfig wait deploy/maestro -n maestro --for condition=Available=True --timeout=300s

# ensure the clusters-service namespace exists
kubectl --kubeconfig=$server_kubeconfig create namespace clusters-service || true

# deploy the mock work-server
envsubst '${image} ${consumer_name} ${service_account_name}' < ${PWD}/test/upgrade/script/resources/deployment.yaml | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 30 # wait for deploy is created
if ! kubectl --kubeconfig=$server_kubeconfig -n clusters-service wait deploy/workserver --for condition=Available=True --timeout=300s; then
    echo "ERROR: workserver deployment failed to become available, describing pod..."
    kubectl --kubeconfig=$server_kubeconfig -n clusters-service describe pod -l app=workserver
    exit 1
fi

# cleanup the test jobs
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete jobs --all --ignore-not-found

# create kubeconfigs for test job
kubectl --kubeconfig=$server_kubeconfig -n clusters-service delete secret maestro-upgrade-test-kubeconfig --ignore-not-found
kubectl --kubeconfig=$server_kubeconfig -n clusters-service create secret generic maestro-upgrade-test-kubeconfig \
    --from-file=agent.kubeconfig=$agent_in_cluster_kubeconfig

# deploy the upgrade test job
envsubst '${image} ${service_account_name}' < ${PWD}/test/upgrade/script/resources/job.yaml | kubectl --kubeconfig=$server_kubeconfig apply -f -

sleep 10 # wait for job created

echo "Waiting for job clusters-service/maestro-upgrade-tests to complete (timeout: ${timeout})..."
if kubectl --kubeconfig=$server_kubeconfig -n clusters-service wait --for=condition=complete --timeout=${timeout} job/maestro-upgrade-tests 2>/dev/null; then
  # Check if job actually succeeded
  succeeded=$(kubectl --kubeconfig=$server_kubeconfig -n clusters-service get job maestro-upgrade-tests -o jsonpath='{.status.succeeded}')
  if [ "$succeeded" = "1" ]; then
    echo "Job completed successfully"
    kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs jobs/maestro-upgrade-tests
    exit 0
  fi
fi

echo "Job clusters-service/maestro-upgrade-tests failed"
kubectl --kubeconfig=$server_kubeconfig -n clusters-service logs jobs/maestro-upgrade-tests
exit 1
