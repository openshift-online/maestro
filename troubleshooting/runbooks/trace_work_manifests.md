# Trace the manifests of the work

After you create, update, or delete a `ManifestWork` via the Maestro gRPC client, the Maestro server synchronizes the change to the Maestro agent. The agent then applies the manifests contained in the `ManifestWork` to its cluster.

Run the following command to check:

- The manifests within the `ManifestWork`
- The corresponding `AppliedManifestWork`

```sh
export KUBECONFIG=<your_management_cluster_kubeconfig>
work_name="<work_name>" troubleshooting/scripts/trace_work_manifests.sh
```

If you only have the name of the manifest wrapped by the work, run following command

```sh
export KUBECONFIG=<your_management_cluster_kubeconfig>

# manifest_kind: the kind of manifest wrapped by the work, e.g. managedclusters, manifestworks, etc.
# manifest_namespace: (optional) the namespace of manifest wrapped by the work, e.g. local-cluster
# manifest_name: the name of manifest wrapped by the work
manifest_kind="<manifest_kind>" manifest_namespace="<manifest_namespace>" manifest_name="<manifest_name>" scripts/trace_work_manifests.sh
```
