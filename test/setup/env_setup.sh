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

# TODO use this as a unified test env setup script instead of test/e2e/setup

kind_version=0.12.0
step_version=0.26.2
istio_version=1.25.5

enable_istio=${ENABLE_ISTIO:-"false"}
msg_broker=${MESSAGE_DRIVER_TYPE:-"mqtt"}

export image_tag="latest"
export external_image_registry="image-registry.testing"
export internal_image_registry="image-registry.testing"

export namespace="maestro"
export agent_namespace="maestro-agent"

export KUBECONFIG=${PWD}/test/_output/.kubeconfig

mkdir -p ${PWD}/test/_output

# Check the dependent tools
if ! command -v kind >/dev/null 2>&1; then
    echo "This script will install kind (https://kind.sigs.k8s.io/) on your machine."
    curl -Lo ./kind-amd64 "https://kind.sigs.k8s.io/dl/v${kind_version}/kind-$(uname)-amd64"
    chmod +x ./kind-amd64
    sudo mv ./kind-amd64 /usr/local/bin/kind
fi

if ! command -v step >/dev/null 2>&1; then
    echo "This script will install step (https://smallstep.com/docs/step-cli/) on your machine."
    curl -Lo ./step_${step_version}_amd64.tar.gz "https://dl.smallstep.com/gh-release/cli/gh-release-header/v${step_version}/step_$(uname | tr '[:upper:]' '[:lower:]')_${step_version}_amd64.tar.gz"
    tar -xzvf step_${step_version}_amd64.tar.gz
    chmod +x ./step_${step_version}/bin/step
    sudo mv ./step_${step_version}/bin/step /usr/local/bin/step
    rm -rf ./step_${step_version}_amd64.tar.gz ./step_${step_version}
fi

if [ "$enable_istio" = "true" ] && ! command -v istioctl >/dev/null 2>&1; then
    echo "This script will install istioctl (https://istio.io/latest/docs/ops/diagnostic-tools/istioctl/) on your machine."
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${istio_version} sh -
    chmod +x ./istio-${istio_version}/bin/istioctl
    sudo mv ./istio-${istio_version}/bin/istioctl /usr/local/bin/istioctl
    rm -rf ./istio-${istio_version}
fi

if command -v docker &> /dev/null; then
    container_tool="docker"
elif command -v podman &> /dev/null; then
    container_tool="podman"
else
    echo "Neither Docker nor Podman is installed, exiting"
    exit 1
fi

# Build images with current code
make image e2e-image

# Create a KinD cluster
if [ ! -f "$KUBECONFIG" ]; then
  cat << EOF | kind create cluster --name maestro --kubeconfig ${KUBECONFIG} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
  - containerPort: 30090
    hostPort: 30090
  - containerPort: 30100
    hostPort: 30100
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    apiVersion: kubelet.config.k8s.io/v1beta1
    syncFrequency: "1s"
    configMapAndSecretChangeDetectionStrategy: "Watch"
EOF
fi

# Load the images based on container tool
if [ "$container_tool" = "docker" ]; then
    kind load docker-image ${external_image_registry}/maestro/maestro:$image_tag --name maestro
    kind load docker-image ${external_image_registry}/maestro/maestro-e2e:$image_tag --name maestro
else
    # related issue: https://github.com/kubernetes-sigs/kind/issues/2038
    podman save ${external_image_registry}/maestro/maestro:$image_tag -o /tmp/maestro.tar
    kind load image-archive /tmp/maestro.tar --name maestro
    rm /tmp/maestro.tar
    podman save ${external_image_registry}/maestro/maestro-e2e:$image_tag -o /tmp/maestro-e2e.tar
    kind load image-archive /tmp/maestro-e2e.tar --name maestro
    rm /tmp/maestro-e2e.tar
fi

# Prepare a in-cluster kubeconfig
in_cluster_kubeconfig=${PWD}/test/_output/.in-cluster.kubeconfig
kubectl --kubeconfig ${KUBECONFIG} config view --minify --flatten > $in_cluster_kubeconfig
context=$(kubectl --kubeconfig ${in_cluster_kubeconfig} config current-context)
cluster_name=$(kubectl --kubeconfig ${in_cluster_kubeconfig} config view -o jsonpath="{.contexts[?(@.name==\"${context}\")].context.cluster}")
cluster_ip=$(kubectl --kubeconfig ${in_cluster_kubeconfig} get svc kubernetes -n default -o jsonpath="{.spec.clusterIP}")
kubectl --kubeconfig ${in_cluster_kubeconfig} config set-cluster "${cluster_name}" --server="https://${cluster_ip}"

# Create namespaces
kubectl create namespace openshift-config-managed || true
kubectl create namespace ${namespace} || true
kubectl create namespace ${agent_namespace} || true
kubectl create namespace clusters-service || true

# Create HTTPS certificates for maestro server
https_cert_dir="${PWD}/test/_output/certs/https"
if [ ! -d "$https_cert_dir" ]; then
  mkdir -p "$https_cert_dir"
  step certificate create "maestro-https-ca" ${https_cert_dir}/ca.crt ${https_cert_dir}/ca.key --kty RSA --profile root-ca --no-password --insecure
  step certificate create "maestro-server" ${https_cert_dir}/tls.crt ${https_cert_dir}/tls.key --kty RSA -san maestro -san maestro.${namespace} -san maestro.${namespace}.svc -san localhost -san 127.0.0.1 --profile leaf --ca ${https_cert_dir}/ca.crt --ca-key ${https_cert_dir}/ca.key --no-password --insecure
  kubectl delete secret maestro-https-certs -n "${namespace}" --ignore-not-found
  kubectl create secret tls maestro-https-certs -n "${namespace}" --cert=${https_cert_dir}/tls.crt --key=${https_cert_dir}/tls.key
fi

# Apply ManifestWork CRD
kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml

# Install istio if enabled
if [ "$enable_istio" = "true" ]; then
  istioctl install --set profile=minimal -y
  kubectl label namespace ${namespace} istio-injection=enabled --overwrite
  kubectl label namespace clusters-service istio-injection=enabled --overwrite
fi

if [ "$msg_broker" = "mqtt" ]; then
  # Deploy the message broker required by the Maestro server
  mqtt_cert_dir="${PWD}/test/_output/certs/mqtt"
  if [ ! -d "$mqtt_cert_dir" ]; then
    # create certs
    mkdir -p "$mqtt_cert_dir"
    step certificate create "maestro-mqtt-ca" ${mqtt_cert_dir}/ca.crt ${mqtt_cert_dir}/ca.key --kty RSA --profile root-ca --no-password --insecure
    step certificate create "maestro-mqtt-broker" ${mqtt_cert_dir}/server.crt ${mqtt_cert_dir}/server.key --kty RSA -san maestro-mqtt -san maestro-mqtt.maestro --profile leaf --ca ${mqtt_cert_dir}/ca.crt --ca-key ${mqtt_cert_dir}/ca.key --no-password --insecure
    step certificate create "maestro-server-client" ${mqtt_cert_dir}/server-client.crt ${mqtt_cert_dir}/server-client.key --kty RSA --profile leaf --ca ${mqtt_cert_dir}/ca.crt --ca-key ${mqtt_cert_dir}/ca.key --no-password --insecure
    step certificate create "maestro-agent-client" ${mqtt_cert_dir}/agent-client.crt ${mqtt_cert_dir}/agent-client.key --kty RSA --profile leaf --ca ${mqtt_cert_dir}/ca.crt --ca-key ${mqtt_cert_dir}/ca.key --no-password --insecure
    # create secrets
    kubectl delete secret maestro-mqtt-certs -n "${namespace}" --ignore-not-found
    kubectl delete secret maestro-server-certs -n "${namespace}" --ignore-not-found
    kubectl delete secret maestro-agent-certs -n "${agent_namespace}" --ignore-not-found
    kubectl create secret generic maestro-mqtt-certs -n "${namespace}" --from-file=ca.crt=${mqtt_cert_dir}/ca.crt --from-file=server.crt=${mqtt_cert_dir}/server.crt --from-file=server.key=${mqtt_cert_dir}/server.key
    kubectl create secret generic maestro-server-certs -n "${namespace}" --from-file=ca.crt=${mqtt_cert_dir}/ca.crt --from-file=client.crt=${mqtt_cert_dir}/server-client.crt --from-file=client.key=${mqtt_cert_dir}/server-client.key
    kubectl create secret generic maestro-agent-certs -n "${agent_namespace}" --from-file=ca.crt=${mqtt_cert_dir}/ca.crt --from-file=client.crt=${mqtt_cert_dir}/agent-client.crt --from-file=client.key=${mqtt_cert_dir}/agent-client.key
    # create a separate secret for MQTT CA keys used in certificate rotation tests
    kubectl create secret generic maestro-mqtt-ca -n "${agent_namespace}" --from-file=ca.crt=${mqtt_cert_dir}/ca.crt --from-file=ca.key=${mqtt_cert_dir}/ca.key
  fi

  export mqtt_user=""
  export mqtt_password_file="/dev/null"
  export mqtt_root_cert="/secrets/mqtt-certs/ca.crt"
  export mqtt_client_cert="/secrets/mqtt-certs/client.crt"
  export mqtt_client_key="/secrets/mqtt-certs/client.key"
  # MQTT broker will be deployed by Helm chart in deploy_server.sh
fi

if [ "$msg_broker" = "grpc" ]; then
  # Prepare certs for gRPC broker
  grpc_broker_cert_dir="${PWD}/test/_output/certs/grpc-broker"
  if [ ! -d "$grpc_broker_cert_dir" ]; then
    # create certs
    mkdir -p "$grpc_broker_cert_dir"
    step certificate create "maestro-grpc-broker-ca" ${grpc_broker_cert_dir}/ca.crt ${grpc_broker_cert_dir}/ca.key --kty RSA --profile root-ca --no-password --insecure
    step certificate create "maestro-grpc-broker-server" ${grpc_broker_cert_dir}/server.crt ${grpc_broker_cert_dir}/server.key --kty RSA -san maestro-grpc-broker -san maestro-grpc-broker.maestro -san maestro-grpc-broker.maestro.svc -san localhost -san 127.0.0.1 --profile leaf --ca ${grpc_broker_cert_dir}/ca.crt --ca-key ${grpc_broker_cert_dir}/ca.key --no-password --insecure
    cat << EOF > ${grpc_broker_cert_dir}/cert.tpl
{
    "subject":{"organization":"open-cluster-management","commonName":"grpc-client"},
    "keyUsage":["digitalSignature"],
    "extKeyUsage": ["serverAuth","clientAuth"]
}
EOF
    step certificate create "maestro-grpc-broker-client" ${grpc_broker_cert_dir}/client.crt ${grpc_broker_cert_dir}/client.key --kty RSA --template ${grpc_broker_cert_dir}/cert.tpl --ca ${grpc_broker_cert_dir}/ca.crt --ca-key ${grpc_broker_cert_dir}/ca.key --no-password --insecure
    # create secrets
    kubectl delete secret maestro-grpc-broker-cert -n "$namespace" --ignore-not-found
    kubectl delete secret maestro-grpc-broker-cert -n "$agent_namespace" --ignore-not-found
    kubectl create secret generic maestro-grpc-broker-cert -n "$namespace" --from-file=ca.crt=${grpc_broker_cert_dir}/ca.crt --from-file=server.crt=${grpc_broker_cert_dir}/server.crt --from-file=server.key=${grpc_broker_cert_dir}/server.key
    kubectl create secret generic maestro-grpc-broker-cert -n "$agent_namespace" --from-file=ca.crt=${grpc_broker_cert_dir}/ca.crt --from-file=client.crt=${grpc_broker_cert_dir}/client.crt --from-file=client.key=${grpc_broker_cert_dir}/client.key
    kubectl create secret generic maestro-grpc-broker-ca -n "$agent_namespace" --from-file=ca.crt=${grpc_broker_cert_dir}/ca.crt --from-file=ca.key=${grpc_broker_cert_dir}/ca.key
  fi
fi

# Database (PostgreSQL) will be deployed by Helm chart in deploy_server.sh
