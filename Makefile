.DEFAULT_GOAL := help

# CGO_ENABLED=0 is not FIPS compliant. large commercial vendors and FedRAMP require FIPS compliant crypto
CGO_ENABLED := 1

# Enable users to override the golang used to accomodate custom installations
GO ?= go

# Allow overriding `oc` command.
# Used by pr_check.py to ssh deploy inside private Hive cluster via bastion host.
oc:=oc

# The version needs to be different for each deployment because otherwise the
# cluster will not pull the new image from the internal registry:
version:=$(shell date +%s)
# Tag for the image:
image_tag ?= $(version)

# The namespace and the environment are calculated from the name of the user to
# avoid clashes in shared infrastructure:
environment:=${USER}
namespace ?= maestro-${USER}
agent_namespace ?= maestro-agent-${USER}

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
db_sslmode:=disable
db_image?=docker.io/library/postgres:14.2

# Message broker connection details
mqtt_host=maestro-mqtt.$(namespace)
mqtt_port=1883
mqtt_user:=maestro
mqtt_password_file=${PWD}/secrets/mqtt.password
mqtt_config_file=${PWD}/secrets/mqtt.config

# Log verbosity level
klog_v:=4

# Location of the JSON web key set used to verify tokens:
jwks_url:=https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs

# consumer name from the database. it is used by the maestro agent to identify itself
consumer_name ?= cluster1

# Client id and secret are used to interact with other UHC services
CLIENT_ID ?= maestro
CLIENT_SECRET ?= maestro

# Enable JWT token verify and authz middleware
ENABLE_JWT ?= true
ENABLE_AUTHZ ?= true
ENABLE_OCM_MOCK ?= false
ENABLE_GRPC_SERVER ?= false

# message driver type, mqtt or grpc, default is mqtt.
MESSAGE_DRIVER_TYPE ?= mqtt

# default replicas for maestro server
SERVER_REPLICAS ?= 1

# Enable set images
POSTGRES_IMAGE ?= docker.io/library/postgres:14.2
MQTT_IMAGE ?= docker.io/library/eclipse-mosquitto:2.0.18

# Test output files
unit_test_json_output ?= ${PWD}/unit-test-results.json
integration_test_json_output ?= ${PWD}/integration-test-results.json

# Prints a list of useful targets.
help:
	@echo ""
	@echo "OpenShift CLuster Manager Example Service"
	@echo ""
	@echo "make verify               verify source code"
	@echo "make lint                 run golangci-lint"
	@echo "make binary               compile binaries"
	@echo "make install              compile binaries and install in GOPATH bin"
	@echo "make run                  run the application"
	@echo "make run/docs             run swagger and host the api spec"
	@echo "make test                 run unit tests"
	@echo "make test-integration     run integration tests"
	@echo "make generate             generate openapi modules"
	@echo "make image                build docker image"
	@echo "make push                 push docker image"
	@echo "make deploy               deploy via templates to local openshift instance"
	@echo "make undeploy             undeploy from local openshift instance"
	@echo "make project              create and use an Example project"
	@echo "make clean                delete temporary generated files"
	@echo "$(fake)"
.PHONY: help

# Encourage consistent tool versions
OPENAPI_GENERATOR_VERSION:=5.4.0
GO_VERSION:=go1.22.

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

ifndef OCM_BASE_URL
	OCM_BASE_URL:="https://api.integration.openshift.com"
endif

# Checks if a GOPATH is set, or emits an error message
check-gopath:
ifndef GOPATH
	$(error GOPATH is not set)
endif
.PHONY: check-gopath

# Verifies that source passes standard checks.
verify: check-gopath
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
	${GO} build ./cmd/maestro
.PHONY: binary

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
test-integration:
	MAESTRO_ENV=testing gotestsum --jsonfile-timing-events=$(integration_test_json_output) --format $(TEST_SUMMARY_FORMAT) -- -p 1 -ldflags -s -v -timeout 1h $(TESTFLAGS) \
			./test/integration
.PHONY: test-integration

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
	maestro server
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
		templates/*-template.json \
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


# NOTE multiline variables are a PITA in Make. To use them in `oc process` later on, we need to first
# export them as environment variables, then use the environment variable in `oc process`
%-template:
	oc process \
		--filename="templates/$*-template.yml" \
		--local="true" \
		--ignore-unknown-parameters="true" \
		--param="ENVIRONMENT=$(MAESTRO_ENV)" \
		--param="KLOG_V=$(klog_v)" \
		--param="SERVER_REPLICAS=$(SERVER_REPLICAS)" \
		--param="DATABASE_HOST=$(db_host)" \
		--param="DATABASE_NAME=$(db_name)" \
		--param="DATABASE_PASSWORD=$(db_password)" \
		--param="DATABASE_PORT=$(db_port)" \
		--param="DATABASE_USER=$(db_user)" \
		--param="DB_SSLMODE=$(db_sslmode)" \
		--param="POSTGRES_IMAGE=$(POSTGRES_IMAGE)" \
		--param="MQTT_HOST=$(mqtt_host)" \
		--param="MQTT_PORT=$(mqtt_port)" \
		--param="MQTT_USER=$(mqtt_user)" \
		--param="MQTT_PASSWORD=$(shell cat $(mqtt_password_file))" \
		--param="MQTT_ROOT_CERT=" \
		--param="MQTT_CLIENT_CERT=" \
		--param="MQTT_CLIENT_KEY=" \
		--param="MQTT_IMAGE=$(MQTT_IMAGE)" \
		--param="IMAGE_REGISTRY=$(internal_image_registry)" \
		--param="IMAGE_REPOSITORY=$(image_repository)" \
		--param="IMAGE_TAG=$(image_tag)" \
		--param="VERSION=$(version)" \
		--param="ENABLE_JWT=$(ENABLE_JWT)" \
		--param="ENABLE_AUTHZ=$(ENABLE_AUTHZ)" \
		--param="AUTHZ_RULES=$$AUTHZ_RULES" \
		--param="ENABLE_SENTRY"=false \
		--param="SENTRY_KEY"=TODO \
		--param="JWKS_URL=$(jwks_url)" \
		--param="OCM_SERVICE_CLIENT_ID=$(CLIENT_ID)" \
		--param="OCM_SERVICE_CLIENT_SECRET=$(CLIENT_SECRET)" \
		--param="TOKEN=$(token)" \
		--param="OCM_BASE_URL=$(OCM_BASE_URL)" \
		--param="ENVOY_IMAGE=$(envoy_image)" \
		--param="ENABLE_JQS="false \
		--param="AGENT_NAMESPACE=${agent_namespace}" \
		--param="EXTERNAL_APPS_DOMAIN=${external_apps_domain}" \
		--param="CONSUMER_NAME=$(consumer_name)" \
		--param="ENABLE_OCM_MOCK=$(ENABLE_OCM_MOCK)" \
		--param="ENABLE_GRPC_SERVER=$(ENABLE_GRPC_SERVER)" \
		--param="MESSAGE_DRIVER_TYPE"=$(MESSAGE_DRIVER_TYPE) \
	> "templates/$*-template.json"


.PHONY: project
project:
	$(oc) new-project "$(namespace)" || $(oc) project "$(namespace)" || true

.PHONY: agent-project
agent-project:
	$(oc) new-project "$(agent_namespace)" || $(oc) project "$(agent_namespace)" || true

.PHONY: image
image: cmds
	$(container_tool) build -t "$(external_image_registry)/$(image_repository):$(image_tag)" .

.PHONY: push
push: image project
	$(container_tool) push "$(external_image_registry)/$(image_repository):$(image_tag)"

deploy-%: project %-template
	$(oc) apply -n $(namespace) --filename="templates/$*-template.json" | egrep --color=auto 'configured|$$'

undeploy-%: project %-template
	$(oc) delete -n $(namespace) --filename="templates/$*-template.json" | egrep --color=auto 'deleted|$$'

.PHONY: deploy-agent
deploy-agent: push agent-project agent-template
	$(oc) apply -n $(agent_namespace) --filename="templates/agent-template.json" | egrep --color=auto 'configured|$$'

.PHONY: undeploy-agent
undeploy-agent: agent-project agent-template
	$(oc) delete -n $(agent_namespace) --filename="templates/agent-template.json" | egrep --color=auto 'deleted|$$'

.PHONY: template
template: \
	secrets-template \
	db-template \
	mqtt-template \
	service-template \
	route-template \
	$(NULL)

# Depending on `template` first helps clustering the "foo configured", "bar unchanged",
# "baz deleted" messages at the end, after all the noisy templating.
.PHONY: deploy
deploy: \
	push \
	template \
	deploy-secrets \
	deploy-db \
	deploy-mqtt \
	deploy-service \
	deploy-route \
	$(NULL)

.PHONY: undeploy
undeploy: \
	template \
	undeploy-secrets \
	undeploy-db \
	undeploy-mqtt \
	undeploy-service \
	undeploy-route \
	$(NULL)

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
	@echo $(shell LC_CTYPE=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 13) > $(mqtt_password_file)

.PHONY: mqtt/setup
mqtt/setup: mqtt/prepare
	@echo '{"brokerHost":"localhost:1883","username":"$(mqtt_user)","password":"$(shell cat $(mqtt_password_file))","topics":{"sourceEvents":"sources/maestro/consumers/+/sourceevents","agentEvents":"sources/maestro/consumers/+/agentevents"}}' > $(mqtt_config_file)
	$(container_tool) run --rm -v $(shell pwd)/hack:/mosquitto/data:z $(MQTT_IMAGE) mosquitto_passwd -c -b /mosquitto/data/mosquitto-passwd.txt $(mqtt_user) $(shell cat $(mqtt_password_file))
	$(container_tool) run --name mqtt-maestro -p 1883:1883 -v $(shell pwd)/hack/mosquitto-passwd.txt:/mosquitto/config/password.txt -v $(shell pwd)/hack/mosquitto.conf:/mosquitto/config/mosquitto.conf -d $(MQTT_IMAGE)

.PHONY: mqtt/teardown
mqtt/teardown:
	$(container_tool) stop mqtt-maestro
	$(container_tool) rm mqtt-maestro

crc/login:
	@echo "Logging into CRC"
	@crc console --credentials -ojson | jq -r .clusterConfig.adminCredentials.password | oc login --username kubeadmin --insecure-skip-tls-verify=true https://api.crc.testing:6443
	@oc whoami --show-token | $(container_tool) login --username kubeadmin --password-stdin "$(external_image_registry)"
.PHONY: crc/login

e2e-test/setup:
	./test/e2e/setup/e2e_setup.sh
.PHONY: e2e-test/setup

e2e-test/teardown:
	./test/e2e/setup/e2e_teardown.sh
.PHONY: e2e-test/teardown

e2e-test: e2e-test/teardown e2e-test/setup
	ginkgo -v --fail-fast --label-filter="!(e2e-tests-spec-resync-reconnect||e2e-tests-status-resync-reconnect)" \
	--output-dir="${PWD}/test/e2e/report" --json-report=report.json --junit-report=report.xml \
	${PWD}/test/e2e/pkg -- \
	-api-server=https://$(shell cat ${PWD}/test/e2e/.external_host_ip):30080 \
	-grpc-server=$(shell cat ${PWD}/test/e2e/.external_host_ip):30090 \
	-consumer-name=$(shell cat ${PWD}/test/e2e/.consumer_name) \
	-consumer-kubeconfig=${PWD}/test/e2e/.kubeconfig
.PHONY: e2e-test
