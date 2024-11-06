SHELL:=/bin/bash

e2e_dir=$(shell cd ${PWD}/../.. && pwd -P)

rosa/setup-maestro:
	./setup/maestro.sh
.PHONY: rosa/setup-maestro

rosa/setup-agent:
	./setup/agent.sh
.PHONY: rosa/setup-agent

rosa/setup-e2e:
	./setup/e2e.sh
.PHONY: rosa/setup-e2e

rosa/e2e-test: rosa/setup-e2e
	ginkgo -v --fail-fast --label-filter="!(e2e-tests-spec-resync-reconnect||e2e-tests-status-resync-reconnect)" \
	--output-dir="$(e2e_dir)/report" --json-report=report.json --junit-report=report.xml \
	${e2e_dir}/pkg -- \
	-api-server="http://127.0.0.1:8000" \
	-grpc-server="127.0.0.1:8090" \
	-server-kubeconfig=$(KUBECONFIG) \
	-agent-kubeconfig=$(KUBECONFIG) \
	-consumer-name=${PWD}/_output/consumer_id
.PHONY: rosa/e2e-test

rosa/teardown:
	./setup/teardown.sh
.PHONY: rosa/teardown
