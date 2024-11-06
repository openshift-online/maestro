#!/usr/bin/env bash

region=${REGION:-""}

if [ -z "$region" ]; then
    echo "cluster region is required"
    exit 1
fi

# Delete AWS PostgreSQL
db_status=$(aws rds delete-db-instance --region ${region} --db-instance-identifier maestro --skip-final-snapshot --delete-automated-backups | jq -r '.DBInstance.DBInstanceStatus')
echo "Deleting maestro db ($db_status)"

i=1
while [ $i -le 20 ]
do
    db_status=$(aws rds describe-db-instances --region ${region} --db-instance-identifier maestro | jq -r '.DBInstances[0].DBInstanceStatus')
    if [[ -z "$db_status" ]]; then
        echo "DB is deleted"
        break
    fi
    echo "[$i] DB status: ${db_status}"
    i=$((i + 1))
    sleep 30
done

aws rds delete-db-subnet-group --region ${region} --db-subnet-group-name maestrosubnetgroup
echo "DB db subnet group is removed"

# Remove AWS IoT polices and certificates
for cert_id in $(aws iot list-certificates --region ${region} | jq -r '.certificates[].certificateId'); do
    cert_arn=$(aws iot describe-certificate --region ${region} --certificate-id $cert_id | jq -r '.certificateDescription.certificateArn')
    # List all
    for policy_name in $(aws iot list-attached-policies --region ${region} --target $cert_arn | jq -r '.policies[].policyName'); do
        if [[ $policy_name == maestro* ]]; then
            echo "delelet policy $policy_name"
            aws iot detach-policy --region ${region} --target $cert_arn --policy-name $policy_name
            aws iot delete-policy --region ${region} --policy-name $policy_name

            echo "delelet certificate $cert_id"
            aws iot update-certificate --region ${region} --certificate-id $cert_id --new-status REVOKED
            sleep 5
            aws iot delete-certificate --region ${region} --certificate-id $cert_id
        fi
    done
done
