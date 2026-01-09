#!/usr/bin/env bash
set -e

# Initialize Google Cloud Pub/Sub emulator with topics and subscriptions for Maestro.
#
# This script creates the necessary topics and subscriptions for the Maestro server
# to communicate with agents using Pub/Sub.
#
# Environment Variables:
#     PUBSUB_EMULATOR_HOST: The emulator host (default: localhost:8085)
#     PUBSUB_PROJECT_ID: The GCP project ID (default: maestro-test)
#     CONSUMER_NAME: Optional consumer name for agent subscriptions

PROJECT_ID="${PUBSUB_PROJECT_ID:-maestro-test}"
EMULATOR_HOST="${PUBSUB_EMULATOR_HOST:-localhost:8085}"
CONSUMER_NAME="${CONSUMER_NAME:-}"

# Base URL for Pub/Sub emulator API
BASE_URL="http://${EMULATOR_HOST}/v1"

echo "Initializing Pub/Sub emulator at ${EMULATOR_HOST}"
echo "Project ID: ${PROJECT_ID}"

# Wait for emulator to be ready
echo "Verifying emulator connectivity..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
    if curl -s --connect-timeout 2 --max-time 5 "http://${EMULATOR_HOST}" > /dev/null 2>&1; then
        echo "Emulator responded! Waiting for API to be fully ready..."
        sleep 2
        echo "Emulator is ready!"
        break
    fi
    attempt=$((attempt + 1))
    if [ $attempt -eq $max_attempts ]; then
        echo "Error: Could not connect to Pub/Sub emulator at http://${EMULATOR_HOST}" >&2
        echo "Please ensure the emulator is running and accessible." >&2
        exit 1
    fi
    sleep 1
done

# Function to create a topic
create_topic() {
    local topic_name=$1
    local topic_path="projects/${PROJECT_ID}/topics/${topic_name}"

    response=$(curl -s --connect-timeout 5 --max-time 10 -w "\n%{http_code}" -X PUT "${BASE_URL}/${topic_path}" 2>&1)
    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo "  ✓ Created topic: ${topic_name}"
        return 0
    elif [ "$http_code" -eq 409 ]; then
        echo "  - Topic already exists: ${topic_name}"
        return 0
    else
        echo "  ✗ Error creating topic ${topic_name}: HTTP ${http_code}" >&2
        echo "  URL: ${BASE_URL}/${topic_path}" >&2
        echo "$response" | head -n-1 >&2
        return 1
    fi
}

# Function to create a subscription
create_subscription() {
    local sub_name=$1
    local topic_name=$2
    local filter_expr=$3

    local sub_path="projects/${PROJECT_ID}/subscriptions/${sub_name}"
    local topic_path="projects/${PROJECT_ID}/topics/${topic_name}"

    # Build JSON payload using jq for proper JSON escaping
    local payload
    if [ -n "$filter_expr" ]; then
        payload=$(jq -n \
            --arg topic "${topic_path}" \
            --arg filter "${filter_expr}" \
            '{topic: $topic, filter: $filter}')
    else
        payload=$(jq -n \
            --arg topic "${topic_path}" \
            '{topic: $topic}')
    fi

    response=$(curl -s --connect-timeout 5 --max-time 10 -w "\n%{http_code}" -X PUT \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "${BASE_URL}/${sub_path}" 2>&1)
    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        if [ -n "$filter_expr" ]; then
            echo "  ✓ Created subscription: ${sub_name} (filtered by ${filter_expr})"
        else
            echo "  ✓ Created subscription: ${sub_name}"
        fi
        return 0
    elif [ "$http_code" -eq 409 ]; then
        echo "  - Subscription already exists: ${sub_name}"
        return 0
    else
        echo "  ✗ Error creating subscription ${sub_name}: HTTP ${http_code}" >&2
        echo "  URL: ${BASE_URL}/${sub_path}" >&2
        echo "$response" | head -n-1 >&2
        return 1
    fi
}

# Create topics
echo "Creating topics..."
topics=(sourceevents sourcebroadcast agentevents agentbroadcast)
for topic in "${topics[@]}"; do
    if ! create_topic "$topic"; then
        echo "" >&2
        echo "✗ Failed to initialize server topics" >&2
        exit 1
    fi
done

# Create server subscriptions
echo ""
echo "Creating server subscriptions..."
if ! create_subscription "agentevents-maestro" "agentevents" 'attributes."ce-originalsource"="maestro"'; then
    echo "" >&2
    echo "✗ Failed to initialize server subscriptions" >&2
    exit 1
fi

if ! create_subscription "agentbroadcast-maestro" "agentbroadcast" ""; then
    echo "" >&2
    echo "✗ Failed to initialize server subscriptions" >&2
    exit 1
fi

# Create agent subscriptions if consumer name is provided
if [ -n "$CONSUMER_NAME" ]; then
    echo ""
    echo "Creating agent subscriptions for consumer '${CONSUMER_NAME}'..."

    if ! create_subscription "sourceevents-${CONSUMER_NAME}" "sourceevents" "attributes.\"ce-clustername\"=\"${CONSUMER_NAME}\""; then
        echo "" >&2
        echo "✗ Failed to initialize agent subscriptions for ${CONSUMER_NAME}" >&2
        exit 1
    fi

    if ! create_subscription "sourcebroadcast-${CONSUMER_NAME}" "sourcebroadcast" ""; then
        echo "" >&2
        echo "✗ Failed to initialize agent subscriptions for ${CONSUMER_NAME}" >&2
        exit 1
    fi
fi

echo ""
echo "✓ Pub/Sub emulator initialized successfully!"
