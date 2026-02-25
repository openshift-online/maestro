# Consumer Commands

Consumers represent target clusters that receive resource bundles from Maestro. The `maestro consumer` command group provides full CRUD operations (create, get, list, update, delete) via the Maestro REST API.

## Table of Contents

- [Synopsis](#synopsis)
- [Commands](#commands)
  - [list](#list)
  - [get](#get)
  - [create](#create)
  - [update](#update)
  - [delete](#delete)
- [Examples](#examples)

## Synopsis

```bash
maestro consumer [command] [flags]
```

### Global Flags

All consumer commands support these flags:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--rest-url` | `MAESTRO_REST_URL` | `https://127.0.0.1:30080` | Maestro REST API base URL |
| `--insecure-skip-verify` | `MAESTRO_REST_INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--timeout` | `MAESTRO_REST_TIMEOUT` | `30s` | HTTP client timeout |

### Configuration Examples

```bash
# Using environment variables
export MAESTRO_REST_URL="https://maestro.example.com:8000"
maestro consumer list

# Using command-line flags
maestro consumer list --rest-url https://maestro.example.com:8000
```

## Commands

### list

List consumers with optional filtering and pagination.

#### Usage

```bash
maestro consumer list [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--page` | int | `1` | Page number |
| `--size` | int | `100` | Page size |
| `--search` | string | - | Search filter (SQL-like syntax) |
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# List all consumers with default pagination
maestro consumer list

# List consumers with custom page size
maestro consumer list --page 1 --size 50

# Search for consumers with name pattern
maestro consumer list --search "name like 'prod%'"

# Search for consumers with specific labels (requires label support in backend)
maestro consumer list --search "name like 'cluster%'"

# Output as JSON
maestro consumer list --output json
```

#### Output Example (Table)

```
ID                          NAME                LABELS                          CREATED AT
2faPrp3ZoCMkzdHnBBWd9wqwVXd  prod-cluster-01     env=production,region=us-east   2024-01-15 10:30:00
2faPrp3ZoCMkzdHnBBWd9wqwVXe  prod-cluster-02     env=production,region=us-west   2024-01-15 10:35:00
2faPrp3ZoCMkzdHnBBWd9wqwVXf  dev-cluster-01      env=development                 2024-01-15 11:00:00
```

---

### get

Get a single consumer by its ID.

#### Usage

```bash
maestro consumer get <id> [flags]
```

#### Arguments

- `<id>` - Consumer ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# Get consumer by ID
maestro consumer get 2faPrp3ZoCMkzdHnBBWd9wqwVXd

# Get consumer as JSON
maestro consumer get 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json
```

#### Output Example (Table)

```
ID:           2faPrp3ZoCMkzdHnBBWd9wqwVXd
Name:         prod-cluster-01
Labels:       env=production, region=us-east-1, tier=premium
Created At:   2024-01-15 10:30:00
Updated At:   2024-01-15 14:20:00
```

---

### create

Create a new consumer with the specified name and optional labels.

#### Usage

```bash
maestro consumer create <name> [flags]
```

#### Arguments

- `<name>` - Consumer name (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--label` | strings | - | Labels in `key=value` format (can be specified multiple times) |
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# Create a consumer with just a name
maestro consumer create prod-cluster-01

# Create a consumer with labels
maestro consumer create prod-cluster-01 \
  --label env=production \
  --label region=us-east-1

# Create a consumer with multiple labels
maestro consumer create dev-cluster-01 \
  --label env=development \
  --label team=platform \
  --label owner=john.doe@example.com

# Create and output as JSON
maestro consumer create staging-cluster-01 \
  --label env=staging \
  --output json
```

#### Label Format

Labels must be in `key=value` format:
- Keys can contain alphanumeric characters, hyphens, underscores, and dots
- Values can be any string
- Multiple labels can be specified using multiple `--label` flags

#### Output Example

```
Consumer created successfully:
ID:           2faPrp3ZoCMkzdHnBBWd9wqwVXd
Name:         prod-cluster-01
Labels:       env=production, region=us-east-1
Created At:   2024-01-15 10:30:00
```

---

### update

Update a consumer's labels.

#### Usage

```bash
maestro consumer update <id> [flags]
```

#### Arguments

- `<id>` - Consumer ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--label` | strings | - | Labels to add/update in `key=value` format |
| `--remove-label` | strings | - | Label keys to remove |
| `-o, --output` | string | `table` | Output format: `json` or `table` |

#### Examples

```bash
# Add or update a single label
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd --label tier=premium

# Add or update multiple labels
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd \
  --label env=production \
  --label tier=gold \
  --label zone=az1

# Remove a label
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd --remove-label deprecated

# Add and remove labels in the same command
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd \
  --label tier=silver \
  --remove-label old-tier

# Update and output as JSON
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd \
  --label status=active \
  --output json
```

#### Behavior

- Labels are merged with existing labels
- If a label key already exists, its value is updated
- Removed labels are deleted from the consumer
- At least one `--label` or `--remove-label` must be specified
- The consumer name cannot be updated

#### Output Example

```
Consumer updated successfully:
ID:           2faPrp3ZoCMkzdHnBBWd9wqwVXd
Name:         prod-cluster-01
Labels:       env=production, region=us-east-1, tier=premium
Updated At:   2024-01-15 14:20:00
```

---

### delete

Delete a consumer by its ID.

#### Usage

```bash
maestro consumer delete <id> [flags]
```

#### Arguments

- `<id>` - Consumer ID (required)

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-y, --yes` | bool | `false` | Skip confirmation prompt |

#### Examples

```bash
# Delete a consumer
maestro consumer delete 2faPrp3ZoCMkzdHnBBWd9wqwVXd
```

#### Important Notes

- **A consumer cannot be deleted if it has existing resource bundles**
- You must delete all resource bundles associated with the consumer first
- Deletion is permanent and cannot be undone

#### Output Example

```
Consumer 2faPrp3ZoCMkzdHnBBWd9wqwVXd deleted successfully
```

---

## Examples

### Complete Workflow: Managing a Consumer Lifecycle

```bash
# 1. Create a new consumer
maestro consumer create prod-cluster-01 \
  --label env=production \
  --label region=us-east-1 \
  --label team=platform

# Output:
# Consumer created successfully:
# ID:           2faPrp3ZoCMkzdHnBBWd9wqwVXd
# Name:         prod-cluster-01
# ...

# 2. List consumers to verify
maestro consumer list --search "name like 'prod%'"

# 3. Get the consumer details
maestro consumer get 2faPrp3ZoCMkzdHnBBWd9wqwVXd

# 4. Update consumer labels
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd \
  --label tier=premium \
  --label sla=99.99

# 5. Update again to remove a label
maestro consumer update 2faPrp3ZoCMkzdHnBBWd9wqwVXd \
  --remove-label team

# 6. Export consumer details as JSON
maestro consumer get 2faPrp3ZoCMkzdHnBBWd9wqwVXd --output json > consumer-backup.json

# 7. List all resource bundles for this consumer
maestro resourcebundle list --search "consumer_name='prod-cluster-01'"

# 8. Delete all resource bundles (if any) before deleting consumer
# maestro resourcebundle delete <bundle-id> --yes

# 9. Delete the consumer
maestro consumer delete 2faPrp3ZoCMkzdHnBBWd9wqwVXd --yes
```

### Batch Operations

```bash
# List all production consumers
maestro consumer list --search "name like 'prod%'" --output json | \
  jq -r '.items[].id'

# Update all production consumers with a new label (requires scripting)
for id in $(maestro consumer list --search "name like 'prod%'" --output json | jq -r '.items[].id'); do
  maestro consumer update "$id" --label updated=2024-01-15
done
```

## See Also

- [ResourceBundle Commands](resourcebundle.md)
- [CLI Overview](README.md)
- [Maestro Architecture](../maestro.md)
