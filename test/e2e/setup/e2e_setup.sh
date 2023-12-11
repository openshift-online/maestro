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

if ! command -v kind >/dev/null 2>&1; then 
    echo "This script will install kind (https://kind.sigs.k8s.io/) on your machine."
    curl -Lo ./kind-amd64 "https://kind.sigs.k8s.io/dl/v0.12.0/kind-$(uname)-amd64"
    chmod +x ./kind-amd64
    sudo mv ./kind-amd64 /usr/local/bin/kind
fi

# 1. create KinD cluster
kind create cluster --name maestro --kubeconfig ./test/e2e/.kubeconfig
export KUBECONFIG=${PWD}/test/e2e/.kubeconfig

# 2. build maestro image and load to KinD cluster
export namespace=maestro
export image_tag=latest
make image
kind load docker-image default-route-openshift-image-registry.apps-crc.testing/maestro/maestro:$image_tag --name maestro

# 3. deploy service-ca
kubectl label node maestro-control-plane node-role.kubernetes.io/master=
kubectl apply -f ./test/e2e/setup/service-ca-crds
kubectl $1 create ns openshift-config-managed
kubectl $1 apply -f ./test/e2e/setup/service-ca/

# 4. deploy maestro into maestro namespace
export ENABLE_JWT=false
export ENABLE_OCM_MOCK=true
kubectl create namespace $namespace || true
make template \
	deploy-secrets \
	deploy-db \
	deploy-mqtt \
	deploy-service

# expose the maestro server via nodeport
kubectl patch service maestro -n $namespace -p '{"spec":{"type":"NodePort", "ports":  [{"nodePort": 30080, "port": 8000, "targetPort": 8000}]}}' --type merge

# 5. create a consumer
export external_host_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' maestro-control-plane)
echo $external_host_ip > ./test/e2e/.external_host_ip
kubectl wait deployment maestro -n $namespace --for condition=Available=True --timeout=200s 
export consumer_id=$(curl -k -X POST -H "Content-Type: application/json" https://${external_host_ip}:30080/api/maestro/v1/consumers -d '{"name": "cluster1"}' | jq '.id')
echo $consumer_id > ./test/e2e/.consumer_id

# 6. deploy maestro agent into maestro-agent namespace
export agent_namespace=maestro-agent
kubectl create namespace $agent_namespace || true
make agent-template
kubectl apply -n ${agent_namespace} --filename="templates/agent-template.json" | egrep --color=auto 'configured|$$'

    
