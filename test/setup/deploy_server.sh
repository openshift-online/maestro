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

if [ "$tls_enable" = "true" ]; then
  # deploy openshift service-ca to generate certs for internal services (metrics/health)
  kubectl label node maestro-control-plane node-role.kubernetes.io/master= --overwrite
  kubectl apply -f "${PWD}/test/setup/service-ca-crds"
  # kubectl create namespace openshift-config-managed || true
  kubectl apply -f "${PWD}/test/setup/service-ca"
  sleep 10 # wait for openshift service-ca-operator is created
  kubectl wait deploy/service-ca-operator -n openshift-service-ca-operator --for condition=Available=True --timeout=300s
  sleep 10 # wait for openshift service-ca is created
  kubectl wait deploy/service-ca -n openshift-service-ca --for condition=Available=True --timeout=300s
  # prepare gRPC service certs if they are not found
  grpc_cert_dir="${PWD}/test/_output/certs/grpc"
  if [ ! -d "$grpc_cert_dir" ]; then
    # create certs
    mkdir -p "$grpc_cert_dir"
    step certificate create "maestro-grpc-ca" ${grpc_cert_dir}/ca.crt ${grpc_cert_dir}/ca.key --kty RSA --profile root-ca --no-password --insecure
    step certificate create "maestro-grpc-server" ${grpc_cert_dir}/server.crt ${grpc_cert_dir}/server.key --kty RSA -san maestro-grpc -san maestro-grpc.maestro -san localhost -san 127.0.0.1 --profile leaf --ca ${grpc_cert_dir}/ca.crt --ca-key ${grpc_cert_dir}/ca.key --no-password --insecure
    cat << EOF > ${grpc_cert_dir}/cert.tpl
{
    "subject":{"organization":"open-cluster-management","commonName":"grpc-client"},
    "keyUsage":["digitalSignature"],
    "extKeyUsage": ["serverAuth","clientAuth"]
}
EOF
    step certificate create "maestro-grpc-client" ${grpc_cert_dir}/client.crt ${grpc_cert_dir}/client.key --kty RSA --template ${grpc_cert_dir}/cert.tpl --ca ${grpc_cert_dir}/ca.crt --ca-key ${grpc_cert_dir}/ca.key --no-password --insecure
    # create secrets
    kubectl delete secret maestro-grpc-cert -n "$namespace" --ignore-not-found
    kubectl create secret generic maestro-grpc-cert -n "$namespace" --from-file=ca.crt=${grpc_cert_dir}/ca.crt --from-file=server.crt=${grpc_cert_dir}/server.crt --from-file=server.key=${grpc_cert_dir}/server.key --from-file=client.crt=${grpc_cert_dir}/client.crt --from-file=client.key=${grpc_cert_dir}/client.key
  fi
fi

# Build Helm values for maestro-server
values_file="${PWD}/test/_output/maestro-server-values.yaml"

# Set Istio annotations if enabled
if [ "$enable_istio" = "true" ]; then
  istio_annotations='  annotations:
    proxy.istio.io/config: |
      {
        "holdApplicationUntilProxyStarts": true
      }'
else
  istio_annotations=''
fi

cat > "$values_file" <<EOF
environment: development

serviceAccount:
  name: maestro

# Logging configuration
logging:
  klogV: "10"

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
  mqtt:
    port: 1883
    host: maestro-mqtt
    tls:
      enabled: ${tls_enable}
      caFile: /secrets/mqtt-certs/ca.crt
      clientCertFile: /secrets/mqtt-certs/client.crt
      clientKeyFile: /secrets/mqtt-certs/client.key
  grpc:
    url: maestro-grpc-broker.${namespace}:8091
    tls:
      enabled: ${tls_enable}
      certFile: /secrets/grpc-broker-cert/server.crt
      keyFile: /secrets/grpc-broker-cert/server.key
      clientCAFile: /secrets/grpc-broker-cert/ca.crt
  pubsub:
    projectID: maestro-test
    endpoint: maestro-pubsub:8085
    disableTLS: true
    topics:
      sourceEvents: projects/maestro-test/topics/sourceevents
      sourceBroadcast: projects/maestro-test/topics/sourcebroadcast
    subscriptions:
      agentEvents: projects/maestro-test/subscriptions/agentevents-maestro
      agentBroadcast: projects/maestro-test/subscriptions/agentbroadcast-maestro

# Server configuration
server:
${istio_annotations}
  https:
    enabled: $( [ "$enable_istio" = "true" ] && echo "false" || echo "true" )
  hostname: ""
  http:
    bindPort: 8000
  grpc:
    bindPort: 8090
    tls:
      enabled: ${tls_enable}
      certFile: /secrets/maestro-grpc-cert/server.crt
      keyFile: /secrets/maestro-grpc-cert/server.key
      clientCAFile: /secrets/maestro-grpc-cert/ca.crt
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
mosquitto:
  enabled: $( [ "$msg_broker" = "mqtt" ] && echo "true" || echo "false" )
  image: quay.io/maestro/eclipse-mosquitto:2.0.18
  service:
    name: maestro-mqtt
    port: 1883
  tls:
    enabled: ${tls_enable}
    caFile: /secrets/mqtt-certs/ca.crt
    clientCertFile: /secrets/mqtt-certs/client.crt
    clientKeyFile: /secrets/mqtt-certs/client.key

# Use embedded Pub/Sub emulator for testing (if Pub/Sub mode)
pubsubEmulator:
  enabled: $( [ "$msg_broker" = "pubsub" ] && echo "true" || echo "false" )
  image: gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators
  projectID: maestro-test
  service:
    name: maestro-pubsub
    port: 8085
EOF

# Deploy using Helm
helm upgrade --install maestro-server \
  ./charts/maestro-server \
  --namespace "${namespace}" \
  --values "$values_file" \
  --wait \
  --timeout 5m

kubectl rollout restart deploy/maestro -n ${namespace}
kubectl rollout status deploy/maestro --timeout=300s -n ${namespace}

# TODO use maestro service health check to ensure the service ready
sleep 30 # wait 30 seconds for the service ready

if [ "$tls_enable" = "true" ]; then
  # deploy grpc-client-token for testing
  kubectl apply -f "${PWD}/test/setup/grpc-client" -n ${namespace}
fi

# Expose the RESTAPI and gRPC service hosts
# HTTPS is enabled unless Istio is enabled (Istio handles mTLS)
if [ "$enable_istio" = "true" ]; then
  echo "http://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
else
  echo "https://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
fi
echo "127.0.0.1:30090" > ${PWD}/test/_output/.external_grpc_endpoint
