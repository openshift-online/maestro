package agent

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	ocmfeature "open-cluster-management.io/api/feature"
	commonoptions "open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"
)

var (
	commonOptions = commonoptions.NewAgentOptions()
	agentOption   = spoke.NewWorkloadAgentOptions()
)

// by default uses 1M as the limit for state feedback
const maxJSONRawLength int32 = 1024 * 1024

func NewAgentCommand() *cobra.Command {
	agentOption.MaxJSONRawLength = maxJSONRawLength
	agentOption.CloudEventsClientCodecs = []string{"manifest", "manifestbundle"}
	cfg := spoke.NewWorkAgentConfig(commonOptions, agentOption)
	cmdConfig := commonOptions.CommonOpts.
		NewControllerCommandConfig("maestro-agent", version.Get(), cfg.RunWorkloadAgent)

	cmd := cmdConfig.NewCommandWithContext(context.TODO())
	cmd.Use = "agent"
	cmd.Short = "Start the Maestro Agent"
	cmd.Long = "Start the Maestro Agent"

	// check if the flag is already registered to avoid duplicate flag define error
	if flag.CommandLine.Lookup("alsologtostderr") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	// add klog flags
	klog.InitFlags(nil)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)

	// add common flags
	// commonOptions.AddFlags(flags)
	// features.SpokeMutableFeatureGate.AddFlag(flags)
	// add agent flags
	agentOption.AddFlags(flags)
	// add alias flags
	addFlags(flags)

	utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
	utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))

	return cmd
}

// addFlags overrides cluster name and leader leader election flags from the agentOption
func addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&commonOptions.SpokeClusterName, "consumer-name",
		commonOptions.SpokeClusterName, "Name of the consumer")
	fs.BoolVar(&commonOptions.CommonOpts.CmdConfig.DisableLeaderElection, "disable-leader-election",
		true, "Disable leader election.")
}
