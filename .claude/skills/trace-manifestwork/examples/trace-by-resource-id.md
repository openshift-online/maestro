# Example: Trace by Resource ID

This example shows a complete trace starting from a database resource ID.

## Input

Resource ID: `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`

## Command

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh --resource-id "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
```

## Output

```
═══════════════════════════════════════════════════
  ManifestWork Trace
═══════════════════════════════════════════════════

Entry Point: Resource ID
  Resource ID: 55c61e54-a3f6-563d-9fec-b1fe297bdfdb

[1/3] Querying database for user work name...
Database: maestro-db (Service cluster)

 id                                   | user_work_name                       |          manifests          |      created_at         |      updated_at         | deleted_at
--------------------------------------+--------------------------------------+-----------------------------+-------------------------+-------------------------+------------
 55c61e54-a3f6-563d-9fec-b1fe297bdfdb | e44ec579-9646-549a-b679-db8d19d6da37 | [{"kind":"Deployment",...}] | 2024-01-15 10:30:00.123 | 2024-01-15 10:32:15.456 |
(1 row)

[2/3] Finding AppliedManifestWork on cluster...
  AppliedManifestWork: f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb

[3/3] Listing applied resources...
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

**Found Relationships:**
- Resource ID: `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- User Work Name: `e44ec579-9646-549a-b679-db8d19d6da37`
- AppliedManifestWork: `f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- Applied Manifest: `Deployment/default/maestro-e2e-upgrade-test`

**Status**: ✓ Active (not deleted)

## Next Steps

To view more details:

```bash
# View full AppliedManifestWork
kubectl get appliedmanifestwork f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb -o yaml

# View the deployment
kubectl get deployment maestro-e2e-upgrade-test -n default -o yaml

# Check deployment status
kubectl get deployment maestro-e2e-upgrade-test -n default
```
