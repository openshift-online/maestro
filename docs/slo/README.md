## Validating SLO Manifests with oslo Before Commit

🔧 1. Install the oslo CLI

You can install oslo using go install:

```shell
go install github.com/OpenSLO/oslo/cmd/oslo@latest
```

> 📌 Make sure your `$GOPATH/bin` is in your `PATH`. You can verify the installation with:

```shell
oslo --help
```

📄 2. Validate SLO Manifests:

Run the following command to validate each manifest file:

```shell
# oslo validate -f docs/slo/maestro-server-slo.yaml
Valid!
# oslo validate -f docs/slo/maestro-agent-slo.yaml
Valid!
```

You can also validate all .yaml files in a folder:

```shell
find docs/slo -name "*.yaml" -exec oslo validate -f {} \;
```
