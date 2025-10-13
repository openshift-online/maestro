# Query Resource

## Prepare

- In ARO-HCP INT environment, run following commands to access Maestro database on service cluster

```sh
kubectl -n maestro scale deployment postgres-breakglass --replicas 1
```

After `postgres-breakglass` is running, run

```sh
kubectl -n maestro exec -it $(kubectl -n maestro get pods -l app=postgres-breakglass -o jsonpath='{.items[0].metadata.name}') -- /bin/bash
```

then run `connect` in the `postgres-breakglass` pod

- If the Maestro database was deployed on the service cluster, run following commands

```sh
kubectl -n maestro exec -it $(kubectl -n maestro get pods -l name=maestro-db -o jsonpath='{.items[0].metadata.name}') -- /bin/bash
```

then in the database pod, run `psql -U maestro -d maestro`

## Query the Maestro resource bundle ID via Clusters Service work name

```sql
SELECT id FROM resources WHERE payload->'metadata'->>'name' = '<work-name>';
```

If you only have the name of the manifest wrapped by the work, run following command on the management cluster to query the work name

```sh
# manifest_kind: the kind of manifest wrapped by the work, e.g. managedclusters, manifestworks, etc.
# manifest_namespace: (optional) the namespace of manifest wrapped by the work, e.g. local-cluster
# manifest_name: the name of manifest wrapped by the work
manifest_kind="<manifest_kind>" manifest_namespace="<manifest_namespace>" manifest_name="<manifest_name>" scripts/trace_work_manifests.sh
```

## Query the work name via Maestro resource bundle ID

```sql
SELECT payload->'metadata'->>'name' AS name FROM resources WHERE id = '<resource-name>';
```
