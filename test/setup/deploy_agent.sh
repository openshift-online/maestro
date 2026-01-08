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

export image_tag=${image_tag:-"latest"}
export external_image_registry=${external_image_registry:-"image-registry.testing"}
export internal_image_registry=${internal_image_registry:-"image-registry.testing"}

export namespace="maestro"
export agent_namespace="maestro-agent"

export KUBECONFIG=${PWD}/test/_output/.kubeconfig

restapi_endpoint=$(cat ${PWD}/test/_output/.external_restapi_endpoint)
if [ -z "$restapi_endpoint" ]; then
  echo "ERROR: REST API endpoint not found in test/_output/.external_restapi_endpoint" >&2
  exit 1
fi

# Create a consumer and the consumer id will be used as the consumer name
if [ ! -f "${PWD}/test/_output/.consumer_name" ]; then
  response=$(curl -s -k -X POST -H "Content-Type: application/json" "${restapi_endpoint}/api/maestro/v1/consumers" -d '{}')
  consumer_name=$(echo "$response" | jq -r '.id')
  if [ -z "$consumer_name" ] || [ "$consumer_name" = "null" ]; then
    echo "Error: Failed to create consumer" >&2
    exit 1
  fi
  echo "$consumer_name" > ${PWD}/test/_output/.consumer_name
fi
consumer_name=$(cat ${PWD}/test/_output/.consumer_name)
export consumer_name
export mqtt_user=""
export mqtt_password_file="/dev/null"
export mqtt_root_cert="/secrets/mqtt-certs/ca.crt"
export mqtt_client_cert="/secrets/mqtt-certs/client.crt"
export mqtt_client_key="/secrets/mqtt-certs/client.key"
export pubsub_host="maestro-pubsub.${namespace}"
export pubsub_port="8085"
export pubsub_project_id="maestro-test"
# crank the client certificate refresh interval for cert rotation test
export broker_client_cert_refresh_duration=5s

# Initialize Pub/Sub agent subscriptions if using pubsub broker
msg_broker=${MESSAGE_DRIVER_TYPE:-"mqtt"}
if [ "$msg_broker" = "pubsub" ]; then
  echo "Initializing Pub/Sub agent subscriptions for consumer: ${consumer_name}..."

  # Create agent-specific subscriptions using the template
  oc process \
    --filename="${PWD}/templates/pubsub-agent-init-job-template.yml" \
    --local="true" \
    --param="NAMESPACE=${agent_namespace}" \
    --param="PUBSUB_HOST=maestro-pubsub.${namespace}" \
    --param="PUBSUB_PORT=8085" \
    --param="PUBSUB_PROJECT_ID=maestro-test" \
    --param="CONSUMER_NAME=${consumer_name}" \
  | kubectl apply -f -

  # Wait for initialization job to complete
  kubectl -n ${agent_namespace} wait --for=condition=complete --timeout=120s job/pubsub-agent-init-${consumer_name}

  # Check if job succeeded
  if ! kubectl -n ${agent_namespace} get job pubsub-agent-init-${consumer_name} -o jsonpath='{.status.succeeded}' | grep -q "1"; then
    echo "ERROR: Pub/Sub agent initialization job failed" >&2
    kubectl -n ${agent_namespace} logs job/pubsub-agent-init-${consumer_name}
    kubectl -n ${agent_namespace} delete job pubsub-agent-init-${consumer_name} --ignore-not-found
    exit 1
  fi

  # Clean up the initialization job
  kubectl -n ${agent_namespace} delete job pubsub-agent-init-${consumer_name} --ignore-not-found

  echo "Pub/Sub agent subscriptions initialized successfully"
fi

# Deploy maestro agent into maestro-agent namespace
make agent-tls-template
kubectl apply -n ${agent_namespace} --filename="templates/agent-tls-template.json" | egrep --color=auto 'configured|$$'

kubectl wait deploy/maestro-agent -n ${agent_namespace} --for condition=Available=True --timeout=200s
