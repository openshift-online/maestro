# Trace ManifestWork Skill

Complete tracing of ManifestWork resources through the Maestro system.

## Overview

This skill helps trace ManifestWork resources through their complete lifecycle in the Maestro system, connecting user-created work names, database resource IDs, AppliedManifestWorks, and applied manifests.

## What This Skill Does

The skill provides bidirectional tracing between:
- User-created work names (assigned by gRPC client)
- Database resource IDs (DB primary key, CloudEvent resourceid)
- AppliedManifestWorks (on management cluster)
- Applied manifests (Deployments, Services, etc.)

## Quick Start

### Trace from Resource ID

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh \
  --resource-id "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
```

### Trace from Manifest

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh \
  --manifest-kind deployment \
  --manifest-name maestro-e2e-upgrade-test \
  --manifest-namespace default
```

### Trace from User Work Name

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh \
  --work-name "e44ec579-9646-549a-b679-db8d19d6da37"
```

## Prerequisites

- `kubectl` installed and configured
- KUBECONFIG set to management cluster
- Access to Maestro database pod (postgres-breakglass or maestro-db)
- Appropriate RBAC permissions

## Files in This Skill

### Scripts

- `scripts/trace.sh` - Complete trace from any entry point

### References

- `references/maestro-data-flow.md` - Complete documentation of Maestro resource flow
- `references/troubleshooting-guide.md` - Common issues and solutions

### Examples

- `examples/trace-by-resource-id.md` - Trace starting from resource ID
- `examples/trace-by-manifest.md` - Trace starting from manifest name
- `examples/trace-by-work-name.md` - Trace starting from user work name

## Common Use Cases

1. **Find work name from manifest**: You see a deployment and want to know which work created it
2. **Find manifests from work name**: You have a work name and want to see what it deployed
3. **Verify work application**: Check if a work was successfully applied to the cluster
4. **Debug missing resources**: Investigate why a work's manifests aren't on the cluster
5. **Understand deletion**: Learn how works are deleted and verify deletion completed

## Key Concepts

### Identifiers

- **User-created work name**: Name assigned by user (e.g., `e44ec579-9646-549a-b679-db8d19d6da37`)
- **Resource ID**: Database primary key (e.g., `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`)
- **AppliedManifestWork name**: Format `{agentID}-{resourceID}`
- **Manifest name**: Actual Kubernetes resource name (e.g., `maestro-e2e-upgrade-test`)

### Data Flow

```
User creates ManifestWork (work name)
    ↓
gRPC Client generates UID (resource ID)
    ↓
Server stores in DB (id = resource ID, payload contains work name)
    ↓
Server sends to Agent (using resource ID as manifestWorkName)
    ↓
Agent creates AppliedManifestWork ({agentID}-{resourceID})
    ↓
Agent applies manifests (with ownerReference to AppliedManifestWork)
```

## Troubleshooting

See `references/troubleshooting-guide.md` for detailed troubleshooting steps.

Common issues:
- kubectl not found → Install kubectl
- KUBECONFIG not set → Export KUBECONFIG to management cluster
- Database pod not found → Verify cluster and namespace
- Work not found → Check if deleted, search by partial name
- AppliedManifestWork not found → Check agent logs, verify work was applied

## Related Documentation

- Main Maestro docs: `/docs/maestro.md`
- CloudEvents spec: https://cloudevents.io/
- OCM ManifestWork: https://open-cluster-management.io/concepts/manifestwork/

## Support

For issues or questions:
1. Review `references/troubleshooting-guide.md`
2. Check Maestro documentation
3. Contact Maestro team with diagnostic information
