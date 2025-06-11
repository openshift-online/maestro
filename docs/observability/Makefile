DEPLOYMENT_NAME="maestro-agent"
CONTAINER_NAME="maestro-agent"
NAMESPACE="maestro"

all: deploy patch-maestro-server
	@echo "Observability services are now configured and enabled."
	@echo "Run the following command to port-forward traffic to the Jaeger service:"
	@echo ""
	@echo "kubectl port-forward -n observability svc/jaeger 16686:16686"
	@echo ""
	@echo "Then open 'http://localhost:16686' in your browser."

deploy:
	@kubectl apply -k jaeger/
	@kubectl wait --for=condition=Available deployment -n observability jaeger --timeout=60s
.PHONY: deploy

patch-maestro-server:
	@kubectl set env -n ${NAMESPACE} deployment ${DEPLOYMENT_NAME} --containers ${CONTAINER_NAME} OTEL_EXPORTER_OTLP_ENDPOINT=http://ingest.observability:4318 OTEL_TRACES_EXPORTER=otlp
	@kubectl wait --for=condition=Available -n ${NAMESPACE} deployment ${DEPLOYMENT_NAME} --timeout=30s
.PHONY: patch-maestro-server
