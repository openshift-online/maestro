#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

subscription_id="${subscription_id}"
resource_groups="${resource_groups}"
region="${region}"

work_dir=${REPO_DIR}/_output/performance/aro
cert_dir=${REPO_DIR}/_output/performance/aro/cert

mkdir -p ${cert_dir}

kubectl --kubeconfig $HOME/.kube/svc-cluster.kubeconfig -n maestro get secrets maestro-mqtt -ojsonpath='{.data.ca\.crt}' | base64 -d > ${cert_dir}/ca.crt

vault_name=$(az keyvault list --query "[?starts_with(name, 'maestro-kv')].name"  -g ${resource_groups} --output tsv)

echo "Create a Key Vault Administrator for $vault_name"
az role assignment create --role "Key Vault Administrator" \
    --assignee ${assignee} \
    --scope /subscriptions/${subscription_id}/resourceGroups/${resource_groups}/providers/Microsoft.KeyVault/vaults/${vault_name}
