#!/usr/bin/env bash

project_id=${PROJECT_ID:-""}
consumer_id=${CONSUMER_ID:-""}

if [ -z "$project_id" ]; then
    echo "project id is required"
    exit 1
fi

if [ -z "$consumer_id" ]; then
    echo "consumer id is required"
    exit 1
fi

# Delete Cloud SQL instance
gcloud sql instances delete -q maestro --project="${project_id}" || true

# Delete topics and subscriptions
gcloud pubsub subscriptions delete projects/${project_id}/subscriptions/sourceevents-${consumer_id} || true
gcloud pubsub subscriptions delete projects/${project_id}/subscriptions/sourcebroadcast-${consumer_id} || true
gcloud pubsub subscriptions delete projects/${project_id}/subscriptions/agentevents-maestro || true
gcloud pubsub subscriptions delete projects/${project_id}/subscriptions/agentbroadcast-maestro || true
gcloud pubsub topics delete projects/${project_id}/topics/sourceevents || true
gcloud pubsub topics delete projects/${project_id}/topics/sourcebroadcast || true
gcloud pubsub topics delete projects/${project_id}/topics/agentevents || true
gcloud pubsub topics delete projects/${project_id}/topics/agentbroadcast || true

# Delete Service Account...
gcloud iam service-accounts delete -q maestro@${project_id}.iam.gserviceaccount.com || true
gcloud iam service-accounts delete -q maestro-agent-${consumer_id}@${project_id}.iam.gserviceaccount.com || true
