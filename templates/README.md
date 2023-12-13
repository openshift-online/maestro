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

## Secrets template

`templates/secrets-template.yml`

This template deploys the `maestro` secret with all of the necessary secret key/value pairs.

## Agent template

`templates/agent-template.yml`

This template deploys the `maestro-agent` deployment and the related resources.
