#!/usr/bin/env bash

#####################
# Setup Maestro agent
#####################

PWD="$(cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1; pwd -P)"
GCP_DIR="$(cd "${PWD}/.." || exit 1; pwd -P)"
PROJECT_DIR="$(cd "${PWD}/../../../../.." || exit 1; pwd -P)"

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

echo "Setting up maestro agent in ${region} (cluster=${cluster_id}, consumer_id=${consumer_id})"

# Get credential to access GKE cluster
gcloud container clusters get-credentials "${cluster_id}" --region "${region}" --project="${project_id}"

# IMAGE_REGISTRY=${IMAGE_REGISTRY:-"quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro"}
# IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-"maestro"}
# IMAGE_TAG=${IMAGE_TAG:-"1de63c6075f2c95c9661d790d164019f60d789f3"}
IMAGE_REGISTRY=${IMAGE_REGISTRY:-"quay.io/morvencao"}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-"maestro"}
IMAGE_TAG=${IMAGE_TAG:-"dev"}

# Enable the IAM binding between GCP maestro-agent service account and maestro-agent k8s service account
echo "Binding maestro agent GSA and KSA for workload identity..."
gcloud iam service-accounts add-iam-policy-binding maestro-agent-${consumer_id}@${project_id}.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:${project_id}.svc.id.goog[maestro-agent/maestro-agent-sa]" || { echo "Workload identity binding failed"; exit 1; }
echo "Maestro agent GSA and KSA are bind"

echo "Deploying maestro agent..."
oc create namespace maestro-agent || true

oc process --filename="${PROJECT_DIR}/templates/agent-template-gcp.yml" \
    --local="true" \
    --param="PROJECT_ID=${project_id}" \
    --param="AGENT_NAMESPACE=maestro-agent" \
    --param="CONSUMER_NAME=${consumer_id}" \
    --param="IMAGE_REGISTRY=${IMAGE_REGISTRY}" \
    --param="IMAGE_REPOSITORY=${IMAGE_REPOSITORY}" \
    --param="IMAGE_TAG=${IMAGE_TAG}" > ${output_dir}/maestro-${consumer_id}-gcp.json

oc apply -f "${output_dir}/maestro-${consumer_id}-gcp.json" || { echo "Manifest application failed"; exit 1; }
oc -n maestro-agent wait deploy/maestro-agent --for condition=Available=True --timeout=300s || { echo "Deployment failed to become ready"; exit 1; }
