# Maestro Test Documentation

This document provides a comprehensive overview of all testing strategies and test suites in the Maestro project.

## Table of Contents

- [Test Overview](#test-overview)
- [Unit and Integration Tests](#unit-and-integration-tests)
- [End-to-End (E2E) Tests](#end-to-end-e2e-tests)
- [Upgrade Tests](#upgrade-tests)
- [Long Running Tests](#long-running-tests)
- [Manual E2E and Upgrade Tests on Custom Clusters](#manual-e2e-and-upgrade-tests-on-custom-clusters)
- [Running Tests Locally](#running-tests-locally)

---

## Test Overview

Maestro employs a comprehensive testing strategy that includes:

1. **Unit Tests**: Test individual functions and components in isolation
2. **Integration Tests**: Test interactions between components (MQTT and gRPC brokers)
3. **E2E Tests**: Test complete workflows in different configurations
4. **Upgrade Tests**: Verify backward compatibility during version upgrades
5. **Long Running Tests**: Automated daily tests on real Azure AKS clusters with upgrade scenarios

---

## Unit and Integration Tests

### Unit Tests

Unit tests validate individual functions and components in isolation.

**Location**: `pkg/*/` directories alongside source code

**Running Unit Tests**:
```bash
make test
```

This command runs all unit tests across the codebase using Go's testing framework.

### Integration Tests

Integration tests validate the interaction between Maestro components and message brokers.

**Location**: `test/integration/`

**Test Types**:

1. **MQTT Integration Tests**
   - Tests Maestro server and agent communication via MQTT broker
   - Validates resource delivery and status updates through MQTT

   ```bash
   make test-integration-mqtt
   ```

2. **gRPC Integration Tests**
   - Tests Maestro server and agent communication via gRPC broker
   - Validates resource delivery and status updates through gRPC

   ```bash
   make test-integration-grpc
   ```

3. **Run All Integration Tests**
   ```bash
   make test-integration
   ```

**What They Test**:
- CloudEvents message transport (MQTT and gRPC)
- Resource synchronization between server and agent
- Status feedback mechanisms
- Error handling and retry logic

---

## End-to-End (E2E) Tests

E2E tests validate complete workflows in containerized environments that simulate production deployments.

### Test Configurations

#### 1. Standard E2E Test
**Purpose**: Basic functional testing with TLS enabled and multiple server replicas

**Configuration**:
- 2 Maestro server replicas
- TLS enabled
- MQTT message broker (default)

**Running**:
```bash
make e2e-test
```

**Environment Variables**:
```bash
container_tool=docker
SERVER_REPLICAS=2
ENABLE_MAESTRO_TLS=true
```

**CI Workflow**: `.github/workflows/e2e.yml` (job: `e2e`)

---

#### 2. Broadcast Subscription E2E Test
**Purpose**: Test broadcast subscription-type with multiple server instances

**Configuration**:
- 3 Maestro server replicas
- Broadcast subscription-type enabled
- Tests broadcast subscription mode where messages are delivered to all server instances

**Running**:
```bash
SERVER_REPLICAS=3 ENABLE_BROADCAST_SUBSCRIPTION=true make e2e-test
```

**Environment Variables**:
```bash
container_tool=docker
SERVER_REPLICAS=3
ENABLE_BROADCAST_SUBSCRIPTION=true
```

**CI Workflow**: `.github/workflows/e2e.yml` (job: `e2e-broadcast-subscription`)

---

#### 3. Istio Service Mesh E2E Test
**Purpose**: Validate Maestro functionality with Istio service mesh

**Configuration**:
- Istio service mesh enabled
- Tests service-to-service communication through Istio
- Validates mTLS and traffic management

**Running**:
```bash
ENABLE_ISTIO=true make e2e-test/istio
```

**Environment Variables**:
```bash
container_tool=docker
ENABLE_ISTIO=true
```

**CI Workflow**: `.github/workflows/e2e.yml` (job: `e2e-with-istio`)

---

#### 4. gRPC Broker E2E Test
**Purpose**: Test Maestro with gRPC message broker instead of MQTT

**Configuration**:
- 2 Maestro server replicas
- gRPC message broker
- TLS enabled

**Running**:
```bash
MESSAGE_DRIVER_TYPE=grpc SERVER_REPLICAS=2 ENABLE_MAESTRO_TLS=true make e2e-test
```

**Environment Variables**:
```bash
container_tool=docker
SERVER_REPLICAS=2
MESSAGE_DRIVER_TYPE=grpc
ENABLE_MAESTRO_TLS=true
```

**CI Workflow**: `.github/workflows/e2e.yml` (job: `e2e-grpc-broker`)

---

### E2E Test Scenarios

All E2E test configurations validate these core scenarios:

1. **Resource Creation and Delivery**
   - Create resources via Maestro API
   - Verify resources are delivered to agents via CloudEvents
   - Confirm resources are applied to target clusters

2. **Status Feedback**
   - Monitor resource status updates from agents
   - Validate status synchronization to Maestro server
   - Verify status visibility through API

3. **Resource Updates**
   - Update existing resources
   - Validate updates are propagated to agents
   - Confirm status reflects updated state

4. **Resource Deletion**
   - Delete resources via API
   - Verify cleanup on target clusters
   - Confirm status updates reflect deletion

5. **Multi-Server Coordination** (when SERVER_REPLICAS > 1)
   - Load distribution across server instances
   - Consistent resource state across replicas
   - Failover scenarios

---

## Upgrade Tests

Upgrade tests verify backward compatibility during rolling upgrades between Maestro versions.

**Location**: `test/upgrade/`

**Documentation**: See [test/upgrade/README.md](upgrade/README.md) for detailed information

### Test Phases

#### Phase 1: Initial Setup with Last Stable Release
- Deploy Maestro server (last stable release)
- Deploy Maestro agent (last stable release)
- Deploy mock work-server with last stable gRPC work client
- Initialize test workloads

#### Phase 2: Server Upgrade Test (Backward Compatibility)
- Upgrade server to latest version
- Keep agent and client on last stable version
- Run tests with old test image
- Validates old clients work with new server

#### Phase 3: Agent and Client Upgrade Test (Backward Compatibility)
- Upgrade agent and work-server to latest version
- Server already on latest version
- Run tests with latest test image
- Validates new clients work with new server

### Running Upgrade Tests

**Full upgrade test suite**:
```bash
ENABLE_ISTIO=true make upgrade-test
```

**Test from specific version**:
```bash
last_tag="v1.2.3" ENABLE_ISTIO=true make upgrade-test
```

**CI Workflow**: `.github/workflows/e2e.yml` (job: `upgrade`)

### Workload Test Cases

1. **Update Deployment via Work**
   - Create ManifestWork containing a Deployment
   - Update Deployment through ManifestWork
   - Verify status reporting

2. **Watch Deployment Status via Readonly Work**
   - Create read-only ManifestWork to watch Deployment
   - Verify status feedback synchronization

3. **Update Nested Work via Work**
   - Create ManifestWork containing another ManifestWork
   - Update nested ManifestWork through parent
   - Verify status propagation

---

## Long Running Tests

Long running tests execute daily on real Azure AKS clusters to validate production-like scenarios.

**Location**: `.github/workflows/longrunning.yml`

**Schedule**: Daily at 2:00 AM UTC (cron: `0 2 * * *`)

**Manual Trigger**: Via `workflow_dispatch`

### Test Infrastructure

**Clusters**:
- **Service Cluster**: Runs Maestro server
- **Management Cluster**: Runs Maestro agent

**Authentication**: Azure OIDC with federated identity

### Test Sequence

1. **Initial E2E Test**
   - Run full E2E test suite on deployed infrastructure
   - Validate all functionality works

2. **Server Upgrade**
   - Roll out latest Maestro server version
   - Run E2E tests to verify backward compatibility
   - Validate agent (old version) works with server (new version)

3. **Agent Upgrade**
   - Roll out latest Maestro agent version
   - Run E2E tests to verify forward compatibility
   - Validate full system with latest versions

### Running Long Running Tests

**Automated**: Runs daily via GitHub Actions schedule

**Manual Trigger**:
```bash
# Via GitHub UI: Actions → Maestro Long Running Test → Run workflow
```

**Requirements**:
- Azure credentials configured in GitHub secrets:
  - `AZURE_CLIENT_ID`
  - `AZURE_TENANT_ID`
  - `AZURE_SUBSCRIPTION_ID`
  - `SVC_RESOURCE_GROUP`
  - `SVC_CLUSTER_NAME`
  - `MGMT_RESOURCE_GROUP`
  - `MGMT_CLUSTER_NAME`
  - `SLACK_WEBHOOK_URL` (for notifications)

**Slack Notifications**: Results posted to configured Slack channel

---

## Manual E2E and Upgrade Tests on Custom Clusters

The manual E2E test workflow allows you to run E2E and upgrade tests on custom Azure AKS clusters. This is useful for testing specific configurations, validating fixes on target infrastructure, or verifying upgrades before production rollout.

**IMPORTANT**: Before delivering a new version of Maestro, upgrade tests MUST be run and pass to ensure compatibility with existing deployments. This validates that the new version can be safely rolled out without breaking existing functionality.

**Location**: `.github/workflows/manual-e2e.yml`

**Trigger**: Manual dispatch with custom cluster parameters

### Test Workflow

The manual E2E test performs the same upgrade testing sequence as the long running tests:

1. **Initial E2E Test**
   - Run full E2E test suite on existing deployed infrastructure
   - Uses latest E2E test image from the main branch
   - Validates current deployment state

2. **Server Upgrade Test**
   - Upgrade Maestro server to latest version using `make e2e/rollout`
   - Run E2E tests to verify backward compatibility
   - Validates that existing agent can work with upgraded server

3. **Agent Upgrade Test**
   - Upgrade Maestro agent to latest version using `make e2e/rollout`
   - Run E2E tests to verify forward compatibility
   - Validates full system with latest versions

### Running Manual E2E Tests

**Via GitHub UI**:
1. Go to Actions → Manual E2E Test → Run workflow
2. Provide cluster parameters:
   - Service cluster resource group
   - Service cluster AKS name
   - Management cluster resource group
   - Management cluster AKS name

**Requirements**:
- Azure credentials configured in GitHub secrets:
  - `AZURE_CLIENT_ID`
  - `AZURE_TENANT_ID`
  - `AZURE_SUBSCRIPTION_ID`
  - `SLACK_WEBHOOK_URL` (for notifications)
- Maestro already deployed on both clusters
- Clusters must be accessible via Azure OIDC authentication

**Use Cases**:
- Test upgrade compatibility on specific cluster configurations
- Validate bug fixes on production-like infrastructure
- Pre-production upgrade verification
- Custom environment testing

**Slack Notifications**: Results posted with cluster details and job status

---

## Running Tests Locally

### Prerequisites

- Go 1.24.4+
- Docker or Podman
- PostgreSQL (for integration tests)
- Eclipse Mosquitto (for MQTT tests)
- Ginkgo test framework

### Setup

1. **Install Ginkgo**:
   ```bash
   go install github.com/onsi/ginkgo/v2/ginkgo@v2.15.0
   ```

2. **Set up local database**:
   ```bash
   make db/setup
   ```

3. **Set up local MQTT broker** (for MQTT tests):
   ```bash
   make mqtt/setup
   ```

### Running Tests

**Unit tests**:
```bash
make test
```

**Integration tests**:
```bash
# All integration tests
make test-integration

# MQTT only
make test-integration-mqtt

# gRPC only
make test-integration-grpc
```

**E2E tests**:
```bash
# Standard E2E
make e2e-test

# With Istio
ENABLE_ISTIO=true make e2e-test/istio

# With gRPC broker
MESSAGE_DRIVER_TYPE=grpc make e2e-test

# With broadcast subscription
ENABLE_BROADCAST_SUBSCRIPTION=true SERVER_REPLICAS=3 make e2e-test
```

**Upgrade tests**:
```bash
ENABLE_ISTIO=true make upgrade-test
```

### Cleanup

**Teardown database**:
```bash
make db/teardown
```

**Teardown MQTT broker**:
```bash
make mqtt/teardown
```

**Clean test environment**:
```bash
make test-env/cleanup
```
