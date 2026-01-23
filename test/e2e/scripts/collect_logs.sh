#!/bin/bash
#
# Copyright (c) 2025 Red Hat, Inc.
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

# Script to collect logs from maestro server and agent pods for debugging

set +e  # Don't exit on errors, we want to collect as much as possible

export KUBECONFIG=${KUBECONFIG:-${PWD}/test/_output/.kubeconfig}
LOG_DIR=${1:-/tmp/maestro-logs}

echo "Collecting maestro logs to ${LOG_DIR}..."
mkdir -p "${LOG_DIR}"

# Collect pod status
echo "Collecting pod status..."
kubectl get pods -A -o wide > "${LOG_DIR}/pods-all.txt" 2>&1 || true
kubectl get pods -n maestro -o wide > "${LOG_DIR}/pods-maestro.txt" 2>&1 || true
kubectl get pods -n maestro-agent -o wide > "${LOG_DIR}/pods-maestro-agent.txt" 2>&1 || true

# Collect pod descriptions
echo "Collecting pod descriptions..."
kubectl describe pods -n maestro > "${LOG_DIR}/describe-pods-maestro.txt" 2>&1 || true
kubectl describe pods -n maestro-agent > "${LOG_DIR}/describe-pods-maestro-agent.txt" 2>&1 || true

# Collect logs from maestro server pods
echo "Collecting logs from maestro server pods..."
for pod in $(kubectl get pods -n maestro -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
  echo "  - Collecting logs from $pod"
  kubectl logs -n maestro "$pod" --all-containers=true --prefix=true > "${LOG_DIR}/${pod}.log" 2>&1 || true
  # Also get previous logs if pod restarted
  kubectl logs -n maestro "$pod" --all-containers=true --prefix=true --previous > "${LOG_DIR}/${pod}-previous.log" 2>&1 || true
done

# Collect logs from maestro agent pods
echo "Collecting logs from maestro agent pods..."
for pod in $(kubectl get pods -n maestro-agent -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
  echo "  - Collecting logs from $pod"
  kubectl logs -n maestro-agent "$pod" --all-containers=true --prefix=true > "${LOG_DIR}/${pod}.log" 2>&1 || true
  # Also get previous logs if pod restarted
  kubectl logs -n maestro-agent "$pod" --all-containers=true --prefix=true --previous > "${LOG_DIR}/${pod}-previous.log" 2>&1 || true
done

# Collect resource status
echo "Collecting resource status..."
kubectl get all -n maestro -o yaml > "${LOG_DIR}/resources-maestro.yaml" 2>&1 || true
kubectl get all -n maestro-agent -o yaml > "${LOG_DIR}/resources-maestro-agent.yaml" 2>&1 || true

# Collect manifest status
kubectl get all -n default -o yaml > "${LOG_DIR}/resources-default.yaml" 2>&1 || true
kubectl get deployment -n default -o yaml > "${LOG_DIR}/deployments-default.yaml" 2>&1 || true

# Collect appliedmanifestworks status
kubectl get appliedmanifestworks.work.open-cluster-management.io -o yaml > "${LOG_DIR}/appliedmanifestworks.yaml" 2>&1 || true

echo "Log collection complete. Logs saved to ${LOG_DIR}"
