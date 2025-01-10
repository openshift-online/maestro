#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

counts=${counts:-1}
db_user=${db_user:-"maestro-server"}

${REPO_DIR}/test/performance/hack/aro-hcp/create-clusters.sh

sleep 5

db_pod_name=$(kubectl -n maestro get pods -l name=maestro-db -ojsonpath='{.items[0].metadata.name}')
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U ${db_user} -c 'select * from consumers'

${REPO_DIR}/test/performance/hack/aro-hcp/prepare.kv.sh

sleep 5

# prepare certs for consumers
i=0
while ((i<counts))
do
    i=$(($i + 1))
    (index=$i ${REPO_DIR}/test/performance/hack/aro-hcp/prepare.kv.certs.sh)
done
