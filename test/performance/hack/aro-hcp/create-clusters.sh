#!/usr/bin/env bash
total=${total:-10}
begin_index=${begin_index:-1}

lastIndex=$(($begin_index + $total - 1))
echo "create clusters from maestro-cluster-$begin_index to maestro-cluster-$lastIndex"

kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: clusters-$begin_index-$lastIndex
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
          - "--cluster-begin-index=$begin_index"
          - "--cluster-counts=$total"
          - "--only-clusters=true"
      restartPolicy: Never
  backoffLimit: 4
EOF
