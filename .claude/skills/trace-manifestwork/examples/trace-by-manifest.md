# Example: Trace by Manifest Details

This example shows a complete trace starting from a manifest (Deployment) on the management cluster.

## Input

- Manifest Kind: `deployment`
- Manifest Name: `maestro-e2e-upgrade-test`
- Manifest Namespace: `default`
- Service Cluster Context: `svc-cluster`
- Management Cluster Context: `mgmt-cluster`

## Command

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh \
  --manifest-kind deployment \
  --manifest-name maestro-e2e-upgrade-test \
  --manifest-namespace default \
  --svc-context svc-cluster \
  --mgmt-context mgmt-cluster
```

## Output

```
═══════════════════════════════════════════════════
  ManifestWork Trace
═══════════════════════════════════════════════════

Entry Point: Manifest Details
  Kind:      deployment
  Name:      maestro-e2e-upgrade-test
  Namespace: default

[1/4] Getting AppliedManifestWork from manifest...
Switching to management cluster context: mgmt-cluster
  AppliedManifestWork: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb

[2/4] Extracting Resource ID from AppliedManifestWork...
  Resource ID: 55c61e54-a3f6-563d-9fec-b1fe297bdfdb

[3/4] Querying database for user work name...
Switching to service cluster context: svc-cluster
Database: maestro-db (Service cluster)

 id                                   | user_work_name                       |      created_at         |      updated_at         | deleted_at
--------------------------------------+--------------------------------------+-------------------------+-------------------------+------------
 55c61e54-a3f6-563d-9fec-b1fe297bdfdb | e44ec579-9646-549a-b679-db8d19d6da37 | 2024-01-15 10:30:00.123 | 2024-01-15 10:32:15.456 |
(1 row)

[4/4] Listing all applied resources...
Applied Manifests:
────────────────────────────
Resource Type       Namespace           Name
───────────────     ──────────────      ────────────────────
deployments         default             maestro-e2e-upgrade-test

═══════════════════════════════════════════════════
  Trace Complete
═══════════════════════════════════════════════════
```

## Summary

**Trace Path:**
```
Deployment (maestro-e2e-upgrade-test)
  ↓ ownerReference
AppliedManifestWork (f1d8a...55c61e54...)
  ↓ spec.manifestWorkName
Resource ID (55c61e54-a3f6-563d-9fec-b1fe297bdfdb)
  ↓ DB lookup
User Work Name (e44ec579-9646-549a-b679-db8d19d6da37)
```

**Found Relationships:**
- Manifest: `Deployment/default/maestro-e2e-upgrade-test`
- AppliedManifestWork: `f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- Resource ID: `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- User Work Name: `e44ec579-9646-549a-b679-db8d19d6da37`

**Status**: ✓ Active (not deleted)

## Verification

Verify the ownerReference points to the AppliedManifestWork:

```bash
kubectl get deployment maestro-e2e-upgrade-test -n default -o jsonpath='{.metadata.ownerReferences[]}'
```

Expected output:
```json
{
  "apiVersion": "work.open-cluster-management.io/v1",
  "kind": "AppliedManifestWork",
  "name": "f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb",
  "uid": "1aa9d3c4-74bf-42ff-a8ae-3d7d930da845"
}
```

## Next Steps

To find other manifests in the same work:

```bash
# List all manifests in this AppliedManifestWork
kubectl get appliedmanifestwork \
  f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb \
  -o jsonpath='{range .status.appliedResources[*]}{.resource}{" / "}{.namespace}{" / "}{.name}{"\n"}{end}'
```
