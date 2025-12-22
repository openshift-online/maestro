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

mqtt_tls_enable=${ENABLE_MAESTRO_TLS:-"false"}
msg_broker=${MESSAGE_DRIVER_TYPE:-"mqtt"}

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

# Build Helm values for maestro-agent
values_file="${PWD}/test/_output/maestro-agent-values.yaml"

cat > "$values_file" <<EOF
environment: development

consumerName: ${consumer_name}

serviceAccount:
  name: maestro-agent-sa

image:
  registry: ${external_image_registry}
  repository: maestro/maestro
  tag: ${image_tag}
  pullPolicy: IfNotPresent

# Logging configuration
logging:
  klogV: "10"

# Message broker configuration
messageBroker:
  type: ${msg_broker}
EOF

# Configure MQTT settings
if [ "$msg_broker" = "mqtt" ]; then
  cat >> "$values_file" <<EOF
  mqtt:
    host: maestro-mqtt.${namespace}
    port: "1883"
    user: ""
    password: ""
EOF

  # Add TLS configuration if enabled
  if [ "$mqtt_tls_enable" = "true" ]; then
    cat >> "$values_file" <<EOF
    rootCert: /secrets/mqtt-certs/ca.crt
    clientCert: /secrets/mqtt-certs/client.crt
    clientKey: /secrets/mqtt-certs/client.key
EOF
  fi
fi

# Configure gRPC settings
if [ "$msg_broker" = "grpc" ]; then
  cat >> "$values_file" <<EOF
  grpc:
    url: maestro-grpc-broker.${namespace}:8091
EOF
fi

# Deploy using Helm
helm upgrade --install maestro-agent \
  ./charts/maestro-agent \
  --namespace "${agent_namespace}" \
  --values "$values_file" \
  --wait \
  --timeout 5m

kubectl wait deploy/maestro-agent -n ${agent_namespace} --for condition=Available=True --timeout=200s
