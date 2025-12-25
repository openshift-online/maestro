# Openshift Deployment Templates

The openshift deployment consists of 4 templates that, together, make an all-in-one deployment.

When deploying to production, the only template necessary is the service template.

## Service template

`templates/service-template.yml`

This is the main service template that deploys two objects, the `maestro` deployment and the related service.

## Route template

`templates/route-template.yml`

This template just deploys a route with the select `app:maestro` to map to the service deployed by the service template.

TLS is used by default for the route. No port is specified, all ports are allowed.

## Database template

`templates/db-template.yml`

This template deploys a simple postgresl-9.4 database deployment with a TLS-enabled service.

## MQTT template

`templates/mqtt-template.yml`

This template deploys a simple mosquitto-2.0.18 mqtt broker deployment.

## Pub/Sub Emulator template

`templates/pubsub-template.yml`

This template deploys a Google Cloud Pub/Sub emulator for local development and e2e testing. The emulator runs in a container and provides a compatible Pub/Sub API endpoint without requiring actual GCP credentials.

`templates/pubsub-init-job-template.yml`

This template creates a Kubernetes Job that initializes the Pub/Sub emulator with the required topics and subscriptions for the Maestro server. It uses the Python Pub/Sub client library to create:
- Topics: `sourceevents`, `sourcebroadcast`, `agentevents`, `agentbroadcast`
- Subscriptions:
  - `agentevents-maestro` - filtered by `ce-originalsource="maestro"`
  - `agentbroadcast-maestro` - receives all broadcast messages

`templates/pubsub-agent-init-job-template.yml`

This template creates a Kubernetes Job that initializes agent-specific Pub/Sub subscriptions. It must be run before deploying each agent and creates:
- Subscription: `sourceevents-{consumer_name}` - filtered by `ce-clustername` attribute
- Subscription: `sourcebroadcast-{consumer_name}` - receives all broadcast messages

For production GCP deployments, use the GCP-specific templates:
- `templates/service-template-gcp.yml` - Maestro server with Pub/Sub integration
- `templates/agent-template-gcp.yml` - Maestro agent with Pub/Sub integration

## Agent template

`templates/agent-template.yml`

This template deploys the `maestro-agent` deployment and the related resources.
