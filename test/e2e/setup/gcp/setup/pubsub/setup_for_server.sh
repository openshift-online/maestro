#!/usr/bin/env bash

#################################
# Setup PubSub for Maestro Server
#################################

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

sa_name="maestro"
sa_id="${sa_name}@${project_id}.iam.gserviceaccount.com"

# Source Topics (Maestro publishes to these)
SOURCE_TOPICS=(
  "sourceevents"
  "sourcebroadcast"
)

# Agent Topics (Maestro subscribes to these)
AGENT_TOPICS=(
  "agentevents"
  "agentbroadcast"
)

# Agent Subscriptions (Maestro listens on these)
AGENT_SUBSCRIPTIONS=(
  "agentevents-maestro:agentevents:attributes.\"ce-originalsource\"=\"maestro\""
  "agentbroadcast-maestro:agentbroadcast:"
)

# ==== EXECUTION ====
echo "Setting project to ${project_id}"
gcloud config set project "${project_id}" >/dev/null

echo "Creating service account ${sa_name}..."
gcloud iam service-accounts create "${sa_name}" \
  --display-name="Pub/Sub Maestro Service Account" || true

echo "Creating Source Topics..."
for TOPIC in "${SOURCE_TOPICS[@]}"; do
  echo " - ${TOPIC}"
  gcloud pubsub topics create "${TOPIC}" --project="${project_id}" || true
done

echo "Creating Agent Topics..."
for TOPIC in "${AGENT_TOPICS[@]}"; do
  echo " - ${TOPIC}"
  gcloud pubsub topics create "${TOPIC}" --project="${project_id}" || true
done

echo "Creating Maestro Server Subscriptions..."
for ENTRY in "${AGENT_SUBSCRIPTIONS[@]}"; do
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
done

echo "Granting Pub/Sub permissions to Maestro service account..."

# Maestro can publish to Source Topics
for TOPIC in "${SOURCE_TOPICS[@]}"; do
  echo " - Granting publisher on topic ${TOPIC}"
  gcloud pubsub topics add-iam-policy-binding "${TOPIC}" \
    --member="serviceAccount:${sa_id}" \
    --role="roles/pubsub.publisher" \
    --project="${project_id}"
done

# Maestro can subscribe to Agent Subscriptions
for ENTRY in "${AGENT_SUBSCRIPTIONS[@]}"; do
  IFS=':' read -r SUB_NAME _ _ <<< "$ENTRY"
  echo " - Granting subscriber on subscription ${SUB_NAME}"
  gcloud pubsub subscriptions add-iam-policy-binding "${SUB_NAME}" \
    --member="serviceAccount:${sa_id}" \
    --role="roles/pubsub.subscriber" \
    --project="${project_id}"
done

echo "✅ Pub/Sub topics and subscriptions setup for maestro server complete!"
echo "Maestro server service account: ${sa_id}"
