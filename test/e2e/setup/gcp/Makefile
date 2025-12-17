SHELL:=/bin/bash

e2e_dir=$(shell cd ${PWD}/../.. && pwd -P)

gcp/setup-pubsub:
	./setup/pubsub/setup_for_server.sh || { echo "Pub/Sub server setup failed" >&2; exit 1; }
	./setup/pubsub/setup_for_agent.sh || { echo "Pub/Sub agent setup failed" >&2; exit 1; }
.PHONY: gcp/setup-pubsub

gcp/setup-maestro:
	./setup/maestro.sh || { echo "Maestro server setup failed" >&2; exit 1; }
.PHONY: gcp/setup-maestro

gcp/setup-agent:
	./setup/agent.sh || { echo "Maestro agent setup failed" >&2; exit 1; }
.PHONY: gcp/setup-agent

gcp/setup-e2e:
	./setup/e2e.sh || { echo "Setup e2e failed"; exit 1; }
.PHONY: gcp/setup-e2e

gcp/e2e-test: gcp/setup-e2e
	@CONSUMER_ID=$$(cat ${PWD}/_output/consumer_id 2>/dev/null || echo "") && \
	if [ -z "$$CONSUMER_ID" ]; then echo "consumer_id file is missing or empty"; exit 1; fi && \
	ginkgo -v --fail-fast $(TEST_FOCUS) \
	--output-dir="$(e2e_dir)/report" --json-report=report.json --junit-report=report.xml \
	${e2e_dir}/pkg -- \
	-api-server="http://127.0.0.1:8000" \
	-grpc-server="127.0.0.1:8090" \
	-server-kubeconfig=$(KUBECONFIG) \
	-agent-kubeconfig=$(KUBECONFIG) \
	-consumer-name="$$CONSUMER_ID" \
	-dump-logs=false
.PHONY: gcp/e2e-test

gcp/teardown:
	./setup/teardown.sh || { echo "Teardown e2e failed"; exit 1; }
.PHONY: gcp/teardown
