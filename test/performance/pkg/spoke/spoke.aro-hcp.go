package spoke

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ocmfeature "open-cluster-management.io/api/feature"
	commonoptions "open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"

	"github.com/openshift-online/maestro/test/performance/pkg/util"
)

type AROHCPSpokeOptions struct {
	AgentConfigDir string

	SpokeKubeConfigPath string

	MaestroNamespace string

	ClusterBeginIndex int
	ClusterCounts     int
}

func NewAROHCPSpokeOptions() *AROHCPSpokeOptions {
	return &AROHCPSpokeOptions{
		MaestroNamespace:  "maestro",
		ClusterBeginIndex: 0,
		ClusterCounts:     1,
	}
}

func (o *AROHCPSpokeOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AgentConfigDir, "agent-config-dir", o.AgentConfigDir, "The dir to save the agent configs")
	fs.StringVar(&o.SpokeKubeConfigPath, "spoke-kubeconfig", o.SpokeKubeConfigPath, "Location of the Spoke kubeconfig")
	fs.IntVar(&o.ClusterBeginIndex, "cluster-begin-index", o.ClusterBeginIndex, "Begin index of the clusters")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
}

func (o *AROHCPSpokeOptions) Run(ctx context.Context) error {
	spokeKubeConfig, err := clientcmd.BuildConfigFromFlags("", o.SpokeKubeConfigPath)
	if err != nil {
		return err
	}

	spokeKubeClient, err := kubernetes.NewForConfig(spokeKubeConfig)
	if err != nil {
		return err
	}

	// start agents
	utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
	utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))

	index := o.ClusterBeginIndex
	for i := 0; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(index)

		_, err := spokeKubeClient.CoreV1().Namespaces().Get(ctx, clusterName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			if _, err := spokeKubeClient.CoreV1().Namespaces().Create(
				ctx,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
				},
				metav1.CreateOptions{},
			); err != nil {
				return err
			}
		case err != nil:
			return err
		}

		klog.Infof("The namespace of cluster %s is created", clusterName)

		go func() {
			klog.Infof("Starting the work agent for cluster %s", clusterName)
			if err := o.startWorkAgent(ctx, spokeKubeConfig, clusterName); err != nil {
				klog.Errorf("failed to start work agent for cluster %s, %v", clusterName, err)
			}
		}()

		index = index + 1
	}

	return nil
}

func (o *AROHCPSpokeOptions) startWorkAgent(ctx context.Context, kubeConfig *rest.Config, clusterName string) error {
	commonOptions := commonoptions.NewAgentOptions()
	commonOptions.AgentID = string(uuid.NewUUID())
	commonOptions.SpokeClusterName = clusterName

	agentOptions := spoke.NewWorkloadAgentOptions()
	agentOptions.StatusSyncInterval = 3 * time.Second
	agentOptions.AppliedManifestWorkEvictionGracePeriod = 5 * time.Second
	agentOptions.MaxJSONRawLength = 1024 * 1024 // 1M
	agentOptions.WorkloadSourceDriver = "mqtt"
	agentOptions.WorkloadSourceConfig = filepath.Join(o.AgentConfigDir, fmt.Sprintf("%s-config.yaml", clusterName))
	agentOptions.CloudEventsClientID = fmt.Sprintf("%s-agent", clusterName)
	agentOptions.CloudEventsClientCodecs = []string{"manifestbundle"}

	agentConfig := spoke.NewWorkAgentConfig(commonOptions, agentOptions)
	return agentConfig.RunWorkloadAgent(ctx, &controllercmd.ControllerContext{
		KubeConfig:    kubeConfig,
		EventRecorder: util.NewRecorder(clusterName),
	})
}
