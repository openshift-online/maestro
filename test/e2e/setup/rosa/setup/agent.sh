#!/usr/bin/env bash

#####################
# Setup Maestro agent
#####################

PWD="$(cd "$(dirname ${BASH_SOURCE[0]})" ; pwd -P)"
ROOT_DIR="$(cd ${PWD}/.. && pwd -P)"

region=${REGION:-""}
consumer_id=${CONSUMER_ID:-""}

if [ -z "$region" ]; then
    echo "region is required"
    exit 1
fi

if [ -z "$consumer_id" ]; then
    echo "consumer id is required"
    exit 1
fi

echo "Setup Maestro agent in ${region} (consumer_id=${consumer_id})"

IMAGE_REGISTRY=${IMAGE_REGISTRY:="quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro"}
IMAGE_REPOSITORY="maestro"
IMAGE_TAG=${IMAGE_TAG:-"1de63c6075f2c95c9661d790d164019f60d789f3"}

output_dir=${ROOT_DIR}/_output
certs_dir=${output_dir}/aws-certs
consumer_cert_dir=${certs_dir}/iot/consumers
policies_dir=${output_dir}/aws-policies

mkdir -p ${consumer_cert_dir}
mkdir -p ${policies_dir}

# Download AWS IoT broker severing CA
echo "Download AWS IoT broker severing CA ...."
curl -s -o ${certs_dir}/iot-ca.pem https://www.amazontrust.com/repository/AmazonRootCA1.pem

# Generated client certs for AWS IoT clients
echo "Generate AWS IoT client certs for Maestro agent ...."
consumer_cert_arn=$(aws iot create-keys-and-certificate \
    --region ${region} \
    --set-as-active \
    --certificate-pem-outfile "${consumer_cert_dir}/${consumer_id}.crt" \
    --public-key-outfile "${consumer_cert_dir}/${consumer_id}.public.key" \
    --private-key-outfile "${consumer_cert_dir}/${consumer_id}.private.key" | jq -r '.certificateArn')
echo "Maestro agent AWS IoT client certs are generated ($consumer_cert_arn)"

# Attach policies for AWS IoT clients
aws_account=$(aws sts get-caller-identity --region ${region} --output json | jq -r '.Account')

echo "Generate AWS IoT policy for Maestro agent ...."
cat $PWD/aws-iot-policies/consumer.template.json | sed "s/{region}/${region}/g" | sed "s/{aws_account}/${aws_account}/g" | sed "s/{consumer_id}/${consumer_id}/g" > $policies_dir/${consumer_id}.json
policy_name=$(aws iot create-policy \
    --region ${region} \
    --policy-name maestro-${consumer_id} \
    --policy-document "file://${policies_dir}/${consumer_id}.json" | jq -r '.policyName')
aws iot attach-policy --region ${region} --policy-name maestro-${consumer_id} --target ${consumer_cert_arn}
echo "Maestro agent AWS IoT policy $policy_name is generated"

# Get AWS IoT broker endpoint
mqtt_host=$(aws iot describe-endpoint --region ${region} --endpoint-type iot:Data-ATS | jq -r '.endpointAddress')
echo "AWS IoT broke: ${mqtt_host}:8883"

sleep 30

# Deploy Maestro agent
oc create namespace maestro-agent || true
oc -n maestro-agent delete secrets maestro-agent-mqtt-creds --ignore-not-found
oc -n maestro-agent create secret generic maestro-agent-mqtt-creds \
    --from-file=ca.crt="${certs_dir}/iot-ca.pem" \
    --from-file=client.crt="${consumer_cert_dir}/${consumer_id}.crt" \
    --from-file=client.key="${consumer_cert_dir}/${consumer_id}.private.key"

oc process --filename="https://raw.githubusercontent.com/openshift-online/maestro/refs/heads/main/templates/agent-template-rosa.yml" \
    --local="true" \
    --param="AGENT_NAMESPACE=maestro-agent" \
    --param="CONSUMER_NAME=${consumer_id}" \
    --param="IMAGE_REGISTRY=${IMAGE_REGISTRY}" \
    --param="IMAGE_REPOSITORY=${IMAGE_REPOSITORY}" \
    --param="IMAGE_TAG=${IMAGE_TAG}" \
    --param="MQTT_HOST=${mqtt_host}" > ${output_dir}/maestro-${consumer_id}-rosa.json

oc apply -f ${output_dir}/maestro-${consumer_id}-rosa.json
oc -n maestro-agent wait deploy/maestro-agent --for condition=Available=True --timeout=300s
