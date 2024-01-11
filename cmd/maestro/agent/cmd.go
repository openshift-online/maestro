package agent

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	ocmfeature "open-cluster-management.io/api/feature"
	"open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"
)

var (
	commonOptions = options.NewAgentOptions()
	agentOption   = spoke.NewWorkloadAgentOptions()
)

func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start the maestro agent",
		Long:  "Start the maestro agent.",
		Run:   runAgent,
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
	commonOptions.CommoOpts.AddFlags(fs)
	addFlags(fs)
	utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
	utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))

	return cmd
}

func runAgent(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
	}()

	// use mqtt as the default driver
	agentOption.WorkloadSourceDriver.Type = spoke.MQTTDriver

	cfg := spoke.NewWorkAgentConfig(commonOptions, agentOption)
	cmdConfig := commonOptions.CommoOpts.
		NewControllerCommandConfig("maestro-agent", version.Get(), cfg.RunWorkloadAgent)
	cmdConfig.DisableLeaderElection = true

	if err := cmdConfig.StartController(ctx); err != nil {
		glog.Fatalf("error running command: %v", err)
	}
}

func addFlags(fs *pflag.FlagSet) {
	// workloadAgentOptions
	fs.DurationVar(&agentOption.StatusSyncInterval, "status-sync-interval", agentOption.StatusSyncInterval, "Interval to sync resource status to hub")
	fs.DurationVar(&agentOption.AppliedManifestWorkEvictionGracePeriod, "resource-eviction-grace-period",
		agentOption.AppliedManifestWorkEvictionGracePeriod, "Grace period for resource eviction")
	fs.StringVar(&commonOptions.SpokeClusterName, "consumer-id", commonOptions.SpokeClusterName, "Id of the consumer")
	// mqtt config file
	fs.StringVar(&agentOption.WorkloadSourceDriver.Config, "mqtt-config-file",
		agentOption.WorkloadSourceDriver.Config, "The config file path of mqtt broker")

}
