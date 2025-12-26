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
    --private-key-outfile "${consumer_cert_dir}/${consumer_id}.private.key" --output json | jq -r '.certificateArn')
echo "Maestro agent AWS IoT client certs are generated ($consumer_cert_arn)"

# Attach policies for AWS IoT clients
aws_account=$(aws sts get-caller-identity --region ${region} --output json | jq -r '.Account')

echo "Generate AWS IoT policy for Maestro agent ...."
cat $PWD/aws-iot-policies/consumer.template.json | sed "s/{region}/${region}/g" | sed "s/{aws_account}/${aws_account}/g" | sed "s/{consumer_id}/${consumer_id}/g" > $policies_dir/${consumer_id}.json
policy_name=$(aws iot create-policy \
    --region ${region} \
    --policy-name maestro-${consumer_id} \
    --policy-document "file://${policies_dir}/${consumer_id}.json" --output json | jq -r '.policyName')
aws iot attach-policy --region ${region} --policy-name maestro-${consumer_id} --target ${consumer_cert_arn}
echo "Maestro agent AWS IoT policy $policy_name is generated"

# Get AWS IoT broker endpoint
mqtt_host=$(aws iot describe-endpoint --region ${region} --endpoint-type iot:Data-ATS --output json | jq -r '.endpointAddress')
echo "AWS IoT broke: ${mqtt_host}:8883"

sleep 30

# Deploy Maestro agent
oc create namespace maestro-agent || true
oc -n maestro-agent delete secrets maestro-agent-certs --ignore-not-found
oc -n maestro-agent create secret generic maestro-agent-certs \
    --from-file=ca.crt="${certs_dir}/iot-ca.pem" \
    --from-file=client.crt="${consumer_cert_dir}/${consumer_id}.crt" \
    --from-file=client.key="${consumer_cert_dir}/${consumer_id}.private.key"

# Create Helm values file for maestro-agent
cat > ${output_dir}/maestro-agent-values.yaml <<EOF
consumerName: ${consumer_id}

cloudeventsClientId: ${consumer_id}

environment: production

messageBroker:
  type: mqtt
  mqtt:
    host: ${mqtt_host}
    user: ""
    port: "8883"
    rootCert: /secrets/mqtt-certs/ca.crt
    clientCert: /secrets/mqtt-certs/client.crt
    clientKey: /secrets/mqtt-certs/client.key
EOF

# Deploy Maestro agent using Helm
PROJECT_DIR="$(cd ${ROOT_DIR}/../../../.. && pwd -P)"
helm upgrade --install maestro-agent \
    ${PROJECT_DIR}/charts/maestro-agent \
    --namespace maestro-agent \
    --create-namespace \
    --values ${output_dir}/maestro-agent-values.yaml
oc -n maestro-agent wait deploy/maestro-agent --for condition=Available=True --timeout=300s
