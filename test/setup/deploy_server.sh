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

export image_tag=${image_tag:-"latest"}
export external_image_registry=${external_image_registry:-"image-registry.testing"}
export internal_image_registry=${internal_image_registry:-"image-registry.testing"}

export namespace="maestro"
export agent_namespace="maestro-agent"

export KUBECONFIG=${PWD}/test/_output/.kubeconfig

# Deploy maestro into maestro namespace
export ENABLE_JWT=false
export ENABLE_OCM_MOCK=true
export maestro_svc_type="NodePort"
export maestro_svc_node_port=30080
export grpc_svc_type="NodePort"
export grpc_svc_node_port=30090
export liveness_probe_init_delay_seconds=1
export readiness_probe_init_delay_seconds=1
if [ -n "${ENABLE_BROADCAST_SUBSCRIPTION}" ] && [ "${ENABLE_BROADCAST_SUBSCRIPTION}" = "true" ]; then
  export subscription_type="broadcast"
  export agent_topic="sources/maestro/consumers/+/agentevents"
fi

rest_api_schema="http"
if [ "$tls_enable" = "true" ]; then
  rest_api_schema="https"
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
  make deploy-service-tls
else
  make deploy-service
fi

kubectl wait deploy/maestro -n $namespace --for condition=Available=True --timeout=300s

# TODO use maestro service health check to ensure the service ready
sleep 30 # wait 30 seconds for the service ready

# Expose the RESTAPI and gRPC service hosts
echo "${rest_api_schema}://127.0.0.1:30080" > ${PWD}/test/_output/.external_restapi_endpoint
echo "127.0.0.1:30090" > ${PWD}/test/_output/.external_grpc_endpoint
