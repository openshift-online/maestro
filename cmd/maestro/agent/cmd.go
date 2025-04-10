package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/openshift-online/maestro/cmd/maestro/common"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/component-base/version"
	ocmfeature "open-cluster-management.io/api/feature"
	commonoptions "open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
)

var (
	commonOptions = commonoptions.NewAgentOptions()
	agentOption   = spoke.NewWorkloadAgentOptions()
)

func init() {
	// register the cloud events metrics for the agent
	generic.RegisterCloudEventsMetrics(legacyregistry.Registerer())
}

// by default uses 1M as the limit for state feedback
const maxJSONRawLength int32 = 1024 * 1024

type runWorkloadFN = func(ctx context.Context, controllerContext *controllercmd.ControllerContext) error

func setOtelRoundTripper(fn runWorkloadFN) runWorkloadFN {
	return func(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
		controllerContext.KubeConfig.Transport = otelhttp.NewTransport(controllerContext.KubeConfig.Transport)
		return fn(ctx, controllerContext)
	}
}

var log = logger.GetLogger()

func NewAgentCommand() *cobra.Command {
	tracingShutdown := func(context.Context) error { return nil }
	if common.TracingEnabled() {
		var err error
		tracingShutdown, err = common.InstallOpenTelemetryTracer(context.Background(), log)
		if err != nil {
			log.Errorf("Can't initialize OpenTelemetry trace provider: %v", err)
			os.Exit(1)
		}
	}
	_ = tracingShutdown // TODO: tbd

	agentOption.MaxJSONRawLength = maxJSONRawLength
	agentOption.CloudEventsClientCodecs = []string{"manifestbundle"}
	cfg := spoke.NewWorkAgentConfig(commonOptions, agentOption)
	cmdConfig := commonOptions.CommonOpts. // NOTE: As of today, the RT setting is ignored by the underlying implementation.
						NewControllerCommandConfig("maestro-agent", version.Get(), setOtelRoundTripper(cfg.RunWorkloadAgent))

	cmd := cmdConfig.NewCommandWithContext(context.TODO())
	cmd.Use = "agent"
	cmd.Short = "Start the Maestro Agent"
	cmd.Long = "Start the Maestro Agent"

	flags := cmd.PersistentFlags()
	flags.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)

	// add common flags
	// commonOptions.AddFlags(flags)
	// features.SpokeMutableFeatureGate.AddFlag(flags)
	// add agent flags
	agentOption.AddFlags(flags)
	// add alias flags
	addFlags(flags)

	// add pre-run to set feature gates
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
		utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))
	}

	return cmd
}

// addFlags overrides cluster name and leader leader election flags from the agentOption
func addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&commonOptions.SpokeClusterName, "consumer-name",
		commonOptions.SpokeClusterName, "Name of the consumer")
	fs.BoolVar(&commonOptions.CommonOpts.CmdConfig.DisableLeaderElection, "disable-leader-election",
		true, "Disable leader election.")
}
