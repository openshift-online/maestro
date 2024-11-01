#!/usr/bin/env bash

#####################
# Setup Maestro e2e
#####################

PWD="$(cd "$(dirname ${BASH_SOURCE[0]})" ; pwd -P)"
ROSA_DIR="$(cd ${PWD}/.. && pwd -P)"

output_dir=${ROSA_DIR}/_output

mkdir -p $output_dir

echo "$output_dir"

# Setup Maestro server
CLUSTER_VPC=$vpc ${PWD}/maestro.sh
sleep 90 # wait the maestro service ready

# Start Maestro servers
exec oc relay service/maestro 8000:8000 -n maestro > ${output_dir}/maestro.svc.log 2>&1 &
maestro_server_pid=$!
echo "Maestro server started: $maestro_server_pid"
echo "$maestro_server_pid" > ${output_dir}/maestro_server.pid
exec oc relay service/maestro-grpc 8090:8090 -n maestro > ${output_dir}/maestro-grpc.svc.log 2>&1 &
maestro_grpc_server_pid=$!
echo "Maestro GRPC server started: $maestro_grpc_server_pid"
echo "$maestro_grpc_server_pid" > ${output_dir}/maestro_grpc_server.pid

# need to wait the relay build the connection before we get the consumer id
sleep 15

# Prepare a consumer
consumer_id=$(curl -s -X POST -H "Content-Type: application/json" http://127.0.0.1:8000/api/maestro/v1/consumers -d '{}' | jq -r '.id')
echo $consumer_id > ${output_dir}/consumer_id
echo "Consumer $consumer_id is created"

# Setup Maestro agent
oc apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml
CONSUMER_ID=$consumer_id ${PWD}/agent.sh
