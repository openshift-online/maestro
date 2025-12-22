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
enable_istio=${ENABLE_ISTIO:-"false"}

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
    enabled: $( [ "$enable_istio" = "true" ] && echo "false" || echo "true" )
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
  tls:
    enabled: ${tls_enable}
    caFile: /secrets/mqtt-certs/ca.crt
    clientCertFile: /secrets/mqtt-certs/client.crt
    clientKeyFile: /secrets/mqtt-certs/client.key
EOF

# Add broadcast subscription config if enabled
if [ "$enable_broadcast" = "true" ]; then
  cat >> "$values_file" <<EOF
  agentTopic: sources/maestro/consumers/+/agentevents
EOF
fi

# Configure gRPC broker if using gRPC
if [ "$msg_broker" = "grpc" ]; then
  cat >> "$values_file" <<EOF

# gRPC broker configuration
grpc:
  enabled: true
  url: maestro-grpc-broker.${namespace}:8091
  tls:
    enabled: ${tls_enable}
    certFile: /secrets/grpc-broker-cert/server.crt
    keyFile: /secrets/grpc-broker-cert/server.key
    clientCAFile: /secrets/grpc-broker-cert/ca.crt
EOF
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
# HTTPS is enabled unless Istio is enabled (Istio handles mTLS)
if [ "$enable_istio" = "true" ]; then
  echo "http://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
else
  echo "https://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
fi
echo "127.0.0.1:30090" > ${PWD}/test/_output/.external_grpc_endpoint
