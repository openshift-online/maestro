#!/usr/bin/env bash

######################
# Teardown Maestro server and AWS resources
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

echo "Tearing down Maestro in ${region} (cluster=$cluster_id)"

# Find VPC for cleanup operations
rosa_infra_id=$(rosa describe cluster --region=${region} --cluster=${cluster_id} -ojson 2>/dev/null | jq -r '.infra_id // empty')
if [[ -n "$rosa_infra_id" ]]; then
    vpc=$(aws ec2 describe-vpcs --region=${region} --filters Name=tag:Name,Values=${rosa_infra_id}-vpc --output json 2>/dev/null | jq -r '.Vpcs[0].VpcId // empty')
    echo "Found VPC: $vpc"
fi

# 1. Uninstall Helm deployment
echo "Removing Maestro Helm deployment..."
helm uninstall maestro-server --namespace maestro 2>/dev/null && echo "Helm deployment removed" || echo "Helm deployment not found or already removed"

# 2. Delete Kubernetes secrets
echo "Removing Kubernetes secrets..."
oc -n maestro delete secret maestro-server-certs --ignore-not-found
oc -n maestro delete secret maestro-db --ignore-not-found
echo "Kubernetes secrets removed"

# 3. Delete Kubernetes namespace
echo "Removing Kubernetes namespace..."
oc delete namespace maestro --ignore-not-found
echo "Kubernetes namespace removed"

# 4. Delete AWS PostgreSQL
echo "Deleting RDS instance..."
db_status=$(aws rds delete-db-instance --region ${region} --db-instance-identifier maestro --skip-final-snapshot --delete-automated-backups --output json 2>/dev/null | jq -r '.DBInstance.DBInstanceStatus // "not-found"')
if [[ "$db_status" != "not-found" ]]; then
    echo "Deleting maestro db ($db_status)"

    i=1
    while [ $i -le 20 ]
    do
        db_status=$(aws rds describe-db-instances --region ${region} --db-instance-identifier maestro --output json 2>/dev/null | jq -r '.DBInstances[0].DBInstanceStatus // empty')
        if [[ -z "$db_status" ]]; then
            echo "RDS instance deleted"
            break
        fi
        echo "[$i] DB status: ${db_status}"
        i=$((i + 1))
        sleep 30
    done
else
    echo "RDS instance not found or already deleted"
fi

# 5. Delete DB subnet group
echo "Deleting DB subnet group..."
aws rds delete-db-subnet-group --region ${region} --db-subnet-group-name maestrosubnetgroup 2>/dev/null && \
    echo "DB subnet group removed" || \
    echo "DB subnet group not found or already removed"

# 6. Delete maestro-created subnets
echo "Deleting maestro-created subnets..."
if [[ -n "$vpc" ]]; then
    for subnet_id in $(aws ec2 describe-subnets --region ${region} --filters "Name=vpc-id,Values=${vpc}" "Name=tag:Name,Values=maestro-db-subnet-*" --output json 2>/dev/null | jq -r '.Subnets[].SubnetId // empty'); do
        if [[ -n "$subnet_id" ]]; then
            echo "Deleting subnet: $subnet_id"
            aws ec2 delete-subnet --region ${region} --subnet-id ${subnet_id} 2>/dev/null && \
                echo "Subnet $subnet_id deleted" || \
                echo "Failed to delete subnet $subnet_id (may be in use)"
        fi
    done
else
    echo "VPC not found, skipping subnet cleanup"
fi

# 7. Remove PostgreSQL security group rule
echo "Removing PostgreSQL security group rule..."
if [[ -n "$vpc" ]]; then
    sg=$(aws ec2 get-security-groups-for-vpc \
        --region ${region} \
        --vpc-id ${vpc} \
        --query "SecurityGroupForVpcs[?GroupName=='default'].GroupId" --output json 2>/dev/null | jq -r '.[0] // empty')

    if [[ -n "$sg" ]]; then
        aws ec2 revoke-security-group-ingress \
            --region ${region} \
            --group-id ${sg} \
            --protocol tcp --port 5432 --cidr 0.0.0.0/0 2>/dev/null && \
            echo "PostgreSQL security group rule removed from ${sg}" || \
            echo "PostgreSQL security group rule not found or already removed"
    else
        echo "Default security group not found"
    fi
else
    echo "VPC not found, skipping security group cleanup"
fi

# 8. Remove AWS IoT policies and certificates
echo "Removing AWS IoT policies and certificates..."
for cert_id in $(aws iot list-certificates --region ${region} --output json 2>/dev/null | jq -r '.certificates[].certificateId // empty'); do
    if [[ -z "$cert_id" ]]; then
        continue
    fi

    cert_arn=$(aws iot describe-certificate --region ${region} --certificate-id $cert_id --output json 2>/dev/null | jq -r '.certificateDescription.certificateArn // empty')
    if [[ -z "$cert_arn" ]]; then
        continue
    fi

    # List and remove maestro policies
    for policy_name in $(aws iot list-attached-policies --region ${region} --target $cert_arn --output json 2>/dev/null | jq -r '.policies[].policyName // empty'); do
        if [[ $policy_name == maestro* ]]; then
            echo "Detaching and deleting policy: $policy_name"
            aws iot detach-policy --region ${region} --target $cert_arn --policy-name $policy_name 2>/dev/null
            aws iot delete-policy --region ${region} --policy-name $policy_name 2>/dev/null && \
                echo "Policy $policy_name deleted" || \
                echo "Failed to delete policy $policy_name"

            echo "Revoking and deleting certificate: $cert_id"
            aws iot update-certificate --region ${region} --certificate-id $cert_id --new-status REVOKED 2>/dev/null
            sleep 5
            aws iot delete-certificate --region ${region} --certificate-id $cert_id 2>/dev/null && \
                echo "Certificate $cert_id deleted" || \
                echo "Failed to delete certificate $cert_id"
        fi
    done
done

# 9. Clean up local files (optional - uncomment if desired)
# output_dir=${ROSA_DIR}/_output
# if [ -d "$output_dir" ]; then
#     echo "Removing local output directory..."
#     rm -rf "$output_dir"
#     echo "Local output directory removed"
# fi

echo ""
echo "Teardown complete!"
echo "Note: Local files in ${ROSA_DIR}/_output are preserved. Remove manually if needed."
