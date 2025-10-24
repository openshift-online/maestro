#!/bin/bash

namespace=${namespace:-"maestro"}
label=${label:-"app=maestro"}
container=${container:-"service"}

log_dir=${logs_dir:-"$HOME/maestro-logs"}
mkdir -p "$log_dir"

pods=$(kubectl -n "$namespace" get pods -l "$label" -o jsonpath='{.items[*].metadata.name}')

if [ -z "$pods" ]; then
  echo "No pods found with label $label in namespace $namespace"
  exit 1
fi

echo "Fetching logs from pods: $pods"

for pod in $pods; do
  LOG_FILE="$log_dir/${pod}.log"
  echo "Saving log for pod: $pod ..."
  kubectl -n "$namespace" -c "$container" logs "$pod" > "$LOG_FILE" 2>&1
  if [ $? -eq 0 ]; then
    echo "Saved to $LOG_FILE"
  else
    echo "Failed to get logs for $pod"
  fi
done

echo "All logs saved in directory: $log_dir"
