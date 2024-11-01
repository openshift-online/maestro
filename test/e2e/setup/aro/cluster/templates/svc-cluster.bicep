@description('Azure Region Location')
param location string = resourceGroup().location

@description('Set to true to prevent resources from being pruned after 48 hours')
param persist bool = false

@description('Captures logged in users UID')
param currentUserId string

@description('AKS cluster name')
param aksClusterName string

@description('Disk size for the AKS system nodes')
param aksSystemOsDiskSizeGB int = 32

@description('Disk size for the AKS user nodes')
param aksUserOsDiskSizeGB int = 32

@description('Name of the resource group for the AKS nodes')
param aksNodeResourceGroupName string = '${resourceGroup().name}-aks1'

@description('VNET address prefix')
param vnetAddressPrefix string

@description('Subnet address prefix')
param subnetPrefix string

@description('Specifies the address prefix of the subnet hosting the pods of the AKS cluster.')
param podSubnetPrefix string

@description('Kuberentes version to use with AKS')
param kubernetesVersion string

@description('Istio control plane version to use with AKS')
param istioVersion array

@description('The name of the keyvault for AKS.')
@maxLength(24)
param aksKeyVaultName string

@description('Manage soft delete setting for AKS etcd key-value store')
param aksEtcdKVEnableSoftDelete bool = true

@description('The resourcegroup for regional infrastructure')
param regionalResourceGroup string

@description('The domain to use to use for the maestro certificate. Relevant only for environments where OneCert can be used.')
param maestroCertDomain string

@description('The name of the eventgrid namespace for Maestro.')
param maestroEventGridNamespacesName string

@description('The name of the keyvault for Maestro Eventgrid namespace certificates.')
@maxLength(24)
param maestroKeyVaultName string

@description('The name of the managed identity that will manage certificates in maestros keyvault.')
param maestroKeyVaultCertOfficerMSIName string = '${maestroKeyVaultName}-cert-officer-msi'

@description('Deploy ARO HCP Maestro Postgres if true')
param deployMaestroPostgres bool = false

@description('If true, make the Maestro Postgres instance private')
param maestroPostgresPrivate bool = true

@description('The name of the Postgres server for Maestro')
@maxLength(60)
param maestroPostgresServerName string

@description('The version of the Postgres server for Maestro')
param maestroPostgresServerVersion string

@description('The size of the Postgres server for Maestro')
param maestroPostgresServerStorageSizeGB int

// Tags the resource group
resource subscriptionTags 'Microsoft.Resources/tags@2024-03-01' = {
  name: 'default'
  scope: resourceGroup()
  properties: {
    tags: {
      persist: toLower(string(persist))
      deployedBy: currentUserId
    }
  }
}

module svcCluster '../modules/aks-cluster-base.bicep' = {
  name: 'svc-cluster'
  scope: resourceGroup()
  params: {
    location: location
    persist: persist
    aksClusterName: aksClusterName
    aksNodeResourceGroupName: aksNodeResourceGroupName
    aksEtcdKVEnableSoftDelete: aksEtcdKVEnableSoftDelete
    kubernetesVersion: kubernetesVersion
    deployIstio: false
    istioVersion: istioVersion
    vnetAddressPrefix: vnetAddressPrefix
    subnetPrefix: subnetPrefix
    podSubnetPrefix: podSubnetPrefix
    clusterType: 'svc-cluster'
    systemOsDiskSizeGB: aksSystemOsDiskSizeGB
    userOsDiskSizeGB: aksUserOsDiskSizeGB
    workloadIdentities: items({
      frontend_wi: {
        uamiName: 'frontend'
        namespace: 'aro-hcp'
        serviceAccountName: 'frontend'
      }
      backend_wi: {
        uamiName: 'backend'
        namespace: 'aro-hcp'
        serviceAccountName: 'backend'
      }
      maestro_wi: {
        uamiName: 'maestro-server'
        namespace: 'maestro'
        serviceAccountName: 'maestro'
      }
    })
    aksKeyVaultName: aksKeyVaultName
  }
}

output aksClusterName string = svcCluster.outputs.aksClusterName

//
//   M A E S T R O
//

module maestroServer '../modules/maestro/maestro-server.bicep' = {
  name: 'maestro-server'
  params: {
    maestroInfraResourceGroup: regionalResourceGroup
    maestroEventGridNamespaceName: maestroEventGridNamespacesName
    maestroKeyVaultName: maestroKeyVaultName
    maestroKeyVaultOfficerManagedIdentityName: maestroKeyVaultCertOfficerMSIName
    maestroKeyVaultCertificateDomain: maestroCertDomain
    deployPostgres: deployMaestroPostgres
    postgresServerName: maestroPostgresServerName
    postgresServerVersion: maestroPostgresServerVersion
    postgresServerStorageSizeGB: maestroPostgresServerStorageSizeGB
    privateEndpointSubnetId: svcCluster.outputs.aksNodeSubnetId
    privateEndpointVnetId: svcCluster.outputs.aksVnetId
    postgresServerPrivate: maestroPostgresPrivate
    maestroServerManagedIdentityPrincipalId: filter(
      svcCluster.outputs.userAssignedIdentities,
      id => id.uamiName == 'maestro-server'
    )[0].uamiPrincipalID
    maestroServerManagedIdentityName: filter(
      svcCluster.outputs.userAssignedIdentities,
      id => id.uamiName == 'maestro-server'
    )[0].uamiName
    location: location
  }
}
