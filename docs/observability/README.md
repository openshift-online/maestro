# Observability for development environment

## Tracing

The maestro server is instrumented with the OpenTelemetry SDK but by default, tracing capability is not enabled. There's no official backend configured to collect and visualize the traces.

For development environment, tracing can be abled and tracing information can be visualized using Jaeger instance.

### Deploy Jaeger all-in-one instance

A [Jaeger](https://www.jaegertracing.io/) instance with in-memory storage to store and visualize the traces received from the maestro server.

#### Install

```
make deploy
```

After installation, the `jaeger` service becomes available in the `observability` namespace. We can access the UI using `kubectl port-forward`:

```
kubectl port-forward -n observability svc/jaeger 16686:16686
```

Open http://localhost:16686 in your browser to access the Jaeger UI.
The `observability` namespace contains a second service named `ingest` which accepts otlp via gRPC and HTTP.

#### Configure maestro server observability

Run the following commands:

```
make patch-maestro-server
```

The export of the trace information is configured via environment variables. Existing deployments are patched as follows:

```diff
+        env:
+        - name: OTEL_EXPORTER_OTLP_ENDPOINT
+          value: https://ingest.observability:4318
+        - name: OTEL_TRACES_EXPORTER
+          value: otlp
```
