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

kind_version=0.12.0
step_version=0.26.2

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

# 1. create KinD cluster
cat << EOF | kind create cluster --name maestro --kubeconfig ./test/e2e/.kubeconfig --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
  - containerPort: 30090
    hostPort: 30090
EOF
export KUBECONFIG=${PWD}/test/e2e/.kubeconfig

# 2. build maestro image and load to KinD cluster
export namespace=maestro
export image_tag=latest
export external_image_registry=image-registry.testing
export internal_image_registry=image-registry.testing
make image
# related issue: https://github.com/kubernetes-sigs/kind/issues/2038
if command -v docker &> /dev/null; then
    kind load docker-image ${external_image_registry}/${namespace}/maestro:$image_tag --name maestro
elif command -v podman &> /dev/null; then
    podman save ${external_image_registry}/${namespace}/maestro:$image_tag -o /tmp/maestro.tar 
    kind load image-archive /tmp/maestro.tar --name maestro 
    rm /tmp/maestro.tar
else 
    echo "Neither Docker nor Podman is installed, exiting"
    exit 1
fi

# 3. deploy service-ca
kubectl label node maestro-control-plane node-role.kubernetes.io/master=
kubectl get pod -A
kubectl apply -f ./test/e2e/setup/service-ca-crds
kubectl $1 create ns openshift-config-managed
kubectl $1 apply -f ./test/e2e/setup/service-ca/
kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml

# 4. deploy maestro into maestro namespace
export ENABLE_JWT=false
export ENABLE_OCM_MOCK=true
export ENABLE_GRPC_SERVER=true
kubectl create namespace $namespace || true
make template \
	deploy-secrets \
	deploy-db \
	deploy-mqtt \
	deploy-service

cat << EOF | kubectl -n $namespace apply -f -
apiVersion: v1
kind: Service
metadata:
  name: maestro-mqtt-server
spec:
  ports:
  - name: mosquitto
    port: 1883
    protocol: TCP
    targetPort: 1883
  selector:
    name: maestro-mqtt
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: maestro-mqtt-agent
spec:
  ports:
  - name: mosquitto
    port: 1883
    protocol: TCP
    targetPort: 1883
  selector:
    name: maestro-mqtt
  type: ClusterIP
EOF

# expose the maestro server via nodeport
kubectl patch service maestro -n $namespace -p '{"spec":{"type":"NodePort", "ports":  [{"nodePort": 30080, "port": 8000, "targetPort": 8000}]}}' --type merge

# expose the maestro grpc server via nodeport
kubectl patch service maestro-grpc -n $namespace -p '{"spec":{"type":"NodePort", "ports":  [{"nodePort": 30090, "port": 8090, "targetPort": 8090}]}}' --type merge

# 5. create a self-signed certificate for mqtt
mqttCertDir=$(mktemp -d)
step certificate create "maestro-mqtt-ca" ${mqttCertDir}/ca.crt ${mqttCertDir}/ca.key --profile root-ca --no-password --insecure
step certificate create "maestro-mqtt-broker" ${mqttCertDir}/server.crt ${mqttCertDir}/server.key -san maestro-mqtt -san maestro-mqtt.maestro -san maestro-mqtt-server -san maestro-mqtt-server.maestro -san maestro-mqtt-agent -san maestro-mqtt-agent.maestro --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure
step certificate create "maestro-server-client" ${mqttCertDir}/server-client.crt ${mqttCertDir}/server-client.key --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure
step certificate create "maestro-agent-client" ${mqttCertDir}/agent-client.crt ${mqttCertDir}/agent-client.key --profile leaf --ca ${mqttCertDir}/ca.crt --ca-key ${mqttCertDir}/ca.key --no-password --insecure

# apply the mosquitto configmap
cat << EOF | kubectl -n $namespace apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: maestro-mqtt
data:
  mosquitto.conf: |
    listener 1883 0.0.0.0
    allow_anonymous false
    use_identity_as_username true
    cafile /mosquitto/certs/ca.crt
    keyfile /mosquitto/certs/server.key
    certfile /mosquitto/certs/server.crt
    tls_version tlsv1.2
    require_certificate true
EOF

# create secret containing the mqtt certs and patch the maestro-mqtt deployment
kubectl create secret generic maestro-mqtt-certs -n $namespace --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=server.crt=${mqttCertDir}/server.crt --from-file=server.key=${mqttCertDir}/server.key
kubectl patch deploy/maestro-mqtt -n $namespace --type='json' -p='[{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"mosquitto-certs","secret":{"secretName":"maestro-mqtt-certs"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"mosquitto-certs","mountPath":"/mosquitto/certs"}}]'
kubectl wait deploy/maestro-mqtt -n $namespace --for condition=Available=True --timeout=200s

# 6. create a self-signed certificate for maestro grpc
grpcCertDir=$(mktemp -d)
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
kubectl create secret generic maestro-grpc-cert -n $namespace --from-file=ca.crt=${grpcCertDir}/ca.crt --from-file=server.crt=${grpcCertDir}/server.crt --from-file=server.key=${grpcCertDir}/server.key --from-file=client.crt=${grpcCertDir}/client.crt --from-file=client.key=${grpcCertDir}/client.key

# create the grpc clusterrolebinding for publishing and subscribing
cat << EOF | kubectl -n $namespace apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: grpc-pub-sub
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: grpc-pub-sub
subjects:
- kind: User
  name: grpc-client
  apiGroup: rbac.authorization.k8s.io
- kind: ServiceAccount
  name: grpc-client
  namespace: $namespace
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grpc-client
---
apiVersion: v1
kind: Secret
metadata:
  name: grpc-client-token
  annotations:
    kubernetes.io/service-account.name: grpc-client
type: kubernetes.io/service-account-token
---
EOF

# patch the maestro deployment to mount the grpc certs and mqtt certs
maestroServerPatch='[{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"mqtt-certs","secret":{"secretName":"maestro-server-certs"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"mqtt-certs","mountPath":"/secrets/mqtt-certs"}},{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"maestro-grpc-cert","secret":{"secretName":"maestro-grpc-cert"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"maestro-grpc-cert","mountPath":"/secrets/maestro-grpc-cert"}},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-client-ca-file=/secrets/maestro-grpc-cert/ca.crt"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-authn-type=token"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-tls-cert-file=/secrets/maestro-grpc-cert/server.crt"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-tls-key-file=/secrets/maestro-grpc-cert/server.key"},{"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/initialDelaySeconds","value":1},{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/initialDelaySeconds","value":1}]'
if [ -n "${ENABLE_BROADCAST_SUBSCRIPTION}" ] && [ "${ENABLE_BROADCAST_SUBSCRIPTION}" = "true" ]; then
    maestroServerPatch='[{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--subscription-type=broadcast"},{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"mqtt-certs","secret":{"secretName":"maestro-server-certs"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"mqtt-certs","mountPath":"/secrets/mqtt-certs"}},{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"maestro-grpc-cert","secret":{"secretName":"maestro-grpc-cert"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"maestro-grpc-cert","mountPath":"/secrets/maestro-grpc-cert"}},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-client-ca-file=/secrets/maestro-grpc-cert/ca.crt"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-authn-type=token"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-tls-cert-file=/secrets/maestro-grpc-cert/server.crt"},{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--grpc-tls-key-file=/secrets/maestro-grpc-cert/server.key"},{"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/initialDelaySeconds","value":1},{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/initialDelaySeconds","value":1}]'
    cat << EOF | kubectl -n $namespace apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: maestro-mqtt
stringData:
  config.yaml: |
    brokerHost: maestro-mqtt-server.maestro:1883
    caFile: /secrets/mqtt-certs/ca.crt
    clientCertFile: /secrets/mqtt-certs/client.crt
    clientKeyFile: /secrets/mqtt-certs/client.key
    topics:
      sourceEvents: sources/maestro/consumers/+/sourceevents
      agentEvents: sources/maestro/consumers/+/agentevents
EOF
else
    cat << EOF | kubectl -n $namespace apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: maestro-mqtt
stringData:
  config.yaml: |
    brokerHost: maestro-mqtt-server.maestro:1883
    caFile: /secrets/mqtt-certs/ca.crt
    clientCertFile: /secrets/mqtt-certs/client.crt
    clientKeyFile: /secrets/mqtt-certs/client.key
    topics:
      sourceEvents: sources/maestro/consumers/+/sourceevents
      agentEvents: \$share/statussubscribers/sources/maestro/consumers/+/agentevents
EOF
fi

# create secret containing the client certs to mqtt broker and patch the maestro deployment
kubectl create secret generic maestro-server-certs -n $namespace --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=client.crt=${mqttCertDir}/server-client.crt --from-file=client.key=${mqttCertDir}/server-client.key
kubectl patch deploy/maestro -n $namespace --type='json' -p=${maestroServerPatch}
kubectl wait deploy/maestro -n $namespace --for condition=Available=True --timeout=200s

# 6. create a consumer
export external_host_ip="127.0.0.1"
echo $external_host_ip > ./test/e2e/.external_host_ip

sleep 5 # wait 5 seconds for the service ready

# the consumer name is not specified, the consumer id will be used as the consumer name
export consumer_name=$(curl -k -X POST -H "Content-Type: application/json" https://${external_host_ip}:30080/api/maestro/v1/consumers -d '{}' | jq '.id')
consumer_name=$(echo "$consumer_name" | sed 's/"//g')
echo $consumer_name > ./test/e2e/.consumer_name

# 7. deploy maestro agent into maestro-agent namespace
export agent_namespace=maestro-agent
kubectl create namespace ${agent_namespace} || true
make agent-template
kubectl apply -n ${agent_namespace} --filename="templates/agent-template.json" | egrep --color=auto 'configured|$$'

# apply the maestro-mqtt secret
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

# create secret containing the client certs to mqtt broker and patch the maestro-agent deployment
kubectl create secret generic maestro-agent-certs -n ${agent_namespace} --from-file=ca.crt=${mqttCertDir}/ca.crt --from-file=client.crt=${mqttCertDir}/agent-client.crt --from-file=client.key=${mqttCertDir}/agent-client.key
kubectl patch deploy/maestro-agent -n ${agent_namespace} --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/command/-","value":"--appliedmanifestwork-eviction-grace-period=30s"},{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"mqtt-certs","secret":{"secretName":"maestro-agent-certs"}}},{"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/-","value":{"name":"mqtt-certs","mountPath":"/secrets/mqtt-certs"}}]'
kubectl wait deploy/maestro-agent -n ${agent_namespace} --for condition=Available=True --timeout=200s

# remove the certs
rm -rf ${mqttCertDir} ${grpcCertDir}
