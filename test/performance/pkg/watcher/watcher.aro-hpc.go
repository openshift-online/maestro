package watcher

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/test/performance/pkg/util"
)

type AROHCPWatcherOptions struct {
	sync.RWMutex

	SpokeKubeConfigPath string
	Index               int
	Totals              int

	clusterTotals int
	workTotals    int
}

func NewAROHCPWatcherOptions() *AROHCPWatcherOptions {
	return &AROHCPWatcherOptions{
		Index:         1,
		Totals:        1,
		clusterTotals: 0,
		workTotals:    0,
	}
}

func (o *AROHCPWatcherOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SpokeKubeConfigPath, "spoke-kubeconfig", o.SpokeKubeConfigPath, "Location of the Spoke kubeconfig")
	fs.IntVar(&o.Index, "clusters-index", o.Index, "The begin index of clusters")
	fs.IntVar(&o.Totals, "clusters-counts", o.Totals, "The counts of clusters")
}

func (o *AROHCPWatcherOptions) Run(ctx context.Context) error {
	spokeKubeConfig, err := clientcmd.BuildConfigFromFlags("", o.SpokeKubeConfigPath)
	if err != nil {
		return err
	}

	clusterClient, err := clusterclientset.NewForConfig(spokeKubeConfig)
	if err != nil {
		return err
	}

	workClient, err := workclientset.NewForConfig(spokeKubeConfig)
	if err != nil {
		return err
	}

	clusterWatcher, err := clusterClient.ClusterV1().ManagedClusters().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	go func() {
		klog.Infof("watching clusters ....")

		ch := clusterWatcher.ResultChan()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				switch event.Type {
				case watch.Added:
					startTime := time.Now()
					obj, err := meta.Accessor(event.Object)
					if err != nil {
						klog.Fatal(err)
					}

					cluster, err := clusterClient.ClusterV1().ManagedClusters().Get(ctx, obj.GetName(), metav1.GetOptions{})
					if err != nil {
						klog.Errorf("error to get cluster: %v, %v", obj.GetName(), err)
						continue
					}

					if meta.IsStatusConditionTrue(cluster.Status.Conditions, clusterv1.ManagedClusterConditionHubAccepted) {
						continue
					}

					conditions := []metav1.Condition{}
					meta.SetStatusCondition(&conditions, metav1.Condition{
						Type:    clusterv1.ManagedClusterConditionHubAccepted,
						Status:  metav1.ConditionTrue,
						Reason:  "HubClusterAdminAccepted",
						Message: "Accepted by hub cluster admin",
					})
					meta.SetStatusCondition(&conditions, metav1.Condition{
						Type:    clusterv1.ManagedClusterConditionJoined,
						Status:  metav1.ConditionTrue,
						Reason:  "ManagedClusterJoined",
						Message: "Managed cluster joined",
					})
					meta.SetStatusCondition(&conditions, metav1.Condition{
						Type:    clusterv1.ManagedClusterConditionAvailable,
						Status:  metav1.ConditionTrue,
						Reason:  "ManagedClusterAvailable",
						Message: "Managed cluster is available",
					})
					meta.SetStatusCondition(&conditions, metav1.Condition{
						Type:    clusterv1.ManagedClusterConditionClockSynced,
						Status:  metav1.ConditionTrue,
						Reason:  "ManagedClusterClockSynced",
						Message: "The clock of the managed cluster is synced with the hub.",
					})

					cluster.Status.Conditions = conditions
					cluster.Status.Capacity = clusterv1.ResourceList{
						clusterv1.ResourceCPU:    *resource.NewQuantity(int64(32), resource.DecimalExponent),
						clusterv1.ResourceMemory: *resource.NewQuantity(int64(1024*1024*64), resource.BinarySI),
					}
					cluster.Status.Allocatable = clusterv1.ResourceList{
						clusterv1.ResourceCPU:    *resource.NewQuantity(int64(16), resource.DecimalExponent),
						clusterv1.ResourceMemory: *resource.NewQuantity(int64(1024*1024*32), resource.BinarySI),
					}
					cluster.Status.Version = clusterv1.ManagedClusterVersion{
						Kubernetes: "1.22",
					}

					_, err = clusterClient.ClusterV1().ManagedClusters().UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
					if err != nil {
						klog.Errorf("error to update cluster: %v, %v", obj.GetName(), err)
						continue
					}

					o.clusterTotals = o.clusterTotals + 1
					klog.Infof("cluster %v is updated, created at: %s, watched at: %s, total=%d, time=%dms",
						cluster.CreationTimestamp.UTC().Format(time.RFC3339),
						startTime.UTC().Format(time.RFC3339),
						obj.GetName(), o.clusterTotals,
						util.UsedTime(startTime, time.Millisecond))
				}
			}
		}
	}()

	index := o.Index
	for i := 1; i <= o.Totals; i++ {
		name := util.ClusterName(index)
		klog.Infof("watching works for %s ....", name)
		workWatcher, err := workClient.WorkV1().ManifestWorks(name).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		go func() {
			ch := workWatcher.ResultChan()
			for {
				select {
				case <-ctx.Done():
					return
				case event, ok := <-ch:
					if !ok {
						return
					}
					switch event.Type {
					case watch.Added:
						startTime := time.Now()
						obj, err := meta.Accessor(event.Object)
						if err != nil {
							klog.Fatal(err)
						}

						work, err := workClient.WorkV1().ManifestWorks(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
						if err != nil {
							klog.Errorf("error to get work: %v/%v, %v", obj.GetNamespace(), obj.GetName(), err)
							continue
						}

						conditions := []metav1.Condition{}
						meta.SetStatusCondition(&conditions, metav1.Condition{
							Type:    workv1.ManifestApplied,
							Status:  metav1.ConditionTrue,
							Reason:  "AppliedManifestComplete",
							Message: "Apply manifest complete",
						})
						meta.SetStatusCondition(&conditions, metav1.Condition{
							Type:    workv1.ManifestAvailable,
							Status:  metav1.ConditionTrue,
							Reason:  "ResourceAvailable",
							Message: "Resource is available",
						})
						meta.SetStatusCondition(&conditions, metav1.Condition{
							Type:    "StatusFeedbackSynced",
							Status:  metav1.ConditionTrue,
							Reason:  "StatusFeedbackSynced",
							Message: "",
						})

						work.Status.Conditions = conditions

						var jsonRaw string
						if strings.HasSuffix(obj.GetName(), "namespace") {
							jsonRaw = toNamespaceJsonRaw(obj.GetName())

						}

						if strings.HasSuffix(obj.GetName(), "hypershift") {
							jsonRaw = toHyperShiftJsonRaw(obj.GetName())
						}

						work.Status.ResourceStatus = workv1.ManifestResourceStatus{
							Manifests: []workv1.ManifestCondition{
								{
									ResourceMeta: workv1.ManifestResourceMeta{
										Ordinal:   0,
										Group:     "work.open-cluster-management.io",
										Resource:  "manifestworks",
										Kind:      "ManifestWork",
										Version:   "v1",
										Name:      work.Name,
										Namespace: work.Namespace,
									},
									StatusFeedbacks: workv1.StatusFeedbackResult{
										Values: []workv1.FeedbackValue{
											{
												Name: "status",
												Value: workv1.FieldValue{
													Type:    workv1.JsonRaw,
													JsonRaw: &jsonRaw,
												},
											},
										},
									},
								},
							},
						}

						_, err = workClient.WorkV1().ManifestWorks(obj.GetNamespace()).UpdateStatus(ctx, work, metav1.UpdateOptions{})
						if err != nil {
							klog.Errorf("error to update work: %v/%v, %v", obj.GetNamespace(), obj.GetName(), err)
							continue
						}

						o.addWorks()
						klog.Infof("work %v/%v is updated, created at: %s, watched at: %s, total=%d, used=%dms",
							obj.GetNamespace(), obj.GetName(),
							work.CreationTimestamp.UTC().Format(time.RFC3339),
							startTime.UTC().Format(time.RFC3339),
							o.getWorks(),
							util.UsedTime(startTime, time.Millisecond))
					}
				}
			}
		}()

		index = index + 1
	}

	return nil
}

func toNamespaceJsonRaw(namespace string) string {
	conditions := []metav1.Condition{}
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    workv1.ManifestApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "AppliedManifestComplete",
		Message: "Apply manifest complete",
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    workv1.ManifestAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "ResourceAvailable",
		Message: "Resource is available",
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    "StatusFeedbackSynced",
		Status:  metav1.ConditionTrue,
		Reason:  "NoStatusFeedbackSynced",
		Message: "",
	})

	resourceStatus := &workv1.ManifestCondition{
		ResourceMeta: workv1.ManifestResourceMeta{
			Ordinal:  0,
			Group:    "",
			Resource: "namespaces",
			Kind:     "Namespace",
			Version:  "v1",
			Name:     namespace,
		},
		Conditions: conditions,
	}

	data, err := json.Marshal(resourceStatus)
	if err != nil {
		klog.Fatal(err)
	}
	return string(data)
}

func toHyperShiftJsonRaw(name string) string {
	conditions := []metav1.Condition{}
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    workv1.ManifestApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "AppliedManifestComplete",
		Message: "Apply manifest complete",
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    workv1.ManifestAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "ResourceAvailable",
		Message: "Resource is available",
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    "StatusFeedbackSynced",
		Status:  metav1.ConditionTrue,
		Reason:  "StatusFeedbackSynced",
		Message: "",
	})

	resourceStatus := &workv1.ManifestResourceStatus{
		Manifests: []workv1.ManifestCondition{},
	}

	for i := 0; i < 5; i++ {
		resourceStatus.Manifests = append(resourceStatus.Manifests, workv1.ManifestCondition{
			Conditions: conditions,
			ResourceMeta: workv1.ManifestResourceMeta{
				Ordinal:   int32(i),
				Group:     "",
				Resource:  "secrets",
				Kind:      "Secret",
				Version:   "v1",
				Name:      name,
				Namespace: name,
			},
		})
	}
	jsonRaw := hostedConditions()
	resourceStatus.Manifests = append(resourceStatus.Manifests, workv1.ManifestCondition{
		Conditions: conditions,
		ResourceMeta: workv1.ManifestResourceMeta{
			Ordinal:   5,
			Group:     "hypershift.openshift.io",
			Resource:  "hostedclusters",
			Kind:      "HostedCluster",
			Version:   "v1beta1",
			Name:      name,
			Namespace: name,
		},
		StatusFeedbacks: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: "status",
					Value: workv1.FieldValue{
						Type:    workv1.JsonRaw,
						JsonRaw: &jsonRaw,
					},
				},
			},
		},
	})

	jsonRaw = nodePoolConditions()
	resourceStatus.Manifests = append(resourceStatus.Manifests, workv1.ManifestCondition{
		Conditions: conditions,
		ResourceMeta: workv1.ManifestResourceMeta{
			Ordinal:   6,
			Group:     "hypershift.openshift.io",
			Resource:  "nodepools",
			Kind:      "NodePool",
			Version:   "v1beta1",
			Name:      name,
			Namespace: name,
		},
		StatusFeedbacks: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: "status",
					Value: workv1.FieldValue{
						Type:    workv1.JsonRaw,
						JsonRaw: &jsonRaw,
					},
				},
			},
		},
	})

	data, err := json.Marshal(resourceStatus)
	if err != nil {
		klog.Fatal(err)
	}
	return string(data)
}

func (o *AROHCPWatcherOptions) addWorks() {
	o.Lock()
	defer o.Unlock()

	o.workTotals = o.workTotals + 1
}

func (o *AROHCPWatcherOptions) getWorks() int {
	o.RLock()
	defer o.RUnlock()

	return o.workTotals
}

func hostedConditions() string {
	conditions := []metav1.Condition{}
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   HostedClusterAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   HostedClusterProgressing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   HostedClusterDegraded,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   InfrastructureReady,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   KubeAPIServerAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   EtcdAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ValidHostedControlPlaneConfiguration,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   CloudResourcesDestroyed,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   HostedClusterDestroyed,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ExternalDNSReachable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ValidReleaseInfo,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionSucceeding,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionUpgradeable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionFailing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionProgressing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionProgressing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionProgressing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionReleaseAccepted,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ClusterVersionProgressing,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   UnmanagedEtcdAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   IgnitionEndpointAvailable,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   IgnitionServerValidReleaseInfo,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   SupportedHostedCluster,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ValidOIDCConfiguration,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ValidReleaseImage,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ValidAzureKMSConfig,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   PlatformCredentialsFound,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ReconciliationActive,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   ReconciliationSucceeded,
		Status: metav1.ConditionTrue,
	})

	status := map[string]string{}
	for _, c := range conditions {
		status[c.Type+"-Status"] = string(c.Status)
		status[c.Type+"-Message"] = c.Type
		status[c.Type+"-Reason"] = c.Type
		status[c.Type+"-LastTransitionTime"] = c.LastTransitionTime.Format("2006-01-02 15:04:05")
	}
	status["progress"] = "progress"
	status["Version-Desired"] = "1.22"
	status["Image-Current"] = "test-image"
	status["Version-Current"] = "1.22"
	status["Version-Status"] = "equals"

	data, err := json.Marshal(status)
	if err != nil {
		klog.Fatal(err)
	}
	return string(data)
}

func nodePoolConditions() string {
	conditions := []metav1.Condition{}
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolValidPlatformImageType,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolValidMachineConfig,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolValidTuningConfig,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolUpdateManagementEnabled,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolAutoscalingEnabled,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolReady,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolReconciliationActive,
		Status: metav1.ConditionFalse,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolAutorepairEnabled,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolUpdatingVersion,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolUpdatingConfig,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   AsExpectedReason,
		Status: metav1.ConditionTrue,
	})
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:   NodePoolValidationFailedReason,
		Status: metav1.ConditionFalse,
	})

	status := map[string]string{}
	for _, c := range conditions {
		status[c.Type+"-Status"] = string(c.Status)
		status[c.Type+"-Message"] = c.Type
		status[c.Type+"-Reason"] = c.Type
		status[c.Type+"-LastTransitionTime"] = c.LastTransitionTime.Format("2006-01-02 15:04:05")
	}

	data, err := json.Marshal(status)
	if err != nil {
		klog.Fatal(err)
	}
	return string(data)
}
