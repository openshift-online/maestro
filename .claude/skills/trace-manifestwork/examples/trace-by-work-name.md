# Example: Trace by User-Created Work Name

This example shows a complete trace starting from the user-created work name.

## Input

User-Created Work Name: `e44ec579-9646-549a-b679-db8d19d6da37`

## Command

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh --work-name "e44ec579-9646-549a-b679-db8d19d6da37"
```

## Output (Service Cluster with maestro-db)

```
═══════════════════════════════════════════════════
  ManifestWork Trace
═══════════════════════════════════════════════════

Entry Point: User-Created Work Name
  Work Name: e44ec579-9646-549a-b679-db8d19d6da37

[1/3] Querying database for resource ID...
Database: maestro-db (Service cluster)

 id                                   | user_work_name                       |          manifests          |      created_at         |      updated_at         | deleted_at
--------------------------------------+--------------------------------------+-----------------------------+-------------------------+-------------------------+------------
 55c61e54-a3f6-563d-9fec-b1fe297bdfdb | e44ec579-9646-549a-b679-db8d19d6da37 | [{"kind":"Deployment",...}] | 2024-01-15 10:30:00.123 | 2024-01-15 10:32:15.456 |
(1 row)

Please copy the resource ID from the query result above and provide it to find AppliedManifestWork.
Then run: ./trace.sh --resource-id '55c61e54-a3f6-563d-9fec-b1fe297bdfdb'

═══════════════════════════════════════════════════
  Trace Complete
═══════════════════════════════════════════════════
```

## Follow-up Command

After getting the resource ID from the database query, run:

```bash
.claude/skills/trace-manifestwork/scripts/trace.sh --resource-id "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
```

This completes the trace by finding the AppliedManifestWork and applied manifests.

## Output (ARO-HCP INT with postgres-breakglass)

For environments using postgres-breakglass, the output is different:

```
═══════════════════════════════════════════════════
  ManifestWork Trace
═══════════════════════════════════════════════════

Entry Point: User-Created Work Name
  Work Name: e44ec579-9646-549a-b679-db8d19d6da37

[1/3] Querying database for resource ID...
Database: postgres-breakglass (ARO-HCP INT)

⚠ Interactive database access required:
1. Run: kubectl -n maestro exec -it postgres-breakglass-7b8c9d6f5-abc12 -- /bin/bash
2. Inside pod, run: connect
3. Paste this SQL query:

SELECT id, payload->'metadata'->>'name' AS user_work_name, payload->'spec'->'workload'->'manifests' AS manifests, created_at, updated_at, deleted_at FROM resources WHERE payload->'metadata'->>'name' = 'e44ec579-9646-549a-b679-db8d19d6da37';

⚠ Manual step required:
1. Copy the resource ID from the database query result above
2. Run: ./trace.sh --resource-id '<resource-id>' to complete the trace

═══════════════════════════════════════════════════
  Trace Complete
═══════════════════════════════════════════════════
```

## Manual Database Query Steps (postgres-breakglass)

1. Connect to the pod:
```bash
kubectl -n maestro exec -it postgres-breakglass-7b8c9d6f5-abc12 -- /bin/bash
```

2. Inside the pod, connect to database:
```bash
connect
```

3. Run the SQL query:
```sql
SELECT id, payload->'metadata'->>'name' AS user_work_name,
       payload->'spec'->'workload'->'manifests' AS manifests,
       created_at, updated_at, deleted_at
FROM resources
WHERE payload->'metadata'->>'name' = 'e44ec579-9646-549a-b679-db8d19d6da37';
```

4. Copy the `id` value (resource ID): `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`

5. Exit the pod and continue the trace:
```bash
.claude/skills/trace-manifestwork/scripts/trace.sh --resource-id "55c61e54-a3f6-563d-9fec-b1fe297bdfdb"
```

## Complete Trace Results

After completing both steps, you will have:

**Database Information:**
- User Work Name: `e44ec579-9646-549a-b679-db8d19d6da37`
- Resource ID: `55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- Created: 2024-01-15 10:30:00
- Updated: 2024-01-15 10:32:15
- Status: Active (deleted_at is null)

**Cluster Information:**
- AppliedManifestWork: `f1d8a1049b93dffc1929d57a719c3a09a4dcbfe0cd6e42840325be3b2dde73c8-55c61e54-a3f6-563d-9fec-b1fe297bdfdb`
- Applied Manifests:
  - `Deployment/default/maestro-e2e-upgrade-test`