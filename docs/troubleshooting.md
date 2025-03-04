# Troubleshooting

## Update Maestro Log Level at Runtime

To aid in troubleshooting, you may need detailed logs from Maestro. Currently, the supported log levels are debug, info, warn, and error, with info as the default. For the complete list of available log levels, refer to [zap log levels](https://github.com/uber-go/zap/blob/master/level.go#L30-L49).

To adjust the log level, create or update the configmap named in `maestro-logging-config ` in maestro namespace. This change will dynamically modify the log level for Maestro without requiring a restart.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: maestro-logging-config
  namespace: maestro
data:
  config.yaml: |
    log_level: debug
```
