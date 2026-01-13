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
vpc=$(aws ec2 describe-vpcs --region=${region} --filters Name=tag:Name,Values=${rosa_infra_id}-vpc --output json | jq -r '.Vpcs[0].VpcId')

echo "Setup Maestro in ${region} (cluster=$cluster_id,vpc=$vpc)"

output_dir=${ROSA_DIR}/_output
certs_dir=${output_dir}/aws-certs
source_cert_dir=${certs_dir}/iot/source
policies_dir=${output_dir}/aws-policies

source_id="maestro"

mkdir -p ${source_cert_dir}
mkdir -p ${policies_dir}

# Generate or retrieve database password
if [ -f "$output_dir/db.password" ]; then
    echo "Using existing database password from $output_dir/db.password"
    db_pw=$(cat "$output_dir/db.password")
else
    echo "Generating new database password..."
    db_pw=$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9' | head -c 16)
    echo -n "$db_pw" > $output_dir/db.password
fi

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
    --private-key-outfile "${source_cert_dir}/${source_id}.private.key" --output json | jq -r '.certificateArn')
echo "Mastro AWS IoT client certs are generated ($maestro_cert_arn)"

# Attach policies for AWS IoT clients
echo "Generate AWS IoT policy for Maestro ...."
aws_account=$(aws sts get-caller-identity --region ${region} --output json | jq -r '.Account')

cat $PWD/aws-iot-policies/source.template.json | sed "s/{region}/${region}/g" | sed "s/{aws_account}/${aws_account}/g" > $policies_dir/source.json

# Check if policy already exists
policy_name=${source_id}
if aws iot get-policy --region ${region} --policy-name ${source_id} >/dev/null 2>&1; then
    echo "IoT policy ${source_id} already exists, skipping creation"
else
    echo "Creating IoT policy ${source_id}..."
    if aws iot create-policy \
        --region ${region} \
        --policy-name ${source_id} \
        --policy-document "file://${policies_dir}/source.json" >/dev/null 2>&1; then
        echo "Maestro AWS IoT policy ${policy_name} is created"
    else
        echo "Failed to create IoT policy (may already exist), continuing..."
    fi
fi

# Attach policy to certificate (ignore if already attached)
aws iot attach-policy --region ${region} --policy-name ${source_id} --target ${maestro_cert_arn} 2>/dev/null || echo "Policy already attached or attachment failed (non-critical)"

# Allow AWS PostgrepSQL connection in the default security group
echo "Prepare AWS RDS PostgrepSQL for Maestro in ${region} (${vpc}) ...."
sg=$(aws ec2 get-security-groups-for-vpc \
    --region ${region} \
    --vpc-id ${vpc} \
    --query "SecurityGroupForVpcs[?GroupName=='default'].GroupId" --output json | jq -r '.[0]')

# Add security group rule (ignore if already exists)
aws ec2 authorize-security-group-ingress \
    --region ${region} \
    --group-id ${sg} \
    --protocol tcp --port 5432 --cidr 0.0.0.0/0 2>/dev/null && \
    echo "PostgrepSQL inbound rule is added to ${sg}" || \
    echo "PostgrepSQL inbound rule already exists in ${sg}"

# Create a database subnet group for AWS PostgrepSQL
# Collect all existing subnets and track unique AZs
subnets=""
declare -a unique_azs
for subnet_data in $(aws ec2 describe-subnets --region ${region} --filters "Name=vpc-id,Values=${vpc}" --output json | jq -c '.Subnets[] | {id: .SubnetId, az: .AvailabilityZone}'); do
    subnet_id=$(echo "$subnet_data" | jq -r '.id')
    subnet_az=$(echo "$subnet_data" | jq -r '.az')
    subnets="$subnets,\"$subnet_id\""

    # Track unique AZs
    if [[ ! " ${unique_azs[@]} " =~ " ${subnet_az} " ]]; then
        unique_azs+=("$subnet_az")
    fi
done

echo "Found ${#unique_azs[@]} unique AZ(s): ${unique_azs[*]}"

# RDS requires at least 2 AZs for DB subnet group
if [ ${#unique_azs[@]} -lt 2 ]; then
    echo "DB subnet group requires at least 2 AZs. Creating additional subnet..."

    # Get VPC CIDR to determine available IP ranges
    vpc_cidr=$(aws ec2 describe-vpcs --region=${region} --vpc-ids ${vpc} --output json | jq -r '.Vpcs[0].CidrBlock')
    echo "VPC CIDR: $vpc_cidr"

    # Find an AZ that doesn't have a subnet yet
    target_az=""
    for az in $(aws ec2 describe-availability-zones --region=${region} --output json | jq -r '.AvailabilityZones[].ZoneName'); do
        if [[ ! " ${unique_azs[@]} " =~ " ${az} " ]]; then
            target_az="$az"
            break
        fi
    done

    if [ -z "$target_az" ]; then
        echo "ERROR: Could not find an available AZ for new subnet"
        exit 1
    fi

    echo "Creating new subnet in AZ: $target_az"

    # Try multiple CIDR blocks to avoid conflicts
    subnet_created=false
    for cidr in "10.0.64.0/18" "10.0.128.0/18" "10.0.192.0/18" "172.31.64.0/20" "172.31.80.0/20" "192.168.64.0/20"; do
        subnet=$(aws ec2 create-subnet \
            --region=${region} \
            --vpc-id ${vpc} \
            --availability-zone ${target_az} \
            --cidr-block ${cidr} \
            --tag-specifications "ResourceType=subnet,Tags=[{Key=Name,Value=maestro-db-subnet-${target_az}}]" 2>&1 | jq -r '.Subnet.SubnetId // empty')

        if [[ -n "$subnet" && "$subnet" != "null" ]]; then
            echo "Successfully created subnet $subnet in $target_az with CIDR $cidr"
            subnets="$subnets,\"$subnet\""
            subnet_created=true
            break
        fi
    done

    if [ "$subnet_created" = false ]; then
        echo "ERROR: Failed to create subnet in $target_az. All CIDR blocks conflicted or failed."
        echo "Please manually create a subnet in a different AZ and re-run the script."
        exit 1
    fi
fi

# Create or update DB subnet group
db_subnet_group_name="maestrosubnetgroup"
echo "Checking DB subnet group ${db_subnet_group_name}..."

# Check if subnet group already exists
existing_group=$(aws rds describe-db-subnet-groups \
    --region ${region} \
    --db-subnet-group-name ${db_subnet_group_name} \
    --output json 2>/dev/null | jq -r '.DBSubnetGroups[0].DBSubnetGroupName // empty')

if [[ -n "$existing_group" ]]; then
    echo "DB subnet group ${db_subnet_group_name} already exists."

    # Verify it has multi-AZ coverage
    subnet_azs=$(aws rds describe-db-subnet-groups \
        --region ${region} \
        --db-subnet-group-name ${db_subnet_group_name} --output json 2>/dev/null | \
        jq -r '.DBSubnetGroups[0].Subnets[].SubnetAvailabilityZone.Name' | sort -u | wc -l)

    if [[ "$subnet_azs" -ge 2 ]]; then
        echo "DB subnet group has adequate multi-AZ coverage ($subnet_azs AZs). Using existing group."
        db_subnet_group=${db_subnet_group_name}
    else
        echo "DB subnet group only covers $subnet_azs AZ. Attempting to delete and recreate..."
        if aws rds delete-db-subnet-group --region ${region} --db-subnet-group-name ${db_subnet_group_name} 2>/dev/null; then
            sleep 5
            echo "Deleted old subnet group, creating new one..."
            db_subnet_group=$(aws rds create-db-subnet-group \
                --region ${region} \
                --db-subnet-group-name ${db_subnet_group_name} \
                --db-subnet-group-description "Maestro DB subnet group" \
                --subnet-ids "[${subnets:1}]" 2>&1 | jq -r '.DBSubnetGroup.DBSubnetGroupName // empty')
        else
            echo "ERROR: Cannot delete subnet group (may be in use). Please delete it manually."
            exit 1
        fi
    fi
else
    echo "Creating new DB subnet group ${db_subnet_group_name}..."
    db_subnet_group=$(aws rds create-db-subnet-group \
        --region ${region} \
        --db-subnet-group-name ${db_subnet_group_name} \
        --db-subnet-group-description "Maestro DB subnet group" \
        --subnet-ids "[${subnets:1}]" 2>&1 | jq -r '.DBSubnetGroup.DBSubnetGroupName // empty')

    if [[ -z "$db_subnet_group" || "$db_subnet_group" == "null" ]]; then
        echo "ERROR: Failed to create DB subnet group"
        exit 1
    fi
fi

echo "PostgrepSQL subnet group ${db_subnet_group} is ready"

# Create AWS PostgrepSQL
db_instance_id="maestro"
echo "Creating RDS PostgreSQL instance ${db_instance_id}..."

# Check if DB instance already exists
if aws rds describe-db-instances --region ${region} --db-instance-identifier ${db_instance_id} --output json >/dev/null 2>&1; then
    echo "DB instance ${db_instance_id} already exists. Using existing instance."
    db_id=${db_instance_id}
else
    echo "Creating new DB instance ${db_instance_id}..."

    # Create DB instance and capture output
    create_output=$(aws rds create-db-instance \
        --region ${region} \
        --engine postgres \
        --allocated-storage 20 \
        --db-instance-class db.t4g.large \
        --db-subnet-group-name ${db_subnet_group} \
        --db-instance-identifier ${db_instance_id} \
        --db-name maestro \
        --master-username maestro \
        --master-user-password "${db_pw}" \
        --output json 2>&1)

    # Check if creation was successful
    if echo "$create_output" | jq -e '.DBInstance.DBInstanceIdentifier' >/dev/null 2>&1; then
        db_id=$(echo "$create_output" | jq -r '.DBInstance.DBInstanceIdentifier')
        echo "DB instance ${db_id} creation initiated successfully"
    else
        echo "ERROR: Failed to create DB instance"
        echo "Error output: $create_output"
        exit 1
    fi
fi

# Wait for DB to become available
echo "Waiting for DB instance to become available..."
i=1
while [ $i -le 20 ]
do
    db_status=$(aws rds describe-db-instances \
        --region ${region} \
        --db-instance-identifier ${db_id} \
        --output json 2>/dev/null | jq -r '.DBInstances[0].DBInstanceStatus // "unknown"')

    echo "[$i] DB status: ${db_status}"
    if [[ "$db_status" == "available" ]]; then
        break
    fi
    i=$((i + 1))
    sleep 30
done

# Get AWS IoT broker and PostgrepSQL endpoints
mqtt_host=$(aws iot describe-endpoint --region ${region} --endpoint-type iot:Data-ATS --output json | jq -r '.endpointAddress')
db_host=$(aws rds describe-db-instances --region ${region} --db-instance-identifier ${db_id} --output json | jq -r '.DBInstances[0].Endpoint.Address')
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

replicas: 3

environment: production

database:
  secretName: maestro-db
  sslMode: verify-full
  maxOpenConnections: 50

messageBroker:
  type: mqtt

  mqtt:
    host: ${mqtt_host}
    port: 8883
    topics:
      sourceEvents: sources/maestro/consumers/+/sourceevents
      agentEvents: "\$share/statussubscribers/sources/maestro/consumers/+/agentevents"
    tls:
      enabled: true
      caFile: /secrets/mqtt-certs/ca.crt
      clientCertFile: /secrets/mqtt-certs/client.crt
      clientKeyFile: /secrets/mqtt-certs/client.key
server:
  https:
    enabled: false
  hostname: ""

service:
  api:
    type: ClusterIP

route:
  enabled: false
EOF

# Deploy Maestro server using Helm
PROJECT_DIR="$(cd ${ROSA_DIR}/../../../.. && pwd -P)"
helm upgrade --install maestro-server \
    ${PROJECT_DIR}/charts/maestro-server \
    --namespace maestro \
    --create-namespace \
    --values ${output_dir}/maestro-server-values.yaml
oc -n maestro wait deploy/maestro --for condition=Available=True --timeout=300s
