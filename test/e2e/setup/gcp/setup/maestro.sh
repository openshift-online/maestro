#!/usr/bin/env bash

######################
# Setup Maestro server
######################

PWD="$(cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1; pwd -P)"
GCP_DIR="$(cd "${PWD}/.." || exit 1; pwd -P)"
PROJECT_DIR="$(cd "${PWD}/../../../../.." || exit 1; pwd -P)"

output_dir="${GCP_DIR}/_output"
mkdir -p "${output_dir}"
echo "${output_dir}"

project_id=${PROJECT_ID:-""}
region=${REGION:-""}
cluster_id=${CLUSTER_ID:-""}

if [ -z "$project_id" ]; then
    echo "project id is required"
    exit 1
fi

if [ -z "$region" ]; then
    echo "region is required"
    exit 1
fi

if [ -z "$cluster_id" ]; then
    echo "GKE cluster id is required"
    exit 1
fi

echo "Setting up maestro server in ${region} (cluster=$cluster_id)"

# Get credential to access GKE cluster
gcloud container clusters get-credentials ${cluster_id} --region ${region} --project=${project_id} || {
    echo "Failed to get credentials for cluster ${cluster_id} in ${region}" >&2
    exit 1
  }

# db password
db_pw=$(LC_CTYPE=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 16)

# Create Cloud SQL instance in the same region
echo "Creating Cloud SQL instance if it doesn't exist..."
if ! gcloud sql instances describe maestro --project=${project_id} >/dev/null 2>&1; then
  gcloud sql instances create maestro \
    --project=${project_id} \
    --region=${region} \
    --edition=enterprise \
    --database-version=POSTGRES_17 \
    --tier=db-custom-2-8192 \
    --availability-type=ZONAL \
    --storage-size=10GB \
    --no-deletion-protection
  echo "Cloud SQL instance is created"
else
  echo "Cloud SQL instance already exists, skipping creation"
fi

# Create database
echo "Creating database if it doesn't exist..."
if ! gcloud sql databases describe maestro --instance=maestro >/dev/null 2>&1; then
  gcloud sql databases create maestro --instance=maestro
  echo "Cloud SQL database is created"
else
  echo "Cloud SQL database already exists, skipping creation"
fi

# Create DB user
echo "Creating database user for Cloud SQL instance if it doesn't exist..."
if ! gcloud sql users describe maestro --instance=maestro >/dev/null 2>&1; then
  gcloud sql users create maestro --instance=maestro --password=${db_pw}
  echo "Cloud SQL user/passwd is created"
else
  echo "Cloud SQL user already exists, updating password"
  gcloud sql users set-password maestro --instance=maestro --password=${db_pw}
fi

# Grant maestro service account access to connect to Cloud SQL
echo "Granting maestro service account access to Cloud SQL..."
gcloud projects add-iam-policy-binding ${project_id} \
  --member="serviceAccount:maestro@${project_id}.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client" || {
    echo "Failed to grant Cloud SQL access permission" >&2
    exit 1
  }
echo "Cloud SQL access permission is granted to maestro service account"

# Enable the IAM binding between GCP maestro service account and maestro k8s service account
echo "Binding maestro GSA and KSA for workload identity..."
gcloud iam service-accounts add-iam-policy-binding maestro@${project_id}.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:${project_id}.svc.id.goog[maestro/maestro]" || {
    echo "Failed to bind maestro GSA and KSA" >&2
    exit 1
  }
echo "Maestro GSA and KSA are bound"

echo "Deploying maestro server..."
oc create namespace maestro || true

oc -n maestro delete secret maestro-db --ignore-not-found
oc -n maestro create secret generic maestro-db \
    --from-literal=db.name=maestro \
    --from-literal=db.host="127.0.0.1" \
    --from-literal=db.port=5432 \
    --from-literal=db.user=maestro \
    --from-literal=db.password="${db_pw}"

# Create Helm values file for maestro-server
cat > ${output_dir}/maestro-server-values.yaml <<EOF

replicas: 3

environment: production

serviceAccount:
  name: maestro
  annotations:
    iam.gke.io/gcp-service-account: maestro@${project_id}.iam.gserviceaccount.com

database:
  secretName: maestro-db
  sslMode: disable
  maxOpenConnections: 50
  cloudSqlProxy:
    enabled: true
    instanceConnectionName: ${project_id}:${region}:maestro

messageBroker:
  type: pubsub
  secretName: maestro-pubsub
  pubsub:
    projectID: ${project_id}
    topics:
      sourceEvents: projects/${project_id}/topics/sourceevents
      sourceBroadcast: projects/${project_id}/topics/sourcebroadcast
    subscriptions:
      agentEvents: projects/${project_id}/subscriptions/agentevents-maestro
      agentBroadcast: projects/${project_id}/subscriptions/agentbroadcast-maestro

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
  enabled: false
EOF

# Deploy Maestro server using Helm
helm upgrade --install maestro-server \
    ${PROJECT_DIR}/charts/maestro-server \
    --namespace maestro \
    --create-namespace \
    --values ${output_dir}/maestro-server-values.yaml || {
  echo "Failed to deploy maestro server" >&2
  exit 1
}
oc -n maestro wait deploy/maestro --for condition=Available=True --timeout=300s || {
    echo "Maestro deployment did not become available within timeout" >&2
    exit 1
  }
