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

# Deploy maestro agent into maestro-agent namespace
make agent-tls-template
kubectl apply -n ${agent_namespace} --filename="templates/agent-tls-template.json" | egrep --color=auto 'configured|$$'

# update the maestro-agent-mqtt secret
cat << EOF | kubectl -n ${agent_namespace} apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: maestro-agent-mqtt
stringData:
  config.yaml: |
    brokerHost: maestro-mqtt-agent.${namespace}:1883
    caFile: /secrets/mqtt-certs/ca.crt
    clientCertFile: /secrets/mqtt-certs/client.crt
    clientKeyFile: /secrets/mqtt-certs/client.key
    topics:
      sourceEvents: sources/maestro/consumers/${consumer_name}/sourceevents
      agentEvents: sources/maestro/consumers/${consumer_name}/agentevents
EOF

kubectl wait deploy/maestro-agent -n ${agent_namespace} --for condition=Available=True --timeout=200s
