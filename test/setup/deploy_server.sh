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

tls_enable=${ENABLE_MAESTRO_TLS:-"false"}
msg_broker=${MESSAGE_DRIVER_TYPE:-"mqtt"}
server_replicas=${SERVER_REPLICAS:-"1"}
enable_broadcast=${ENABLE_BROADCAST_SUBSCRIPTION:-"false"}

export image_tag=${image_tag:-"latest"}
export external_image_registry=${external_image_registry:-"image-registry.testing"}
export internal_image_registry=${internal_image_registry:-"image-registry.testing"}

export namespace="maestro"
export KUBECONFIG=${PWD}/test/_output/.kubeconfig

# Build Helm values for maestro-server
values_file="${PWD}/test/_output/maestro-server-values.yaml"

cat > "$values_file" <<EOF
environment: development

serviceAccount:
  name: maestro

image:
  registry: ${external_image_registry}
  repository: maestro/maestro
  tag: ${image_tag}
  pullPolicy: IfNotPresent

replicas: ${server_replicas}

# Database configuration - use embedded PostgreSQL for testing
database:
  maxOpenConnections: 50
  sslMode: disable
  debug: true
  secretName: maestro-rds

# Message broker configuration
messageBroker:
  type: ${msg_broker}
  secretName: maestro-${msg_broker}

# Server configuration
server:
  https:
    enabled: ${tls_enable}
  hostname: ""
  http:
    bindPort: 8000
  grpc:
    enabled: true
    bindPort: 8090
  metrics:
    bindPort: 8080
    https:
      enabled: false
  healthCheck:
    bindPort: 8083
  httpReadTimeout: 5s
  httpWriteTimeout: 30s

# Service configuration - NodePort for e2e testing
service:
  api:
    type: NodePort
    port: 8000
    nodePort: 30080
  grpc:
    type: NodePort
    port: 8090
    nodePort: 30090
  metrics:
    port: 8080
  healthcheck:
    port: 8083

# Disable route for KinD testing
route:
  enabled: false

# Probes configuration - shorter delays for testing
livenessProbe:
  httpGet:
    path: /healthcheck
    port: 8083
    scheme: HTTP
  initialDelaySeconds: 1
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /healthcheck
    port: 8083
    scheme: HTTP
  initialDelaySeconds: 1
  periodSeconds: 5

# Use embedded PostgreSQL for testing
postgresql:
  enabled: true
  image: quay.io/maestro/postgres:17.2
  database:
    name: maestro
    user: maestro
    password: maestro
    host: maestro-db
  service:
    name: maestro-db
    port: 5432
  persistence:
    enabled: true
    size: 512Mi
  secretName: maestro-rds

# Use embedded MQTT broker for testing (if MQTT mode)
mqtt:
  enabled: $( [ "$msg_broker" = "mqtt" ] && echo "true" || echo "false" )
  image: quay.io/maestro/eclipse-mosquitto:2.0.18
  service:
    name: maestro-mqtt
    port: 1883
  host: maestro-mqtt
  user: ""
  password: ""
EOF

# Add broadcast subscription config if enabled
if [ "$enable_broadcast" = "true" ]; then
  cat >> "$values_file" <<EOF
  agentTopic: sources/maestro/consumers/+/agentevents
EOF
fi

# Create database secret (override embedded PostgreSQL secret)
kubectl delete secret maestro-rds -n "${namespace}" --ignore-not-found
kubectl create secret generic maestro-rds -n "${namespace}" \
  --from-literal=db.host=maestro-db \
  --from-literal=db.port=5432 \
  --from-literal=db.user=maestro \
  --from-literal=db.password=maestro \
  --from-literal=db.name=maestro

# Create MQTT secret if using MQTT broker
if [ "$msg_broker" = "mqtt" ]; then
  # MQTT config with or without TLS
  if [ "$tls_enable" = "true" ]; then
    mqtt_config=$(cat <<MQTT_EOF
brokerHost: maestro-mqtt:1883
caFile: /secrets/mqtt-certs/ca.crt
clientCertFile: /secrets/mqtt-certs/client.crt
clientKeyFile: /secrets/mqtt-certs/client.key
topics:
  sourceEvents: sources/maestro/consumers/+/sourceevents
  agentEvents: $( [ "$enable_broadcast" = "true" ] && echo "sources/maestro/consumers/+/agentevents" || echo "sources/maestro/consumers/+/agentevents" )
MQTT_EOF
)
  else
    mqtt_config=$(cat <<MQTT_EOF
brokerHost: maestro-mqtt:1883
topics:
  sourceEvents: sources/maestro/consumers/+/sourceevents
  agentEvents: sources/maestro/consumers/+/agentevents
MQTT_EOF
)
  fi

  kubectl delete secret maestro-mqtt -n "${namespace}" --ignore-not-found
  kubectl create secret generic maestro-mqtt -n "${namespace}" \
    --from-literal=config.yaml="$mqtt_config"
fi

# Create gRPC secret if using gRPC broker
if [ "$msg_broker" = "grpc" ]; then
  grpc_config="url: maestro-grpc-broker.maestro:8091"
  kubectl delete secret maestro-grpc -n "${namespace}" --ignore-not-found
  kubectl create secret generic maestro-grpc -n "${namespace}" \
    --from-literal=config.yaml="$grpc_config"
fi

# Deploy using Helm
helm upgrade --install maestro-server \
  ./charts/maestro-server \
  --namespace "${namespace}" \
  --values "$values_file" \
  --wait \
  --timeout 5m

kubectl wait deploy/maestro -n $namespace --for condition=Available=True --timeout=300s

# TODO use maestro service health check to ensure the service ready
sleep 30 # wait 30 seconds for the service ready

# Expose the RESTAPI and gRPC service hosts
rest_api_schema=$( [ "$tls_enable" = "true" ] && echo "https" || echo "http" )
echo "${rest_api_schema}://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
echo "127.0.0.1:30090" > ${PWD}/test/_output/.external_grpc_endpoint
