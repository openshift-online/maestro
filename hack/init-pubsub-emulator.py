#!/usr/bin/env python3
"""
Initialize Google Cloud Pub/Sub emulator with topics and subscriptions for Maestro.

This script creates the necessary topics and subscriptions for the Maestro server
to communicate with agents using Pub/Sub.

Environment Variables:
    PUBSUB_EMULATOR_HOST: The emulator host (default: localhost:8085)
    PUBSUB_PROJECT_ID: The GCP project ID (default: maestro-test)
"""

import os
import sys
from google.cloud import pubsub_v1
from google.api_core import exceptions


def init_server_topics_and_subscriptions(project_id: str):
    """Initialize topics and subscriptions for the Maestro server."""
    publisher = pubsub_v1.PublisherClient()
    subscriber = pubsub_v1.SubscriberClient()

    # Topics to create
    topics = ['sourceevents', 'sourcebroadcast', 'agentevents', 'agentbroadcast']

    print("Creating topics...")
    for topic_name in topics:
        topic_path = publisher.topic_path(project_id, topic_name)
        try:
            publisher.create_topic(request={"name": topic_path})
            print(f"  ✓ Created topic: {topic_name}")
        except exceptions.AlreadyExists:
            print(f"  - Topic already exists: {topic_name}")
        except Exception as e:
            print(f"  ✗ Error creating topic {topic_name}: {e}", file=sys.stderr)
            return False

    # Server subscriptions to create (name:topic:filter)
    subscriptions = [
        ('agentevents-maestro', 'agentevents', 'attributes.ce-originalsource="maestro"'),
        ('agentbroadcast-maestro', 'agentbroadcast', '')
    ]

    print("\nCreating server subscriptions...")
    for sub_name, topic_name, filter_expr in subscriptions:
        subscription_path = subscriber.subscription_path(project_id, sub_name)
        topic_path = publisher.topic_path(project_id, topic_name)
        try:
            if filter_expr:
                subscriber.create_subscription(
                    request={"name": subscription_path, "topic": topic_path, "filter": filter_expr}
                )
                print(f"  ✓ Created subscription: {sub_name} (filtered by {filter_expr})")
            else:
                subscriber.create_subscription(
                    request={"name": subscription_path, "topic": topic_path}
                )
                print(f"  ✓ Created subscription: {sub_name}")
        except exceptions.AlreadyExists:
            print(f"  - Subscription already exists: {sub_name}")
        except Exception as e:
            print(f"  ✗ Error creating subscription {sub_name}: {e}", file=sys.stderr)
            return False

    return True


def init_agent_subscriptions(project_id: str, consumer_name: str):
    """Initialize subscriptions for a Maestro agent."""
    publisher = pubsub_v1.PublisherClient()
    subscriber = pubsub_v1.SubscriberClient()

    # Agent subscriptions to create: (subscription_name, topic_name, filter)
    subscriptions = [
        (
            f'sourceevents-{consumer_name}',
            'sourceevents',
            f'attributes.ce-clustername="{consumer_name}"'
        ),
        (
            f'sourcebroadcast-{consumer_name}',
            'sourcebroadcast',
            ''  # No filter for broadcast
        )
    ]

    print(f"\nCreating agent subscriptions for consumer '{consumer_name}'...")
    for sub_name, topic_name, filter_expr in subscriptions:
        subscription_path = subscriber.subscription_path(project_id, sub_name)
        topic_path = publisher.topic_path(project_id, topic_name)

        try:
            if filter_expr:
                subscriber.create_subscription(
                    request={
                        "name": subscription_path,
                        "topic": topic_path,
                        "filter": filter_expr
                    }
                )
                print(f"  ✓ Created subscription: {sub_name} (filtered)")
            else:
                subscriber.create_subscription(
                    request={
                        "name": subscription_path,
                        "topic": topic_path
                    }
                )
                print(f"  ✓ Created subscription: {sub_name}")
        except exceptions.AlreadyExists:
            print(f"  - Subscription already exists: {sub_name}")
        except Exception as e:
            print(f"  ✗ Error creating subscription {sub_name}: {e}", file=sys.stderr)
            return False

    return True


def main():
    project_id = os.getenv('PUBSUB_PROJECT_ID', 'maestro-test')
    emulator_host = os.getenv('PUBSUB_EMULATOR_HOST', 'localhost:8085')
    consumer_name = os.getenv('CONSUMER_NAME', '')

    print(f"Initializing Pub/Sub emulator at {emulator_host}")
    print(f"Project ID: {project_id}")

    # Initialize server topics and subscriptions
    if not init_server_topics_and_subscriptions(project_id):
        print("\n✗ Failed to initialize server topics and subscriptions", file=sys.stderr)
        sys.exit(1)

    # Initialize agent subscriptions if consumer name is provided
    if consumer_name:
        if not init_agent_subscriptions(project_id, consumer_name):
            print(f"\n✗ Failed to initialize agent subscriptions for {consumer_name}", file=sys.stderr)
            sys.exit(1)

    print("\n✓ Pub/Sub emulator initialized successfully!")


if __name__ == '__main__':
    try:
        main()
    except Exception as e:
        print(f"✗ Unexpected error: {e}", file=sys.stderr)
        sys.exit(1)
