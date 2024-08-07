#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../../.." ; pwd -P)"

index=${index:-1}

subscription_id="${subscription_id}"
resource_groups="${resource_groups}"
region="${region}"

work_dir=${REPO_DIR}/_output/performance/aro
cert_dir=${REPO_DIR}/_output/performance/aro/cert

mkdir -p ${cert_dir}

vault_name=$(az keyvault list --query "[?starts_with(name, 'maestro-kv')].name"  -g ${resource_groups} --output tsv)

# prepare consumer client 
consumer_name="maestro-cluster-$index"

echo "Create certificate in $vault_name for $consumer_name"
cat > ${cert_dir}/${consumer_name}.json <<EOF
{
  "issuerParameters": {
    "certificateTransparency": null,
    "name": "Self"
  },
  "keyProperties": {
    "curve": null,
    "exportable": true,
    "keySize": 2048,
    "keyType": "RSA",
    "reuseKey": true
  },
  "lifetimeActions": [
    {
      "action": {
        "actionType": "AutoRenew"
      },
      "trigger": {
        "daysBeforeExpiry": 90
      }
    }
  ],
  "secretProperties": {
    "contentType": "application/x-pkcs12"
  },
  "x509CertificateProperties": {
    "keyUsage": [
      "cRLSign",
      "dataEncipherment",
      "digitalSignature",
      "keyEncipherment",
      "keyAgreement",
      "keyCertSign"
    ],
    "subject": "CN=${consumer_name}",
    "subjectAlternativeNames": {
    "dnsNames": [
      "${consumer_name}.selfsigned.maestro.keyvault.aro-int.azure.com"
    ]
  },
    "validityInMonths": 12
  }
}
EOF

az keyvault certificate create --vault-name ${vault_name} -n ${consumer_name} -p @${cert_dir}/${consumer_name}.json

echo "Get certificate from $vault_name for $consumer_name"
az keyvault certificate list --vault-name ${vault_name}

az keyvault secret download --id https://${vault_name}.vault.azure.net/certificates/${consumer_name} --file ${cert_dir}/${consumer_name}.base64

cat ${cert_dir}/${consumer_name}.base64 | base64 -d > ${cert_dir}/${consumer_name}.pfx

openssl pkcs12 -in ${cert_dir}/${consumer_name}.pfx -passin "pass:" -nocerts -nodes -out ${cert_dir}/${consumer_name}.key
openssl pkcs12 -in ${cert_dir}/${consumer_name}.pfx -passin "pass:" -nokeys -out ${cert_dir}/${consumer_name}.pem

thumbprint=$(openssl x509 -fingerprint -in ${cert_dir}/${consumer_name}.pem | grep "SHA1 Fingerprint" | sed -e 's/SHA1 Fingerprint=//g' | sed -e 's/://g')

event_grid="${resource_groups}-eventgrid"
echo "Prepare client in event grid ${event_grid} for $consumer_name"
az eventgrid namespace client create -g ${resource_groups} \
    --namespace-name ${event_grid} \
    -n ${consumer_name} \
    --authentication-name ${consumer_name}.selfsigned.maestro.keyvault.aro-int.azure.com \
    --attributes "{'role':'consumer','consumer_name':'${consumer_name}'}" \
    --client-certificate-authentication "{validationScheme:ThumbprintMatch,allowed-thumbprints:[$thumbprint]}"

host="${resource_groups}-eventgrid.${region}.ts.eventgrid.azure.net"
src_topic="sources/maestro/consumers/${consumer_name}/sourceevents"
agent_topic="sources/maestro/consumers/${consumer_name}/agentevents"

cat > "${cert_dir}/${consumer_name}-config.yaml" << EOF
brokerHost: "$host:8883"
caFile: ${cert_dir}/ca.crt
clientCertFile: ${cert_dir}/${consumer_name}.pem
clientKeyFile: ${cert_dir}/${consumer_name}.key
topics:
  sourceEvents: $src_topic
  agentEvents: $agent_topic
EOF

echo "The certificates is creaed for $consumer_name"
