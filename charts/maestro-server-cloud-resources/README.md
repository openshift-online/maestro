# maestro-server-cloud-resources

Helm chart that provisions GCP cloud resources required by the Maestro Server
when deploying on Google Cloud Platform (GKE).

## Prerequisites

- **GKE cluster** with [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) enabled
- **[Config Connector](https://cloud.google.com/config-connector/docs/overview)** installed and configured on the cluster
- **[External Secrets Operator (ESO)](https://external-secrets.io/)** installed (for database credential generation)
- **Pub/Sub topics** must already exist in the GCP project (created externally, e.g. by Terraform or manually)

## What This Chart Creates

### Cloud SQL (when `database.enabled: true`)

| Resource | Type | Purpose |
|---|---|---|
| `SQLInstance` | Config Connector | PostgreSQL Cloud SQL instance |
| `SQLDatabase` | Config Connector | Database within the instance |
| `SQLUser` | Config Connector | Database user (password from ESO) |
| `Password` | ESO Generator | Stable random password (never auto-rotates) |
| `ExternalSecret` | ESO | Kubernetes Secret with `db.host`, `db.port`, `db.name`, `db.user`, `db.password` |

### Cloud SQL API (when `database.enabled: true`)

| Resource | Type | Purpose |
|---|---|---|
| `Service` | Config Connector | Enables `sqladmin.googleapis.com` API |

### Pub/Sub (when `pubsub.enabled: true`)

| Resource | Type | Purpose |
|---|---|---|
| `PubSubSubscription` (agentevents) | Config Connector | Server consumes agent status events |
| `PubSubSubscription` (agentbroadcast) | Config Connector | Server consumes agent broadcast events |

### IAM & Workload Identity

| Resource | Type | Purpose |
|---|---|---|
| `IAMServiceAccount` | Config Connector | GCP SA for the Maestro Server |
| `IAMPolicyMember` (WIF) | Config Connector | Allows the k8s SA to impersonate the GCP SA |
| `IAMPolicyMember` (Cloud SQL) | Config Connector | `roles/cloudsql.client` for the Cloud SQL Auth Proxy |
| `IAMPolicyMember` (Pub/Sub) | Config Connector | Publisher/subscriber/viewer grants per topic and subscription |

### Consumer Registration (when `consumerRegistration.enabled: true`)

| Resource | Type | Purpose |
|---|---|---|
| `CronJob` | Kubernetes | Periodically discovers and registers Maestro consumers |
| `ServiceAccount` | Kubernetes | Dedicated SA for the CronJob |
| `IAMPartialPolicy` | Config Connector | `roles/secretmanager.viewer` for consumer discovery via Secret Manager labels |

## Usage

```bash
helm install maestro-server-cloud-resources ./charts/maestro-server-cloud-resources \
  --namespace hyperfleet \
  --set gcp.project=my-gcp-project-id \
  --set gcp.region=us-central1
```

## Values

### Required

| Value | Default | Description |
|---|---|---|
| `gcp.project` | `""` | GCP project ID |
| `gcp.region` | `us-central1` | GCP region |

### Server Identity

| Value | Default | Description |
|---|---|---|
| `serverServiceAccount` | `maestro` | Name of the Maestro Server k8s ServiceAccount (must match `maestro-server` chart) |

### Cloud SQL

| Value | Default | Description |
|---|---|---|
| `database.enabled` | `true` | Enable Cloud SQL resources (instance, database, user, ESO credentials, and `sqladmin.googleapis.com` API) |
| `database.instance.name` | `maestro-db` | Cloud SQL instance name |
| `database.instance.version` | `POSTGRES_15` | PostgreSQL version |
| `database.instance.tier` | `db-custom-1-3840` | Machine type (1 vCPU, 3.75 GB) |
| `database.instance.diskSize` | `10` | Disk size in GB |
| `database.instance.diskType` | `PD_SSD` | Disk type |
| `database.instance.diskAutoresize` | `true` | Enable automatic disk resizing |
| `database.instance.diskAutoresizeLimit` | `100` | Max disk size in GB |
| `database.instance.deletionProtection` | `false` | Set `true` for production |
| `database.instance.ipConfiguration.ipv4Enabled` | `true` | Enable public IP (set `false` for private-only) |
| `database.instance.ipConfiguration.requireSsl` | `false` | Require SSL connections (set `true` for production) |
| `database.instance.backupConfiguration.enabled` | `true` | Enable automated backups |
| `database.instance.backupConfiguration.pointInTimeRecoveryEnabled` | `true` | Enable PITR |
| `database.instance.backupConfiguration.retainedBackups` | `7` | Number of backups to retain |
| `database.instance.maintenanceWindow.day` | `7` | Maintenance day (1=Mon, 7=Sun) |
| `database.instance.maintenanceWindow.hour` | `2` | Maintenance hour (UTC) |
| `database.instance.maintenanceWindow.updateTrack` | `stable` | Update track |
| `database.name` | `maestro` | Database name within the instance |
| `database.user` | `maestro` | Database user name |
| `database.host` | `localhost` | Database host in the credentials secret (localhost for Cloud SQL Auth Proxy sidecar) |
| `database.port` | `5432` | Database port in the credentials secret |
| `database.passwordSecret.name` | `maestro-db` | Name of the Kubernetes Secret for DB credentials |

### Pub/Sub

| Value | Default | Description |
|---|---|---|
| `pubsub.enabled` | `true` | Create Pub/Sub subscriptions and IAM bindings |
| `pubsub.topics.sourceEvents` | `""` | Full GCP resource name (e.g. `projects/my-project/topics/sourceevents`) |
| `pubsub.topics.sourceBroadcast` | `""` | Full GCP resource name (e.g. `projects/my-project/topics/sourcebroadcast`) |
| `pubsub.topics.agentEvents` | `""` | Full GCP resource name (e.g. `projects/my-project/topics/agentevents`) |
| `pubsub.topics.agentBroadcast` | `""` | Full GCP resource name (e.g. `projects/my-project/topics/agentbroadcast`) |
| `pubsub.subscriptions.agentEvents` | `""` | Full GCP resource name (e.g. `projects/my-project/subscriptions/agentevents-server`) |
| `pubsub.subscriptions.agentBroadcast` | `""` | Full GCP resource name (e.g. `projects/my-project/subscriptions/agentbroadcast-server`) |

### Consumer Registration

| Value | Default | Description |
|---|---|---|
| `consumerRegistration.enabled` | `true` | Deploy the consumer registration CronJob |
| `consumerRegistration.schedule` | `*/5 * * * *` | CronJob schedule |
| `consumerRegistration.maestroUrl` | `""` | Maestro Server URL (e.g. `http://maestro.hyperfleet:8000`) |
| `consumerRegistration.serviceAccount` | `maestro-consumer-registration` | ServiceAccount for the CronJob |
| `consumerRegistration.image` | `google/cloud-sdk:slim` | Container image for the CronJob |
