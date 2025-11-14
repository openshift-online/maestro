#!/usr/bin/env bash

#####################
# Setup Maestro e2e
#####################

PWD="$(cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1; pwd -P)"
GCP_DIR="$(cd "${PWD}/.." || exit 1; pwd -P)"

output_dir="${GCP_DIR}/_output"
mkdir -p "${output_dir}"
echo "${output_dir}"

project_id=${PROJECT_ID:-""}
region=${REGION:-""}
consumer_id=${CONSUMER_ID:-""}
cluster_id=${CLUSTER_ID:-""}

if [ -z "$project_id" ]; then
    echo "project id is required"
    exit 1
fi

if [ -z "$region" ]; then
    echo "region is required"
    exit 1
fi

if [ -z "$consumer_id" ]; then
    echo "consumer id is required"
    exit 1
fi

if [ -z "$cluster_id" ]; then
    echo "GKE cluster id is required"
    exit 1
fi

# Get credential to access GKE cluster
gcloud container clusters get-credentials ${cluster_id} --region ${region} --project=${project_id}

# Setup Maestro server
${PWD}/maestro.sh
sleep 90 # wait the maestro service ready

# Start Maestro servers
oc relay service/maestro 8000:8000 -n maestro > "${output_dir}/maestro.svc.log" 2>&1 &
maestro_server_pid=$!
echo "Maestro server started: $maestro_server_pid"
echo "$maestro_server_pid" > ${output_dir}/maestro_server.pid
oc relay service/maestro-grpc 8090:8090 -n maestro > "${output_dir}/maestro-grpc.svc.log" 2>&1 &
maestro_grpc_server_pid=$!
echo "Maestro GRPC server started: $maestro_grpc_server_pid"
echo "$maestro_grpc_server_pid" > ${output_dir}/maestro_grpc_server.pid

# need to wait the relay build the connection before we get the consumer id
sleep 15

# Prepare a consumer
created_consumer_id=$(curl -s -X POST -H "Content-Type: application/json" http://127.0.0.1:8000/api/maestro/v1/consumers -d "{\"name\": \"${consumer_id}\"}" | jq -r '.name')
echo $created_consumer_id > ${output_dir}/consumer_id
echo "Consumer $created_consumer_id is created"

# Setup Maestro agent
oc apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml
CONSUMER_ID=$created_consumer_id ${PWD}/agent.sh

sleep 90 # wait the maestro agent ready
