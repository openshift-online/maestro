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

# enable istio on the ENABLE_ISTIO env
enable_istio=${ENABLE_ISTIO:-"false"}

kind_version=0.12.0
step_version=0.26.2
istio_version=1.25.5

export namespace="maestro"
export agent_namespace="maestro-agent"
export image_tag=${image_tag:-"latest"}
export external_image_registry=${external_image_registry:-"image-registry.testing"}
export internal_image_registry=${internal_image_registry:-"image-registry.testing"}

export KUBECONFIG=${PWD}/test/e2e/.kubeconfig

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

# 1. create KinD cluster
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
EOF
fi

# 2. build maestro image and load to KinD cluster
if [ $external_image_registry == "image-registry.testing" ]; then
  make image csclient-image
  # related issue: https://github.com/kubernetes-sigs/kind/issues/2038
  if command -v docker &> /dev/null; then
      kind load docker-image ${external_image_registry}/${namespace}/maestro:$image_tag --name maestro
      kind load docker-image ${external_image_registry}/${namespace}/maestro-csclient:$image_tag --name maestro
  elif command -v podman &> /dev/null; then
      podman save ${external_image_registry}/${namespace}/maestro:$image_tag -o /tmp/maestro.tar 
      kind load image-archive /tmp/maestro.tar --name maestro 
      rm /tmp/maestro.tar
      podman save ${external_image_registry}/${namespace}/maestro-csclient:$image_tag -o /tmp/maestro-csclient.tar
      kind load image-archive /tmp/maestro-csclient.tar --name maestro
      rm /tmp/maestro-csclient.tar
  else 
      echo "Neither Docker nor Podman is installed, exiting"
      exit 1
  fi
fi

export csclient_image=${external_image_registry}/${namespace}/maestro-csclient:$image_tag

# 3. deploy service-ca
kubectl label node maestro-control-plane node-role.kubernetes.io/master= --overwrite
kubectl get pod -A
kubectl apply -f ./test/e2e/setup/service-ca-crds
kubectl create ns openshift-config-managed || true
kubectl apply -f ./test/e2e/setup/service-ca/
kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml

# install istio if enabled
if [ "$enable_istio" = "true" ]; then
  istioctl install --set profile=minimal -y
fi

# 4. create maestro namespace
kubectl create namespace $namespace || true
kubectl label namespace $namespace istio-injection=enabled --overwrite
# create maestro-agent namespace
kubectl create namespace ${agent_namespace} || true
# create csclient namespace
kubectl create namespace csclient || true
kubectl label namespace csclient istio-injection=enabled --overwrite

# 5. create a self-signed certificate for mqtt
mqttCertDir="./test/e2e/certs/mqtt"
if [ ! -d "$mqttCertDir" ]; then
  mkdir -p $mqttCertDir
  step certificate create "maestro-mqtt-ca" ${mqttCertDir}/ca.crt ${mqttCertDir}/ca.key --profile root-ca --no-password --insecure
  step certificate create "maestro-mqtt-broker" ${mqttCertDir}/server.crt ${mqttCertDir}/server.key -san maestro-mqtt -san maestro-mqtt.maestro -san maestro-mqtt-server -san maestro-mqtt-server.maestro -san maestro-mqtt-agent -san maestro-mqtt-agent.maestro --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure
  step certificate create "maestro-server-client" ${mqttCertDir}/server-client.crt ${mqttCertDir}/server-client.key --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure
  step certificate create "maestro-agent-client" ${mqttCertDir}/agent-client.crt ${mqttCertDir}/agent-client.key --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure
  kubectl create secret generic maestro-mqtt-certs -n $namespace --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=server.crt=${mqttCertDir}/server.crt --from-file=server.key=${mqttCertDir}/server.key
  kubectl create secret generic maestro-server-certs -n $namespace --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=client.crt=${mqttCertDir}/server-client.crt --from-file=client.key=${mqttCertDir}/server-client.key
  kubectl create secret generic maestro-agent-certs -n ${agent_namespace} --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=client.crt=${mqttCertDir}/agent-client.crt --from-file=client.key=${mqttCertDir}/agent-client.key
fi

# 6. create a self-signed certificate for maestro grpc
grpcCertDir="./test/e2e/certs/grpc"
if [ ! -d "$grpcCertDir" ]; then
  mkdir -p $grpcCertDir
  step certificate create "maestro-grpc-ca" ${grpcCertDir}/ca.crt ${grpcCertDir}/ca.key --profile root-ca --no-password --insecure
  step certificate create "maestro-grpc-server" ${grpcCertDir}/server.crt ${grpcCertDir}/server.key -san maestro-grpc -san maestro-grpc.maestro -san localhost -san 127.0.0.1 --profile leaf --ca ${grpcCertDir}/ca.crt --ca-key ${grpcCertDir}/ca.key --no-password --insecure
  cat << EOF > ${grpcCertDir}/cert.tpl
{
    "subject":{"organization":"open-cluster-management","commonName":"grpc-client"},
    "keyUsage":["digitalSignature"],
    "extKeyUsage": ["serverAuth","clientAuth"]
}
EOF
  step certificate create "maestro-grpc-client" ${grpcCertDir}/client.crt ${grpcCertDir}/client.key --template ${grpcCertDir}/cert.tpl --ca ${grpcCertDir}/ca.crt --ca-key ${grpcCertDir}/ca.key --no-password --insecure
fi

kubectl create secret generic maestro-grpc-cert -n $namespace --from-file=ca.crt=${grpcCertDir}/ca.crt --from-file=server.crt=${grpcCertDir}/server.crt --from-file=server.key=${grpcCertDir}/server.key --from-file=client.crt=${grpcCertDir}/client.crt --from-file=client.key=${grpcCertDir}/client.key || true
kubectl create secret generic maestro-grpc-cert -n csclient --from-file=ca.crt=${grpcCertDir}/ca.crt --from-file=client.crt=${grpcCertDir}/client.crt --from-file=client.key=${grpcCertDir}/client.key || true

grpcBrokerCertDir="./test/e2e/certs/grpc-broker"
if [ ! -d "$grpcBrokerCertDir" ]; then
  mkdir -p $grpcBrokerCertDir
  step certificate create "maestro-grpc-broker-ca" ${grpcBrokerCertDir}/ca.crt ${grpcBrokerCertDir}/ca.key --profile root-ca --no-password --insecure
  step certificate create "maestro-grpc-broker-server" ${grpcBrokerCertDir}/server.crt ${grpcBrokerCertDir}/server.key -san maestro-grpc-broker -san maestro-grpc-broker.maestro -san maestro-grpc-broker.maestro.svc -san localhost -san 127.0.0.1 --profile leaf --ca ${grpcBrokerCertDir}/ca.crt --ca-key ${grpcBrokerCertDir}/ca.key --no-password --insecure
  cat << EOF > ${grpcBrokerCertDir}/cert.tpl
{
    "subject":{"organization":"open-cluster-management","commonName":"grpc-client"},
    "keyUsage":["digitalSignature"],
    "extKeyUsage": ["serverAuth","clientAuth"]
}
EOF
  step certificate create "maestro-grpc-broker-client" ${grpcBrokerCertDir}/client.crt ${grpcBrokerCertDir}/client.key --template ${grpcBrokerCertDir}/cert.tpl --ca ${grpcBrokerCertDir}/ca.crt --ca-key ${grpcBrokerCertDir}/ca.key --no-password --insecure
  kubectl create secret generic maestro-grpc-broker-cert -n $namespace --from-file=ca.crt=${grpcBrokerCertDir}/ca.crt --from-file=server.crt=${grpcBrokerCertDir}/server.crt --from-file=server.key=${grpcBrokerCertDir}/server.key --from-file=client.crt=${grpcBrokerCertDir}/client.crt --from-file=client.key=${grpcBrokerCertDir}/client.key
  kubectl create secret generic maestro-grpc-broker-cert -n $agent_namespace --from-file=ca.crt=${grpcBrokerCertDir}/ca.crt --from-file=server.crt=${grpcBrokerCertDir}/server.crt --from-file=server.key=${grpcBrokerCertDir}/server.key --from-file=client.crt=${grpcBrokerCertDir}/client.crt --from-file=client.key=${grpcBrokerCertDir}/client.key
fi

# 7. deploy maestro into maestro namespace
export ENABLE_JWT=false
export ENABLE_OCM_MOCK=true
export maestro_svc_type="NodePort"
export maestro_svc_node_port=30080
export grpc_svc_type="NodePort"
export grpc_svc_node_port=30090
export liveness_probe_init_delay_seconds=1
export readiness_probe_init_delay_seconds=1
export mqtt_user=""
export mqtt_password_file="/dev/null"
export mqtt_root_cert="/secrets/mqtt-certs/ca.crt"
export mqtt_client_cert="/secrets/mqtt-certs/client.crt"
export mqtt_client_key="/secrets/mqtt-certs/client.key"
if [ -n "${ENABLE_BROADCAST_SUBSCRIPTION}" ] && [ "${ENABLE_BROADCAST_SUBSCRIPTION}" = "true" ]; then
  export subscription_type="broadcast"
  export agent_topic="sources/maestro/consumers/+/agentevents"
fi

make deploy-secrets \
	deploy-db \
	deploy-mqtt-tls \
	deploy-service-tls

# disable grpc tls when istio is enabled because istio provides mutual tls
if [ "$enable_istio" = "true" ]; then
  kubectl get deploy/maestro -n $namespace -o json | jq '.spec.template.spec.containers[0].command |= map(select(startswith("--grpc-tls-") | not))' | kubectl apply -f -
fi
kubectl wait deploy/maestro-mqtt  --for condition=Available=True --timeout=200s
kubectl wait deploy/maestro -n $namespace --for condition=Available=True --timeout=200s

sleep 30 # wait 30 seconds for the service ready

# 8. create a consumer
export external_host_ip="127.0.0.1"
echo $external_host_ip > ./test/e2e/.external_host_ip

# the consumer name is not specified, the consumer id will be used as the consumer name
if [ ! -f "./test/e2e/.consumer_name" ]; then
  consumer_name=$(curl -s -k -X POST -H "Content-Type: application/json" https://${external_host_ip}:30080/api/maestro/v1/consumers -d '{}' | jq '.id')
  consumer_name=$(echo "$consumer_name" | sed 's/"//g')
  echo $consumer_name > ./test/e2e/.consumer_name
fi
export consumer_name=$(cat ./test/e2e/.consumer_name)

# 9. deploy maestro agent into maestro-agent namespace
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

# 10. deploy a client in csclient namespace
envsubst < ./examples/csclient/deploy.yaml | kubectl apply -f -
kubectl patch svc/csclient -n csclient -p '{"spec":{"type":"NodePort","ports":[{"port":80,"targetPort":8080,"nodePort":30100}]}}'
kubectl wait deploy/csclient -n csclient --for condition=Available=True --timeout=200s

# disable grpc tls when istio is enabled because istio provides mutual tls
if [ "$enable_istio" = "true" ]; then
  kubectl get deploy/csclient -n csclient -o json | jq '.spec.template.spec.containers[0].command |= map(select(startswith("--grpc-client-") | not))' | kubectl apply -f -
fi
