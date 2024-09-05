package watcher

// Conditions.
const (
	// HostedClusterAvailable indicates whether the HostedCluster has a healthy
	// control plane.
	// When this is false for too long and there's no clear indication in the "Reason", please check the remaining more granular conditions.
	HostedClusterAvailable = "Available"
	// HostedClusterProgressing indicates whether the HostedCluster is attempting
	// an initial deployment or upgrade.
	// When this is false for too long and there's no clear indication in the "Reason", please check the remaining more granular conditions.
	HostedClusterProgressing = "Progressing"
	// HostedClusterDegraded indicates whether the HostedCluster is encountering
	// an error that may require user intervention to resolve.
	HostedClusterDegraded = "Degraded"

	// Bubble up from HCP.

	// InfrastructureReady bubbles up the same condition from HCP. It signals if the infrastructure for a control plane to be operational,
	// e.g. load balancers were created successfully.
	// A failure here may require external user intervention to resolve. E.g. hitting quotas on the cloud provider.
	InfrastructureReady = "InfrastructureReady"
	// KubeAPIServerAvailable bubbles up the same condition from HCP. It signals if the kube API server is available.
	// A failure here often means a software bug or a non-stable cluster.
	KubeAPIServerAvailable = "KubeAPIServerAvailable"
	// EtcdAvailable bubbles up the same condition from HCP. It signals if etcd is available.
	// A failure here often means a software bug or a non-stable cluster.
	EtcdAvailable = "EtcdAvailable"
	// ValidHostedControlPlaneConfiguration bubbles up the same condition from HCP. It signals if the hostedControlPlane input is valid and
	// supported by the underlying management cluster.
	// A failure here is unlikely to resolve without the changing user input.
	ValidHostedControlPlaneConfiguration = "ValidHostedControlPlaneConfiguration"
	// CloudResourcesDestroyed bubbles up the same condition from HCP. It signals if the cloud provider infrastructure created by Kubernetes
	// in the consumer cloud provider account was destroyed.
	// A failure here may require external user intervention to resolve. E.g. cloud provider perms were corrupted. E.g. the guest cluster was broken
	// and kube resource deletion that affects cloud infra like service type load balancer can't succeed.
	CloudResourcesDestroyed = "CloudResourcesDestroyed"
	// HostedClusterDestroyed indicates that a hosted has finished destroying and that it is waiting for a destroy grace period to go away.
	// The grace period is determined by the hypershift.openshift.io/destroy-grace-period annotation in the HostedCluster if present.
	HostedClusterDestroyed = "HostedClusterDestroyed"
	// ExternalDNSReachable bubbles up the same condition from HCP. It signals if the configured external DNS is reachable.
	// A failure here requires external user intervention to resolve. E.g. changing the external DNS domain or making sure the domain is created
	// and registered correctly.
	ExternalDNSReachable = "ExternalDNSReachable"
	// ValidReleaseInfo bubbles up the same condition from HCP. It indicates if the release contains all the images used by hypershift
	// and reports missing images if any.
	ValidReleaseInfo = "ValidReleaseInfo"

	// Bubble up from HCP which bubbles up from CVO.

	// ClusterVersionSucceeding indicates the current status of the desired release
	// version of the HostedCluster as indicated by the Failing condition in the
	// underlying cluster's ClusterVersion.
	ClusterVersionSucceeding = "ClusterVersionSucceeding"
	// ClusterVersionUpgradeable indicates the Upgradeable condition in the
	// underlying cluster's ClusterVersion.
	ClusterVersionUpgradeable = "ClusterVersionUpgradeable"
	// ClusterVersionFailing bubbles up Failing from the CVO.
	ClusterVersionFailing = "ClusterVersionFailing"
	// ClusterVersionProgressing bubbles up configv1.OperatorProgressing from the CVO.
	ClusterVersionProgressing = "ClusterVersionProgressing"
	// ClusterVersionAvailable bubbles up Failing configv1.OperatorAvailable from the CVO.
	ClusterVersionAvailable = "ClusterVersionAvailable"
	// ClusterVersionReleaseAccepted bubbles up Failing ReleaseAccepted from the CVO.
	ClusterVersionReleaseAccepted = "ClusterVersionReleaseAccepted"

	// UnmanagedEtcdAvailable indicates whether a user-managed etcd cluster is
	// healthy.
	UnmanagedEtcdAvailable = "UnmanagedEtcdAvailable"

	// IgnitionEndpointAvailable indicates whether the ignition server for the
	// HostedCluster is available to handle ignition requests.
	// A failure here often means a software bug or a non-stable cluster.
	IgnitionEndpointAvailable = "IgnitionEndpointAvailable"

	// IgnitionServerValidReleaseInfo indicates if the release contains all the images used by the local ignition provider
	// and reports missing images if any.
	IgnitionServerValidReleaseInfo = "IgnitionServerValidReleaseInfo"

	// ValidHostedClusterConfiguration signals if the hostedCluster input is valid and
	// supported by the underlying management cluster.
	// A failure here is unlikely to resolve without the changing user input.
	ValidHostedClusterConfiguration = "ValidConfiguration"

	// SupportedHostedCluster indicates whether a HostedCluster is supported by
	// the current configuration of the hypershift-operator.
	// e.g. If HostedCluster requests endpointAcess Private but the hypershift-operator
	// is running on a management cluster outside AWS or is not configured with AWS
	// credentials, the HostedCluster is not supported.
	// A failure here is unlikely to resolve without the changing user input.
	SupportedHostedCluster = "SupportedHostedCluster"

	// ValidOIDCConfiguration indicates if an AWS cluster's OIDC condition is
	// detected as invalid.
	// A failure here may require external user intervention to resolve. E.g. oidc was deleted out of band.
	ValidOIDCConfiguration = "ValidOIDCConfiguration"

	// ValidReleaseImage indicates if the release image set in the spec is valid
	// for the HostedCluster. For example, this can be set false if the
	// HostedCluster itself attempts an unsupported version before 4.9 or an
	// unsupported upgrade e.g y-stream upgrade before 4.11.
	// A failure here is unlikely to resolve without the changing user input.
	ValidReleaseImage = "ValidReleaseImage"

	// ValidKubeVirtInfraNetworkMTU indicates if the MTU configured on an infra cluster
	// hosting a guest cluster utilizing kubevirt platform is a sufficient value that will avoid
	// performance degradation due to fragmentation of the double encapsulation in ovn-kubernetes
	ValidKubeVirtInfraNetworkMTU = "ValidKubeVirtInfraNetworkMTU"

	// ValidAWSIdentityProvider indicates if the Identity Provider referenced
	// in the cloud credentials is healthy. E.g. for AWS the idp ARN is referenced in the iam roles.
	// 		"Version": "2012-10-17",
	//		"Statement": [
	//			{
	//				"Effect": "Allow",
	//				"Principal": {
	//					"Federated": "{{ .ProviderARN }}"
	//				},
	//					"Action": "sts:AssumeRoleWithWebIdentity",
	//				"Condition": {
	//					"StringEquals": {
	//						"{{ .ProviderName }}:sub": {{ .ServiceAccounts }}
	//					}
	//				}
	//			}
	//		]
	//
	// A failure here may require external user intervention to resolve.
	ValidAWSIdentityProvider = "ValidAWSIdentityProvider"

	// ValidAWSKMSConfig indicates whether the AWS KMS role and encryption key are valid and operational
	// A failure here indicates that the role or the key are invalid, or the role doesn't have access to use the key.
	ValidAWSKMSConfig = "ValidAWSKMSConfig"

	// ValidAzureKMSConfig indicates whether the given KMS input for the Azure platform is valid and operational
	// A failure here indicates that the input is invalid, or permissions are missing to use the encryption key.
	ValidAzureKMSConfig = "ValidAzureKMSConfig"

	// AWSDefaultSecurityGroupCreated indicates whether the default security group
	// for AWS workers has been created.
	// A failure here indicates that NodePools without a security group will be
	// blocked from creating machines.
	AWSDefaultSecurityGroupCreated = "AWSDefaultSecurityGroupCreated"

	// AWSDefaultSecurityGroupDeleted indicates whether the default security group
	// for AWS workers has been deleted.
	// A failure here indicates that the Security Group has some dependencies that
	// there are still pending cloud resources to be deleted that are using that SG.
	AWSDefaultSecurityGroupDeleted = "AWSDefaultSecurityGroupDeleted"

	// PlatformCredentialsFound indicates that credentials required for the
	// desired platform are valid.
	// A failure here is unlikely to resolve without the changing user input.
	PlatformCredentialsFound = "PlatformCredentialsFound"

	// ReconciliationActive indicates if reconciliation of the HostedCluster is
	// active or paused hostedCluster.spec.pausedUntil.
	ReconciliationActive = "ReconciliationActive"
	// ReconciliationSucceeded indicates if the HostedCluster reconciliation
	// succeeded.
	// A failure here often means a software bug or a non-stable cluster.
	ReconciliationSucceeded = "ReconciliationSucceeded"

	// ClusterSizeComputed indicates that a t-shirt size was computed for this HostedCluster.
	// The last transition time for this condition is used to manage how quickly transitions occur.
	ClusterSizeComputed = "ClusterSizeComputed"
	// ClusterSizeTransitionPending indicates that a t-shirt size transition is pending, but has
	// not been applied yet. This may either be due to transition delays on the cluster itself
	// or from management-cluster-wide limits to transition throughput.
	ClusterSizeTransitionPending = "ClusterSizeTransitionPending"
	// ClusterSizeTransitionRequired exposes the next t-shirt size that the cluster will transition to.
	ClusterSizeTransitionRequired = "ClusterSizeTransitionRequired"
)

// Reasons.
const (
	StatusUnknownReason         = "StatusUnknown"
	AsExpectedReason            = "AsExpected"
	NotFoundReason              = "NotFound"
	WaitingForAvailableReason   = "WaitingForAvailable"
	SecretNotFoundReason        = "SecretNotFound"
	WaitingForGracePeriodReason = "WaitingForGracePeriod"
	BlockedReason               = "Blocked"

	InfraStatusFailureReason           = "InfraStatusFailure"
	WaitingOnInfrastructureReadyReason = "WaitingOnInfrastructureReady"

	EtcdQuorumAvailableReason     = "QuorumAvailable"
	EtcdWaitingForQuorumReason    = "EtcdWaitingForQuorum"
	EtcdStatefulSetNotFoundReason = "StatefulSetNotFound"

	UnmanagedEtcdMisconfiguredReason = "UnmanagedEtcdMisconfigured"
	UnmanagedEtcdAsExpected          = "UnmanagedEtcdAsExpected"

	FromClusterVersionReason = "FromClusterVersion"

	InvalidConfigurationReason            = "InvalidConfiguration"
	KubeconfigWaitingForCreateReason      = "KubeconfigWaitingForCreate"
	UnsupportedHostedClusterReason        = "UnsupportedHostedCluster"
	InsufficientClusterCapabilitiesReason = "InsufficientClusterCapabilities"
	OIDCConfigurationInvalidReason        = "OIDCConfigurationInvalid"
	PlatformCredentialsNotFoundReason     = "PlatformCredentialsNotFound"
	InvalidImageReason                    = "InvalidImage"
	InvalidIdentityProvider               = "InvalidIdentityProvider"

	InvalidIAMRoleReason = "InvalidIAMRole"

	InvalidAzureCredentialsReason = "InvalidAzureCredentials"
	AzureErrorReason              = "AzureError"

	ExternalDNSHostNotReachableReason = "ExternalDNSHostNotReachable"

	KASLoadBalancerNotReachableReason = "KASLoadBalancerNotReachable"

	MissingReleaseImagesReason = "MissingReleaseImages"

	ReconciliationPausedConditionReason             = "ReconciliationPaused"
	ReconciliationInvalidPausedUntilConditionReason = "InvalidPausedUntilValue"

	KubeVirtSuboptimalMTUReason = "KubeVirtSuboptimalMTUDetected"
)

// Messages.
const (
	// AllIsWellMessage is standard message.
	AllIsWellMessage = "All is well"
)

// Conditions
const (
	// NodePoolValidGeneratedPayload signals if the ignition sever generated an ignition payload successfully for Nodes in that pool.
	// A failure here often means a software bug or a non-stable cluster.
	NodePoolValidGeneratedPayload = "ValidGeneratedPayload"
	// NodePoolValidPlatformImageType signals if an OS image e.g. an AMI was found successfully based on the consumer input e.g. releaseImage.
	// If the image is direct user input then this condition is meaningless.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolValidPlatformImageType = "ValidPlatformImage"
	// NodePoolValidReleaseImage signals if the input in nodePool.spec.release.image is valid.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolValidReleaseImage = "ValidReleaseImage"
	// NodePoolValidMachineConfig signals if the content within nodePool.spec.config is valid.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolValidMachineConfig = "ValidMachineConfig"
	// NodePoolValidTuningConfig signals if the content within nodePool.spec.tuningConfig is valid.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolValidTuningConfig = "ValidTuningConfig"

	// NodePoolUpdateManagementEnabled signals if the nodePool.spec.management input is valid.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolUpdateManagementEnabled = "UpdateManagementEnabled"
	// NodePoolAutoscalingEnabled signals if nodePool.spec.replicas and nodePool.spec.AutoScaling input is valid.
	// A failure here is unlikely to resolve without the changing user input.
	NodePoolAutoscalingEnabled = "AutoscalingEnabled"
	// NodePoolAutorepairEnabled signals if MachineHealthChecks resources were created successfully.
	// A failure here often means a software bug or a non-stable cluster.
	NodePoolAutorepairEnabled = "AutorepairEnabled"

	// NodePoolUpdatingVersion signals if a version update is currently happening in NodePool.
	NodePoolUpdatingVersion = "UpdatingVersion"
	// NodePoolUpdatingConfig signals if a config update is currently happening in NodePool.
	NodePoolUpdatingConfig = "UpdatingConfig"
	// NodePoolUpdatingPlatformMachineTemplate signals if a platform machine template update is currently happening in NodePool.
	NodePoolUpdatingPlatformMachineTemplate = "UpdatingPlatformMachineTemplate"
	// NodePoolReady bubbles up CAPI MachineDeployment/MachineSet Ready condition.
	// This is true when all replicas are ready Nodes.
	// When this is false for too long, NodePoolAllMachinesReady and NodePoolAllNodesHealthy might provide more context.
	NodePoolReady = "Ready"
	// NodePoolAllMachinesReady bubbles up and aggregates CAPI Machine Ready condition.
	// It signals when the infrastructure for a Machine resource was created successfully.
	// https://github.com/kubernetes-sigs/cluster-api/blob/main/api/v1beta1/condition_consts.go
	// A failure here may require external user intervention to resolve. E.g. hitting quotas on the cloud provider.
	NodePoolAllMachinesReady = "AllMachinesReady"
	// NodePoolAllNodesHealthy bubbles up and aggregates CAPI NodeHealthy condition.
	// It signals when the Node for a Machine resource is healthy.
	// https://github.com/kubernetes-sigs/cluster-api/blob/main/api/v1beta1/condition_consts.go
	// A failure here often means a software bug or a non-stable cluster.
	NodePoolAllNodesHealthy = "AllNodesHealthy"

	// NodePoolReconciliationActive signals the state of nodePool.spec.pausedUntil.
	NodePoolReconciliationActive = "ReconciliationActive"

	// NodePoolReachedIgnitionEndpoint signals if at least an instance was able to reach the ignition endpoint to get the payload.
	// When this is false for too long it may require external user intervention to resolve. E.g. Enable AWS security groups to enable networking access.
	NodePoolReachedIgnitionEndpoint = "ReachedIgnitionEndpoint"

	// NodePoolAWSSecurityGroupAvailable signals whether the NodePool has an available security group to use.
	// If the security group is specified for the NodePool, this condition is always true. If no security group is specified
	// for the NodePool, the status of this condition depends on the availability of the default security group in the HostedCluster.
	NodePoolAWSSecurityGroupAvailable = "AWSSecurityGroupAvailable"

	// NodePoolValidMachineTemplate signal that the machine template created by the node pool is valid
	NodePoolValidMachineTemplate = "ValidMachineTemplate"

	// NodePoolClusterNetworkCIDRConflictType signals if a NodePool's machine objects are colliding with the
	// cluster network's CIDR range. This can indicate why some network functionality might be degraded.
	NodePoolClusterNetworkCIDRConflictType = "ClusterNetworkCIDRConflict"
)

// Reasons
const (
	NodePoolValidationFailedReason        = "ValidationFailed"
	NodePoolInplaceUpgradeFailedReason    = "InplaceUpgradeFailed"
	NodePoolNotFoundReason                = "NotFound"
	NodePoolFailedToGetReason             = "FailedToGet"
	IgnitionEndpointMissingReason         = "IgnitionEndpointMissing"
	IgnitionCACertMissingReason           = "IgnitionCACertMissing"
	IgnitionNotReached                    = "ignitionNotReached"
	DefaultAWSSecurityGroupNotReadyReason = "DefaultSGNotReady"
	NodePoolValidArchPlatform             = "ValidArchPlatform"
	NodePoolInvalidArchPlatform           = "InvalidArchPlatform"
	InvalidKubevirtMachineTemplate        = "InvalidKubevirtMachineTemplate"
	CIDRConflictReason                    = "CIDRConflict"
)
