.DEFAULT_GOAL := help

# CGO_ENABLED=0 is not FIPS compliant. large commercial vendors and FedRAMP require FIPS compliant crypto
CGO_ENABLED := 1

# Enable users to override the golang used to accomodate custom installations
GO ?= go

# Set GOPATH from go env if not already set
GOPATH ?= $(shell $(GO) env GOPATH)
export GOPATH

# Allow overriding `oc` command.
# Used by pr_check.py to ssh deploy inside private Hive cluster via bastion host.
oc:=oc

# The version needs to be different for each deployment because otherwise the
# cluster will not pull the new image from the internal registry:
version:=$(shell date +%s)
# Tag for the image:
image_tag ?= $(version)

# The namespace where maestro server and agent will be deployed.
namespace ?= maestro
agent_namespace ?= maestro-agent

# a tool for managing containers and images, etc. You can set it as docker
container_tool ?= podman

# In the development environment we are pushing the image directly to the image
# registry inside the development cluster. That registry has a different name
# when it is accessed from outside the cluster and when it is accessed from
# inside the cluster. We need the external name to push the image, and the
# internal name to pull it.
external_apps_domain ?= apps-crc.testing
external_image_registry ?= default-route-openshift-image-registry.$(external_apps_domain)
internal_image_registry ?= image-registry.openshift-image-registry.svc:5000

# The name of the image repository needs to start with the name of an existing
# namespace because when the image is pushed to the internal registry of a
# cluster it will assume that that namespace exists and will try to create a
# corresponding image stream inside that namespace. If the namespace doesn't
# exist the push fails. This doesn't apply when the image is pushed to a public
# repository, like `docker.io` or `quay.io`.
image_repository ?= $(namespace)/maestro

# Database connection details
db_name:=maestro
db_host=maestro-db.$(namespace)
db_port=5432
db_user:=maestro
db_password:=foobar-bizz-buzz
db_password_file=${PWD}/secrets/db.password
db_image?=quay.io/maestro/postgres:17.2

# Message broker connection details
mqtt_host ?= maestro-mqtt.$(namespace)
mqtt_port ?= 1883
mqtt_user ?= maestro
mqtt_password_file ?= ${PWD}/secrets/mqtt.password
mqtt_config_file ?= ${PWD}/secrets/mqtt.config
mqtt_image ?= quay.io/maestro/eclipse-mosquitto:2.0.18

# Pub/Sub emulator configuration
pubsub_host ?= maestro-pubsub.$(namespace)
pubsub_port ?= 8085
pubsub_project_id ?= maestro-test
pubsub_config_file ?= ${PWD}/secrets/pubsub.config
pubsub_image ?= gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators

# Log verbosity level
klog_v:=2

# message driver type, mqtt, grpc or pubsub, default is mqtt.
MESSAGE_DRIVER_TYPE ?= mqtt

# Test output files
unit_test_json_output ?= ${PWD}/unit-test-results.json
mqtt_integration_test_json_output ?= ${PWD}/mqtt-integration-test-results.json
pubsub_integration_test_json_output ?= ${PWD}/pubsub-integration-test-results.json
grpc_integration_test_json_output ?= ${PWD}/grpc-integration-test-results.json

# Prints a list of useful targets.
help:
	@echo ""
	@echo "Maestro Service"
	@echo ""
	@echo "make verify               verify source code"
	@echo "make lint                 run golangci-lint"
	@echo "make binary               compile binaries"
	@echo "make install              compile binaries and install in GOPATH bin"
	@echo "make db/setup             setup local PostgreSQL database"
	@echo "make db/teardown          teardown local PostgreSQL database"
	@echo "make mqtt/setup           setup local MQTT broker"
	@echo "make mqtt/teardown        teardown local MQTT broker"
	@echo "make pubsub/setup         setup local Pub/Sub emulator"
	@echo "make pubsub/init          initialize Pub/Sub topics and subscriptions"
	@echo "make pubsub/teardown      teardown local Pub/Sub emulator"
	@echo "make run                  run the application"
	@echo "make run/docs             run swagger and host the api spec"
	@echo "make test                 run unit tests"
	@echo "make test-integration     run integration tests"
	@echo "make generate             generate openapi modules"
	@echo "make image                build docker image"
	@echo "make push                 push docker image"
	@echo "make deploy               deploy maestro server via Helm"
	@echo "make deploy-agent         deploy maestro agent via Helm (requires consumer_name)"
	@echo "make undeploy             undeploy maestro server"
	@echo "make undeploy-agent       undeploy maestro agent"
	@echo "make lint-charts          lint Helm charts"
	@echo "make clean                delete temporary generated files"
	@echo "$(fake)"
.PHONY: help

# Encourage consistent tool versions
OPENAPI_GENERATOR_VERSION:=5.4.0
GO_VERSION:=go1.25.

### Constants:
version:=$(shell date +%s)
GOLANGCI_LINT_BIN:=$(shell go env GOPATH)/bin/golangci-lint

### Envrionment-sourced variables with defaults
# Can be overriden by setting environment var before running
# Example:
#   MAESTRO_ENV=testing make run
#   export MAESTRO_ENV=testing; make run
# Set the environment to development by default
ifndef MAESTRO_ENV
	MAESTRO_ENV:=development
endif

ifndef TEST_SUMMARY_FORMAT
	TEST_SUMMARY_FORMAT=short-verbose
endif

# Ensures GOPATH is set (now auto-configured at top of Makefile)
check-gopath:
	@echo "GOPATH is set to: $(GOPATH)"
.PHONY: check-gopath

install-golang-gci:
	go install github.com/daixiang0/gci@v0.13.7

fmt-imports: install-golang-gci
	gci write --skip-generated -s standard -s default -s "prefix(github.com/openshift-online/maestro)" -s localmodule cmd pkg test

verify-fmt-imports: install-golang-gci
	@output=$$(gci diff --skip-generated -s standard -s default -s "prefix(github.com/openshift-online/maestro)" -s localmodule cmd pkg test); \
	if [ -n "$$output" ]; then \
	    echo "Go import diff output is not empty: $$output"; \
	    echo "Please run 'make fmt-imports' to format the golang files imports automatically."; \
	    exit 1; \
	else \
	    echo "Go import diff output is empty"; \
	fi

# Verifies that source passes standard checks.
verify: check-gopath verify-fmt-imports
	${GO} vet \
		./cmd/... \
		./pkg/...
	! gofmt -l cmd pkg test |\
		sed 's/^/Unformatted file: /' |\
		grep .
	@ ${GO} version | grep -q "$(GO_VERSION)" || \
		( \
			printf '\033[41m\033[97m\n'; \
			echo "* Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			echo "* Your go version is not the expected $(GO_VERSION) *"; \
			echo "* Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			printf '\033[0m'; \
		)
.PHONY: verify

# Runs our linter to verify that everything is following best practices
# Requires golangci-lint to be installed @ $(go env GOPATH)/bin/golangci-lint
# Linter is set to ignore `unused` stuff due to example being incomplete by definition
lint:
	$(GOLANGCI_LINT_BIN) run -e unused \
		./cmd/... \
		./pkg/...
.PHONY: lint

# Build binaries
# NOTE it may be necessary to use CGO_ENABLED=0 for backwards compatibility with centos7 if not using centos7
binary: check-gopath
	${GO} mod vendor
	${GO} build $(BUILD_OPTS) ./cmd/maestro
.PHONY: binary

maestro-cli:
	${GO} build $(BUILD_OPTS) -o maestro-cli ./examples/manifestwork/client.go
.PHONY: maestro-cli

# Install
install: check-gopath
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=boringcrypto ${GO} install -ldflags="$(ldflags)" ./cmd/maestro
	@ ${GO} version | grep -q "$(GO_VERSION)" || \
		( \
			printf '\033[41m\033[97m\n'; \
			echo "* Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			echo "* Your go version is not the expected $(GO_VERSION) *"; \
			echo "* Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			printf '\033[0m'; \
		)
.PHONY: install

# Runs the unit tests.
#
# Args:
#   TESTFLAGS: Flags to pass to `go test`. The `-v` argument is always passed.
#
# Examples:
#   make test TESTFLAGS="-run TestSomething"
test:
	MAESTRO_ENV=testing gotestsum --jsonfile-timing-events=$(unit_test_json_output) --format $(TEST_SUMMARY_FORMAT) -- -p 1 -v $(TESTFLAGS) \
		./pkg/... \
		./cmd/...
.PHONY: test

# Runs the integration tests.
#
# Args:
#   TESTFLAGS: Flags to pass to `go test`. The `-v` argument is always passed.
#
# Example:
#   make test-integration
#   make test-integration TESTFLAGS="-run TestAccounts"     acts as TestAccounts* and run TestAccountsGet, TestAccountsPost, etc.
#   make test-integration TESTFLAGS="-run TestAccountsGet"  runs TestAccountsGet
#   make test-integration TESTFLAGS="-short"                skips long-run tests
test-integration: test-integration-mqtt test-integration-pubsub test-integration-grpc
.PHONY: test-integration

test-integration-mqtt:
	MESSAGE_DRIVER_TYPE=mqtt MESSAGE_DRIVER_CONFIG=$(PWD)/secrets/mqtt.config MAESTRO_ENV=testing gotestsum --jsonfile-timing-events=$(mqtt_integration_test_json_output) --format $(TEST_SUMMARY_FORMAT) -- -count=1 -p 1 -ldflags -s -v -timeout 1h $(TESTFLAGS) \
			./test/integration
.PHONY: test-integration-mqtt

test-integration-pubsub:
	MESSAGE_DRIVER_TYPE=pubsub MESSAGE_DRIVER_CONFIG=$(PWD)/secrets/pubsub.config PUBSUB_EMULATOR_HOST=localhost:$(pubsub_port) MAESTRO_ENV=testing gotestsum --jsonfile-timing-events=$(pubsub_integration_test_json_output) --format $(TEST_SUMMARY_FORMAT) -- -count=1 -p 1 -ldflags -s -v -timeout 1h $(TESTFLAGS) \
			./test/integration
.PHONY: test-integration-pubsub

test-integration-grpc:
	MESSAGE_DRIVER_TYPE=grpc MAESTRO_ENV=testing gotestsum --jsonfile-timing-events=$(grpc_integration_test_json_output) --format $(TEST_SUMMARY_FORMAT) -- -count=1 -p 1 -ldflags -s -v -timeout 1h $(TESTFLAGS) \
			./test/integration
.PHONY: test-integration-grpc

# Regenerate openapi client and models
generate:
	rm -rf pkg/api/openapi
	$(container_tool) build -t ams-openapi -f Dockerfile.openapi .
	$(eval OPENAPI_IMAGE_ID=`$(container_tool) create -t ams-openapi -f Dockerfile.openapi .`)
	$(container_tool) cp $(OPENAPI_IMAGE_ID):/local/pkg/api/openapi ./pkg/api/openapi
	$(container_tool) cp $(OPENAPI_IMAGE_ID):/local/data/generated/openapi/openapi.go ./data/generated/openapi/openapi.go
.PHONY: generate

run: install
	maestro migration
	@if [ "$(MESSAGE_DRIVER_TYPE)" = "grpc" ]; then \
		maestro server --message-broker-type=$(MESSAGE_DRIVER_TYPE); \
	else \
		maestro server --message-broker-type=$(MESSAGE_DRIVER_TYPE) --message-broker-config-file=./secrets/$(MESSAGE_DRIVER_TYPE).config; \
	fi
.PHONY: run

# Run Swagger and host the api docs
run/docs:
	@echo "Please open http://localhost/"
	docker run -d -p 80:8080 -e SWAGGER_JSON=/maestro.yaml -v $(PWD)/openapi/maestro.yaml:/maestro.yaml swaggerapi/swagger-ui
.PHONY: run/docs

# Delete temporary files
clean:
	rm -rf \
		$(binary) \
		data/generated/openapi/*.json \
.PHONY: clean

.PHONY: cmds
cmds:
	for cmd in $$(ls cmd); do \
		${GO} build \
			-ldflags="$(ldflags)" \
			-o "$${cmd}" \
			"./cmd/$${cmd}" \
			|| exit 1; \
	done

.PHONY: image
image: cmds
	$(container_tool) build -t "$(external_image_registry)/$(image_repository):$(image_tag)" .

.PHONY: e2e-image
e2e-image:
	$(container_tool) build -f Dockerfile.e2e -t "$(external_image_registry)/$(image_repository)-e2e:$(image_tag)" .

.PHONY: push
push: image
	$(container_tool) push "$(external_image_registry)/$(image_repository):$(image_tag)"

# Deploy Maestro server using Helm charts
.PHONY: deploy
deploy:
	helm upgrade --install maestro-server \
		./charts/maestro-server \
		--namespace $(namespace) \
		--create-namespace \
		--set mqtt.enabled=true \
		--set route.enabled=true \
		--set postgresql.enabled=true

# Undeploy Maestro server using Helm charts
.PHONY: undeploy
undeploy:
	helm uninstall maestro-server --namespace $(namespace) || true

# Deploy Maestro agent using Helm charts
# Optional: Set install_work_crds=true to install CRDs (default: false to skip if already exists)
.PHONY: deploy-agent
deploy-agent:
	@if [ -z "$(consumer_name)" ]; then \
		echo "Error: consumer_name must be set"; \
		exit 1; \
	fi
	helm upgrade --install maestro-agent \
		./charts/maestro-agent \
		--namespace $(agent_namespace) \
		--create-namespace \
		--set consumerName=$(consumer_name) \
		--set installWorkCRDs=$(if $(install_work_crds),$(install_work_crds),false)

# Undeploy Maestro agent using Helm charts
.PHONY: undeploy-agent
undeploy-agent:
	helm uninstall maestro-agent --namespace $(agent_namespace) || true

.PHONY: db/setup
db/setup:
	@echo $(db_password) > $(db_password_file)
	$(container_tool) run --name psql-maestro -e POSTGRES_DB=$(db_name) -e POSTGRES_USER=$(db_user) -e POSTGRES_PASSWORD=$(db_password) -p $(db_port):5432 -d $(db_image)

.PHONY: db/login
db/login:
	$(container_tool) exec -it psql-maestro bash -c "psql -h localhost -U $(db_user) $(db_name)"

.PHONY: db/teardown
db/teardown:
	$(container_tool) stop psql-maestro
	$(container_tool) rm psql-maestro

.PHONY: mqtt/prepare
mqtt/prepare:
	@openssl rand -base64 13 | tr -dc 'a-zA-Z0-9' | head -c 13 > $(mqtt_password_file)

.PHONY: mqtt/setup
mqtt/setup: mqtt/prepare
	@echo '{"brokerHost":"localhost:1883","username":"$(mqtt_user)","password":"$(shell cat $(mqtt_password_file))","topics":{"sourceEvents":"sources/maestro/consumers/+/sourceevents","agentEvents":"sources/maestro/consumers/+/agentevents"}}' > $(mqtt_config_file)
	$(container_tool) run --rm -v $(shell pwd)/hack:/mosquitto/data:z $(mqtt_image) mosquitto_passwd -c -b /mosquitto/data/mosquitto-passwd.txt $(mqtt_user) $(shell cat $(mqtt_password_file))
	$(container_tool) run --name mqtt-maestro -p 1883:1883 -v $(shell pwd)/hack/mosquitto-passwd.txt:/mosquitto/config/password.txt -v $(shell pwd)/hack/mosquitto.conf:/mosquitto/config/mosquitto.conf -d $(mqtt_image)

.PHONY: mqtt/teardown
mqtt/teardown:
	$(container_tool) stop mqtt-maestro
	$(container_tool) rm mqtt-maestro

.PHONY: pubsub/setup
pubsub/setup:
	@mkdir -p ${PWD}/secrets
	@echo '{"projectID":"$(pubsub_project_id)","endpoint":"localhost:$(pubsub_port)","insecure":true,"topics":{"sourceEvents":"projects/$(pubsub_project_id)/topics/sourceevents","sourceBroadcast":"projects/$(pubsub_project_id)/topics/sourcebroadcast"},"subscriptions":{"agentEvents":"projects/$(pubsub_project_id)/subscriptions/agentevents-maestro","agentBroadcast":"projects/$(pubsub_project_id)/subscriptions/agentbroadcast-maestro"}}' > $(pubsub_config_file)
	$(container_tool) run --name pubsub-maestro -p $(pubsub_port):8085 -e PUBSUB_PROJECT_ID=$(pubsub_project_id) -d $(pubsub_image) gcloud beta emulators pubsub start --host-port=0.0.0.0:8085 --project=$(pubsub_project_id)

.PHONY: pubsub/teardown
pubsub/teardown:
	$(container_tool) stop pubsub-maestro
	$(container_tool) rm pubsub-maestro

.PHONY: pubsub/init
pubsub/init:
	@PUBSUB_EMULATOR_HOST=localhost:$(pubsub_port) PUBSUB_PROJECT_ID=$(pubsub_project_id) bash hack/init-pubsub-emulator.sh

crc/login:
	@echo "Logging into CRC"
	@crc console --credentials -ojson | jq -r .clusterConfig.adminCredentials.password | oc login --username kubeadmin --insecure-skip-tls-verify=true https://api.crc.testing:6443
	@oc whoami --show-token | $(container_tool) login --username kubeadmin --password-stdin "$(external_image_registry)"
.PHONY: crc/login

# Set up the test environment infrastructure
# Creates necessary namespaces, secrets, and prerequisites (database, MQTT/gRPC broker) for running e2e/upgrade tests
test-env/setup:
	./test/setup/env_setup.sh
.PHONY: test-env/setup

# Deploy the Maestro server component to the test environment using Helm
test-env/deploy-server:
	./test/setup/deploy_server.sh
.PHONY: test-env/deploy-server

# Deploy the Maestro agent component to the test environment using Helm
# Configures agent to connect to the deployed server
test-env/deploy-agent:
	./test/setup/deploy_agent.sh
.PHONY: test-env/deploy-agent

# Clean up the test environment
# Removes all deployed resources, namespaces, and test artifacts
test-env/cleanup:
	./test/setup/env_cleanup.sh
.PHONY: test-env/cleanup

# Prepare the test environment using Helm charts
test-env: test-env/cleanup test-env/setup test-env/deploy-server test-env/deploy-agent
.PHONY: test-env

# Runs the e2e tests.
#
# Args:
#   TEST_FOCUS: Flags to pass to `ginkgo run`.
# The `-v` argument is always passed.
#
# Example:
#   make e2e-test/run
#   make e2e-test/run TEST_FOCUS="--focus=Resources" run only the Resources tests
e2e-test/run:
	ginkgo -v --fail-fast --label-filter='$(LABEL_FILTER)' $(TEST_FOCUS) \
	--output-dir="${PWD}/test/e2e/report" --json-report=report.json --junit-report=report.xml \
	${PWD}/test/e2e/pkg -- \
	-api-server=$(shell cat ${PWD}/test/_output/.external_restapi_endpoint) \
	-grpc-server=$(shell cat ${PWD}/test/_output/.external_grpc_endpoint) \
	-server-kubeconfig=${PWD}/test/_output/.kubeconfig \
	-agent-kubeconfig=${PWD}/test/_output/.kubeconfig \
	-consumer-name=$(shell cat ${PWD}/test/_output/.consumer_name)
.PHONY: e2e-test/run

# Runs the e2e tests in local.
# Args:
#   ENABLE_MAESTRO_TLS: ENABLE Maestro services TLS, default is false
#
# Example:
#   make e2e-test
#   ENABLE_MAESTRO_TLS=true make e2e-test
# NOTE: Uses Helm charts for deployment
e2e-test: test-env e2e-test/run
.PHONY: e2e-test

e2e-test/istio: test-env
	./test/e2e/istio/test.sh
.PHONY: e2e-test/istio

e2e/rollout:
ifndef KUBECONFIG
	$(error "Must set KUBECONFIG")
endif
	KUBECONFIG=$(KUBECONFIG) ./test/e2e/setup/roll_out.sh
.PHONY: e2e/rollout

upgrade-test: test-env/cleanup test-env/setup
	./test/upgrade/test.sh
.PHONY: upgrade-test

# ==============================================================================
# Helm Chart Utility Targets
# ==============================================================================

# Lint all Helm charts
lint-charts:
	helm lint charts/maestro-server
	helm lint charts/maestro-agent
.PHONY: lint-charts

# Package all Helm charts
package-charts:
	helm package charts/maestro-server -d charts/
	helm package charts/maestro-agent -d charts/
.PHONY: package-charts

# Render maestro-server chart templates (dry-run)
template-server:
	helm template maestro-server ./charts/maestro-server --namespace $(namespace)
.PHONY: template-server

# Render maestro-agent chart templates (dry-run)
template-agent:
	helm template maestro-agent ./charts/maestro-agent --namespace $(agent_namespace)
.PHONY: template-agent
