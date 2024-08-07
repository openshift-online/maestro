#!/usr/bin/env bash
works=${works:-300}

index=${index:-1}

echo "create works for maestro-cluster-$index"

kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: works-$index
  namespace: maestro
spec:
  template:
    spec:
      containers:
      - name: aro-hcp-clusters
        image: quay.io/skeeey/maestro-perf-tool:aro-hcp
        imagePullPolicy: IfNotPresent
        args:
          - "/maestroperf"
          - "aro-hcp-prepare"
          - "--cluster-begin-index=$index"
          - "--cluster-counts=1"
          - "--work-counts=$works"
      restartPolicy: Never
  backoffLimit: 4
EOF
