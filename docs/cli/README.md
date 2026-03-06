# Maestro CLI Documentation

The Maestro CLI provides command-line tools for running the Maestro server and managing Maestro resources including consumers (target clusters) and resource bundles (Kubernetes manifests).

## Table of Contents

- [Installation](#installation)
- [Available Commands](#available-commands)

## Installation

### From Source

Build and install the Maestro CLI from source:

```bash
# Clone the repository
git clone https://github.com/openshift-online/maestro.git
cd maestro

# Build the binary
make binary

# Install to GOPATH/bin
make install

# Or use the binary directly
./maestro --help
```

### Verify Installation

```bash
maestro --help
```

## Available Commands

The Maestro CLI provides the following command groups:

### Server Command

Run the Maestro server with configured database and message broker.

- [`server`](server.md) - Start the Maestro server

See [Server Command](server.md) for detailed documentation.

### Consumer Commands

Manage consumers (target clusters) that receive resource bundles from Maestro.

- [`consumer list`](consumer.md#list) - List consumers
- [`consumer get`](consumer.md#get) - Get a consumer by ID
- [`consumer create`](consumer.md#create) - Create a new consumer
- [`consumer update`](consumer.md#update) - Update a consumer
- [`consumer delete`](consumer.md#delete) - Delete a consumer

See [Consumer Commands](consumer.md) for detailed documentation.

### ResourceBundle Commands

Manage resource bundles (collections of Kubernetes manifests).

- [`resourcebundle list`](resourcebundle.md#list) - List resource bundles
- [`resourcebundle get`](resourcebundle.md#get) - Get a resource bundle by ID
- [`resourcebundle apply`](resourcebundle.md#apply) - Create or update a resource bundle
- [`resourcebundle delete`](resourcebundle.md#delete) - Delete a resource bundle
- [`resourcebundle status`](resourcebundle.md#status) - Get resource bundle status

See [ResourceBundle Commands](resourcebundle.md) for detailed documentation.

## Additional Resources

- [Server Command Reference](server.md)
- [Consumer Commands Reference](consumer.md)
- [ResourceBundle Commands Reference](resourcebundle.md)
- [Maestro Architecture](../maestro.md)
- [Maestro Troubleshooting](../troubleshooting.md)
