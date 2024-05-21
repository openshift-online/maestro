package agent

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	ocmfeature "open-cluster-management.io/api/feature"
	"open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"
)

func NewAgentCommand() *cobra.Command {
	agentOptions := NewAgentOptions()

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start the maestro agent",
		Long:  "Start the maestro agent.",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())

			stopCh := make(chan os.Signal, 1)
			signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				defer cancel()
				<-stopCh
			}()

			if err := agentOptions.Run(ctx); err != nil {
				klog.Fatal(err)
			}

			<-ctx.Done()
		},
	}

	// check if the flag is already registered to avoid duplicate flag define error
	if flag.CommandLine.Lookup("alsologtostderr") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
	// add klog flags
	klog.InitFlags(nil)

	fs := cmd.PersistentFlags()
	fs.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	fs.AddGoFlagSet(flag.CommandLine)
	agentOptions.AddFlags(fs)

	utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
	utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))

	return cmd
}

type AgentOptions struct {
	CommonOptions  *options.AgentOptions
	WorkOptions    *spoke.WorkloadAgentOptions
	KubeConfigFile string
	Namespace      string
}

func NewAgentOptions() *AgentOptions {
	workOptions := spoke.NewWorkloadAgentOptions()
	// use 1M as the default limit for state feedback
	workOptions.MaxJSONRawLength = 1024 * 1024
	// use mqtt as the default driver
	workOptions.WorkloadSourceDriver = "mqtt"
	// use manifest as the default codec
	workOptions.CloudEventsClientCodecs = []string{"manifest"}

	return &AgentOptions{
		CommonOptions: options.NewAgentOptions(),
		WorkOptions:   workOptions,
	}
}

func (o *AgentOptions) Run(ctx context.Context) error {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Warningf("failed to get kubeconfig from cluster inside, will use '--kubeconfig' to build client")

		kubeConfig, err = clientcmd.BuildConfigFromFlags("", o.KubeConfigFile)
		if err != nil {
			return fmt.Errorf("unable to load kubeconfig from file %q: %v", o.KubeConfigFile, err)
		}
	}

	namespace := o.Namespace
	if len(namespace) == 0 {
		namespace, err = getComponentNamespace()
		if err != nil {
			return err
		}
	}

	controllerContext := &controllercmd.ControllerContext{
		KubeConfig:        kubeConfig,
		EventRecorder:     events.NewLoggingEventRecorder("maestro-agent"),
		OperatorNamespace: namespace,
	}

	return spoke.NewWorkAgentConfig(o.CommonOptions, o.WorkOptions).
		RunWorkloadAgent(ctx, controllerContext)
}

func (o *AgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KubeConfigFile, "kubeconfig",
		o.KubeConfigFile, "Location of the kubeconfig file")
	fs.StringVar(&o.Namespace, "namespace",
		o.Namespace, "Namespace where the agent runs")
	// workloadAgentOptions
	fs.Float32Var(&o.CommonOptions.CommoOpts.QPS, "kube-api-qps",
		o.CommonOptions.CommoOpts.QPS, "QPS to use while talking with apiserver on spoke cluster")
	fs.IntVar(&o.CommonOptions.CommoOpts.Burst, "kube-api-burst",
		o.CommonOptions.CommoOpts.Burst, "Burst to use while talking with apiserver on spoke cluster")
	fs.Int32Var(&o.WorkOptions.MaxJSONRawLength, "max-json-raw-length",
		o.WorkOptions.MaxJSONRawLength, "The maximum size of the JSON raw string returned from status feedback")
	fs.DurationVar(&o.WorkOptions.StatusSyncInterval, "status-sync-interval",
		o.WorkOptions.StatusSyncInterval, "Interval to sync resource status to hub")
	fs.DurationVar(&o.WorkOptions.AppliedManifestWorkEvictionGracePeriod, "resource-eviction-grace-period",
		o.WorkOptions.AppliedManifestWorkEvictionGracePeriod, "Grace period for resource eviction")
	fs.StringVar(&o.CommonOptions.SpokeClusterName, "consumer-name",
		o.CommonOptions.SpokeClusterName, "Name of the consumer")
	// message broker config file
	fs.StringVar(&o.WorkOptions.WorkloadSourceConfig, "message-broker-config-file",
		o.WorkOptions.WorkloadSourceConfig, "The config file path of the message broker, it can be mqtt broker or kafka broker")
	fs.StringVar(&o.WorkOptions.WorkloadSourceDriver, "message-broker-type",
		o.WorkOptions.WorkloadSourceDriver, "Message broker type")
	fs.StringVar(&o.WorkOptions.CloudEventsClientID, "agent-client-id",
		o.WorkOptions.CloudEventsClientID, "The ID of the agent client, by default it is <consumer-id>-work-agent")
	fs.StringSliceVar(&o.WorkOptions.CloudEventsClientCodecs, "agent-client-codecs",
		o.WorkOptions.CloudEventsClientCodecs, "The codecs of the agent client. The valid codecs are manifest and manifestbundle")
}

func getComponentNamespace() (string, error) {
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	return string(nsBytes), nil
}
