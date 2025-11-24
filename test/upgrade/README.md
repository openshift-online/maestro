# Maestro Upgrade Tests

This directory contains upgrade tests that verify Maestro's ability to maintain compatibility and functionality during rolling upgrades between versions.

## Overview

The upgrade tests simulate a real-world upgrade scenario where:
1. An older version of Maestro server and agent are deployed
2. Resources are created and workloads are initialized
3. The server is upgraded to the latest version
4. The agent and gRPC work client are upgraded to the latest version
5. All functionality is verified to work across version boundaries

## Running the Tests

### Full Upgrade Test Suite

To run the complete upgrade test sequence (recommended):

```bash
cd $HOME/go/src/github.com/openshift-online/maestro
make upgrade-test/run
```

This will:
1. Set up the test environment with the last stable release
2. Deploy and initialize test workloads
3. Upgrade the Maestro server to the latest version
4. Run upgrade tests against the upgraded server
5. Upgrade the Maestro agent and gRPC client
6. Verify all functionality works with the latest versions

### Custom Release Testing

To test upgrade from a specific version:

```bash
last_tag="<a specific release version>" make upgrade-test/run
```

## Test Scenarios

The upgrade test suite (`test.sh`) validates upgrade compatibility through three distinct phases, ensuring both backward and forward compatibility:

### Phase 1: Initial Setup with Last Stable Release
**Purpose**: Establish baseline with the previous version
- Deploy Maestro server (last stable release)
- Deploy Maestro agent (last stable release)
- Deploy mock work-server with last stable gRPC work client
- Initialize test workloads:
  - Create ManifestWork for a standard Deployment
  - Create read-only ManifestWork to watch Deployment status
  - Create nested ManifestWork (ManifestWork containing another ManifestWork)

### Phase 2: Server Upgrade Test (Backward Compatibility)
**Purpose**: Verify old clients can work with new server

**Step 1** - Upgrade server only:
```bash
deploy_agent="false" make e2e-test/setup
```
- Upgrades Maestro server to latest version
- Keeps Maestro agent on last stable version
- Keeps gRPC work client on last stable version

**Step 2** - Run upgrade tests with old test image:
```bash
IMAGE="$img_registry/maestro-e2e:$last_tag" ${PWD}/test/upgrade/script/run.sh
```
- Uses last stable version's test suite
- Validates:
  - **Update Deployment via Work**: Old client can update deployments through new server
  - **Watch Deployment Status**: Old client can watch status feedback through new server
  - **Update Nested Work**: Old client can manage nested ManifestWorks through new server
- Ensures new server maintains backward compatibility with old clients

**Step 3** - Run e2e tests with old test image:
```bash
IMAGE="$img_registry/maestro-e2e:$last_tag" ${PWD}/test/e2e/istio/test.sh
```
- Runs full e2e test suite with old version's expectations
- Validates that all existing functionality still works

### Phase 3: Agent and Client Upgrade Test (Forward Compatibility)
**Purpose**: Verify new clients work with new server

**Step 1** - Upgrade agent and work-server:
```bash
make e2e-test/setup
```
- Upgrades Maestro agent to latest version
- Upgrades mock work-server to latest gRPC work client
- Server already upgraded in Phase 2

**Step 2** - Run upgrade tests with latest test image:
```bash
${PWD}/test/upgrade/script/run.sh
```
- Uses latest version's test suite
- Validates:
  - **Update Deployment via Work**: New client can update deployments through new server
  - **Watch Deployment Status**: New client can watch status feedback through new server
  - **Update Nested Work**: New client can manage nested ManifestWorks through new server
- Verifies existing workloads created with old versions still work with new versions

**Step 3** - Run e2e tests with latest test image:
```bash
${PWD}/test/e2e/istio/test.sh
```
- Runs full e2e test suite with latest version
- Validates all new features and functionality

### Workload Test Cases

Throughout all phases, the test suite validates these specific scenarios:

#### 1. Update Deployment via Work
- Creates a ManifestWork containing a Deployment
- Verifies the Deployment is applied to the target cluster
- Updates the Deployment through the ManifestWork (changes replica count)
- Ensures the status is correctly reported back to the Maestro server
- **Upgrade validation**: Workloads created with old version can be updated with new version

#### 2. Watch Deployment Status via Readonly Work
- Creates a read-only ManifestWork to watch an existing Deployment
- Verifies status feedback is synchronized
- Updates the Deployment directly on the cluster
- Ensures the updated status is correctly reported through the read-only work
- **Upgrade validation**: Status watching works across version boundaries

#### 3. Update Nested Work via Work
- Creates a ManifestWork that contains another ManifestWork (nested work)
- Verifies the nested ManifestWork is applied
- Updates the nested ManifestWork through the parent work
- Ensures status propagation works correctly for nested resources
- **Upgrade validation**: Complex resource hierarchies survive upgrades
