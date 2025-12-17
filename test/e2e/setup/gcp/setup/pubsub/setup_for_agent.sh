#!/usr/bin/env bash

################################
# Setup PubSub for Maestro Agent
################################

set -euo pipefail

# ==== CONFIGURATION ====
project_id="${PROJECT_ID:-}"
cluster_name=${CONSUMER_ID:-""}

if [ -z "$project_id" ]; then
    echo "project id is required"
    exit 1
fi

if [ -z "$cluster_name" ]; then
    echo "consumer id is required"
    exit 1
fi

sa_name="maestro-agent-${cluster_name}"
sa_id="${sa_name}@${project_id}.iam.gserviceaccount.com"

# Agent Topics (Maestro subscribes to these)
AGENT_TOPICS=(
  "agentevents"
  "agentbroadcast"
)

# Source Subscriptions to create: name:topic:filter(optional): (Maestro Agent listens on these)
SOURCE_SUBSCRIPTIONS=(
  "sourceevents-${cluster_name}:sourceevents:attributes.\"ce-clustername\"=\"${cluster_name}\""
  "sourcebroadcast-${cluster_name}:sourcebroadcast:"
)

# ==== EXECUTION ====
echo "Registering new cluster: ${cluster_name}"

echo "Setting project to ${project_id}"
gcloud config set project "${project_id}" >/dev/null

echo "Creating service account ${sa_name}..."
gcloud iam service-accounts create "${sa_name}" \
  --display-name="Pub/Sub Maestro Agent Service Account for ${cluster_name}" || true

for ENTRY in "${SOURCE_SUBSCRIPTIONS[@]}"; do
  IFS=':' read -r SUB_NAME TOPIC FILTER <<< "$ENTRY"
  echo "Creating subscription ${SUB_NAME} for topic ${TOPIC}..."
  if [[ -n "${FILTER}" ]]; then
    gcloud pubsub subscriptions create "${SUB_NAME}" \
        --topic="${TOPIC}" \
        --message-filter="${FILTER}" \
        --project="${project_id}" \
        --ack-deadline=60 || true
  else
    gcloud pubsub subscriptions create "${SUB_NAME}" \
      --topic="${TOPIC}" \
      --project="${project_id}" \
      --ack-deadline=60 || true
  fi

  echo "Granting subscriber role to ${sa_id} on ${SUB_NAME}..."
  gcloud pubsub subscriptions add-iam-policy-binding "${SUB_NAME}" \
    --member="serviceAccount:${sa_id}" \
    --role="roles/pubsub.subscriber" \
    --project="${project_id}"
done

# Maestro Agent can publish to Agent Topics
for TOPIC in "${AGENT_TOPICS[@]}"; do
  echo " - Granting publisher on topic ${TOPIC}"
  gcloud pubsub topics add-iam-policy-binding "${TOPIC}" \
    --member="serviceAccount:${sa_id}" \
    --role="roles/pubsub.publisher" \
    --project="${project_id}"
done

echo "✅ PubSub topics and subscriptions for cluster '${cluster_name}' completes!"
echo "Maestro agent service account: ${sa_id}"
