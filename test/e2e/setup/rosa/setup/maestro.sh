#!/usr/bin/env bash

######################
# Setup Maestro server
######################

PWD="$(cd "$(dirname ${BASH_SOURCE[0]})" ; pwd -P)"
ROSA_DIR="$(cd ${PWD}/.. && pwd -P)"

region=${REGION:-""}
cluster_id=${CLUSTER_ID:-""}

if [ -z "$region" ]; then
    echo "region is required"
    exit 1
fi

if [ -z "$cluster_id" ]; then
    echo "cluster id is required"
    exit 1
fi

# Find Maestro server vpc
rosa_infra_id=$(rosa describe cluster --region=${region} --cluster=${cluster_id} -ojson | jq -r '.infra_id')
vpc=$(aws ec2 describe-vpcs --region=${region} \
    --filters Name=tag:Name,Values=${rosa_infra_id}-vpc | jq -r '.Vpcs[0].VpcId')

echo "Setup Maestro in ${region} (cluster=$cluster_id,vpc=$vpc)"

IMAGE_REGISTRY=${IMAGE_REGISTRY:-"quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro"}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-"maestro"}
IMAGE_TAG=${IMAGE_TAG:-"1de63c6075f2c95c9661d790d164019f60d789f3"}

output_dir=${ROSA_DIR}/_output
certs_dir=${output_dir}/aws-certs
source_cert_dir=${certs_dir}/iot/source
policies_dir=${output_dir}/aws-policies

source_id="maestro"

mkdir -p ${source_cert_dir}
mkdir -p ${policies_dir}

db_pw=$(LC_CTYPE=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 16)
echo "$db_pw" > $output_dir/db.password

# Download AWS IoT broker severing CA
echo "Download AWS IoT broker and database severing CA ...."
curl -s -o ${certs_dir}/iot-ca.pem https://www.amazontrust.com/repository/AmazonRootCA1.pem
curl -s -o ${certs_dir}/db-ca.pem "https://truststore.pki.rds.amazonaws.com/${region}/${region}-bundle.pem"

# Generate client certs for AWS IoT clients
echo "Generate AWS IoT client certs for Maestro ...."
maestro_cert_arn=$(aws iot create-keys-and-certificate \
    --region ${region} \
    --set-as-active \
    --certificate-pem-outfile "${source_cert_dir}/${source_id}.crt" \
    --public-key-outfile "${source_cert_dir}/${source_id}.public.key" \
    --private-key-outfile "${source_cert_dir}/${source_id}.private.key" | jq -r '.certificateArn')
echo "Mastro AWS IoT client certs are generated ($maestro_cert_arn)"

# Attach policies for AWS IoT clients
echo "Generate AWS IoT policy for Maestro ...."
aws_account=$(aws sts get-caller-identity --region ${region} --output json | jq -r '.Account')

cat $PWD/aws-iot-policies/source.template.json | sed "s/{region}/${region}/g" | sed "s/{aws_account}/${aws_account}/g" > $policies_dir/source.json
policy_name=$(aws iot create-policy \
    --region ${region} \
    --policy-name ${source_id} \
    --policy-document "file://${policies_dir}/source.json" | jq -r '.policyName')
aws iot attach-policy --region ${region} --policy-name ${source_id} --target ${maestro_cert_arn}
echo "Maestro AWS IoT policy $policy_name is generated"

# Allow AWS PostgrepSQL connection in the default security group
echo "Prepare AWS RDS PostgrepSQL for Maestro in ${region} (${vpc}) ...."
sg=$(aws ec2 get-security-groups-for-vpc \
    --region ${region} \
    --vpc-id ${vpc} \
    --query "SecurityGroupForVpcs[?GroupName=='default'].GroupId" | jq -r '.[0]')
result=$(aws ec2 authorize-security-group-ingress \
    --region ${region} \
    --group-id ${sg} \
    --protocol tcp --port 5432 --cidr 0.0.0.0/0 | jq -r '.Return')
echo "PostgrepSQL inbound rule is added to ${sg} (${result})"

# Create a database subnet group for AWS PostgrepSQL
subnets=""
subnets_counts=0
for subnet in $(aws ec2 describe-subnets --region ${region} --filters "Name=vpc-id,Values=${vpc}" | jq -r '.Subnets[].SubnetId'); do
    subnets="$subnets,\"$subnet\""
    subnets_counts=$((subnets_counts+1))
done

if [ $subnets_counts -le 2 ]; then
    # The DB subnet group doesn't meet Availability Zone (AZ) coverage requirement. Current AZ coverage: us-west-2a. Add subnets to cover at least 2 AZs.
    current_az=$(aws ec2 describe-subnets --region ${region} --filters "Name=vpc-id,Values=${vpc}" | jq -r '.Subnets[0].AvailabilityZone')
    for az in $(aws ec2 describe-availability-zones --region=${region} | jq -r '.AvailabilityZones[].ZoneName'); do
        if [[ "$az" != "$current_az" ]]; then
            subnet=$(aws ec2 create-subnet \
                --region=${region} \
                --vpc-id ${vpc} \
                --availability-zone ${az} \
                --cidr-block 10.0.64.0/18 \
                --tag-specifications "ResourceType=subnet,Tags=[{Key=Name,Value=maestro-db-subnet-${az}}]" | jq -r '.Subnet.SubnetId')
            subnets="$subnets,\"$subnet\""
            break
        fi
    done
fi

db_subnet_group=$(aws rds create-db-subnet-group \
    --region ${region} \
    --db-subnet-group-name maestrosubnetgroup \
    --db-subnet-group-description "Maestro DB subnet group" \
    --subnet-ids "[${subnets:1}]" | jq -r '.DBSubnetGroup.DBSubnetGroupName')
echo "PostgrepSQL subnet group ${db_subnet_group} is created"

# Create AWS PostgrepSQL
db_id=$(aws rds create-db-instance \
    --region ${region} \
    --engine postgres \
    --allocated-storage 20 \
    --db-instance-class db.t4g.large \
    --db-subnet-group-name ${db_subnet_group} \
    --db-instance-identifier maestro \
    --db-name maestro \
    --master-username maestro \
    --master-user-password "${db_pw}" | jq -r '.DBInstance.DBInstanceIdentifier')
db_id=maestro
i=1
while [ $i -le 20 ]
do
    db_status=$(aws rds describe-db-instances --region ${region} --db-instance-identifier ${db_id} | jq -r '.DBInstances[0].DBInstanceStatus')
    echo "[$i] DB status: ${db_status}"
    if [[ "$db_status" == "available" ]]; then
        break
    fi
    i=$((i + 1))
    sleep 30
done

# Get AWS IoT broker and PostgrepSQL endpoints
mqtt_host=$(aws iot describe-endpoint --region ${region} --endpoint-type iot:Data-ATS | jq -r '.endpointAddress')
db_host=$(aws rds describe-db-instances --region ${region} --db-instance-identifier ${db_id} | jq -r '.DBInstances[0].Endpoint.Address')
echo "AWS IoT broke: ${mqtt_host}:8883"
echo "AWS RDS PostgreSQL: ${db_host}:5432 (${db_id})"

# Deploy Maestro server
oc create namespace maestro || true

oc -n maestro delete secrets maestro-server-certs --ignore-not-found
oc -n maestro create secret generic maestro-server-certs \
    --from-file=ca.crt="${certs_dir}/iot-ca.pem" \
    --from-file=client.crt="${source_cert_dir}/maestro.crt" \
    --from-file=client.key="${source_cert_dir}/maestro.private.key"

oc -n maestro delete secret maestro-db --ignore-not-found
oc -n maestro create secret generic maestro-db \
    --from-literal=db.name=maestro \
    --from-literal=db.host=${db_host} \
    --from-literal=db.port=5432 \
    --from-literal=db.user=maestro \
    --from-literal=db.password="${db_pw}" \
    --from-file=db.ca_cert="${certs_dir}/db-ca.pem"

# Create Helm values file for maestro-server
cat > ${output_dir}/maestro-server-values.yaml <<EOF
image:
  registry: ${IMAGE_REGISTRY%/*}
  repository: ${IMAGE_REGISTRY#*/}/${IMAGE_REPOSITORY}
  tag: ${IMAGE_TAG}

replicas: 3

environment: production

database:
  secretName: maestro-db
  sslMode: verify-full
  maxOpenConnections: 50

messageBroker:
  type: mqtt

mqtt:
  enabled: false
  host: ${mqtt_host}
  tls:
    enabled: true
    caFile: /secrets/mqtt-creds/ca.crt
    clientCertFile: /secrets/mqtt-creds/client.crt
    clientKeyFile: /secrets/mqtt-creds/client.key

server:
  https:
    enabled: false
  grpc:
    enabled: true
  hostname: ""

service:
  api:
    type: ClusterIP

route:
  enabled: true
  tls:
    enabled: true
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
EOF

# Deploy Maestro server using Helm
PROJECT_DIR="$(cd ${ROSA_DIR}/../.. && pwd -P)"
helm upgrade --install maestro-server \
    ${PROJECT_DIR}/charts/maestro-server \
    --namespace maestro \
    --create-namespace \
    --values ${output_dir}/maestro-server-values.yaml
oc -n maestro wait deploy/maestro --for condition=Available=True --timeout=300s
