using '../templates/region.bicep'

// maestro
param maestroKeyVaultName = take('maestro-kv-${uniqueString(currentUserId)}', 24)
param maestroEventGridNamespacesName = take('maestro-eg-${uniqueString(currentUserId)}', 24)
param maestroEventGridMaxClientSessionsPerAuthName = 4

// These parameters are always overriden in the Makefile
param currentUserId = ''
