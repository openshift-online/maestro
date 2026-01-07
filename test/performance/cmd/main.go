package main

import (
	"context"
	goflag "flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/server"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/test/performance/pkg/hub"
	"github.com/openshift-online/maestro/test/performance/pkg/spoke"
	"github.com/openshift-online/maestro/test/performance/pkg/watcher"
)

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(pflag.CommandLine)
	logs.InitLogs()
	defer logs.FlushLogs()

	cmd := &cobra.Command{
		Use:   "maestroperf",
		Short: "Maestro Performance Test Tool",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(
		newAROHCPPreparationCommand(),
		newAROHCPSpokeCommand(),
		newAROHCPWatchCommand(),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newAROHCPPreparationCommand() *cobra.Command {
	o := hub.NewAROHCPPreparerOptions()
	cmd := &cobra.Command{
		Use:   "aro-hcp-prepare",
		Short: "Prepare clusters or works in Maestro for ARO HCP",
		Long:  "Prepare clusters or works in Maestro for ARO HCP",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("\nShutting down aro-hcp-prepare.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run aro-hcp-prepare, %v", err)
			}
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}

func newAROHCPSpokeCommand() *cobra.Command {
	o := spoke.NewAROHCPSpokeOptions()
	cmd := &cobra.Command{
		Use:   "aro-hcp-spoke",
		Short: "Start agents for ARO HCP",
		Long:  "Start agents for ARO HCP",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("\nShutting down aro-hcp-spoke.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run aro-hcp-spoke, %v", err)
			}

			<-ctx.Done()
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}

func newAROHCPWatchCommand() *cobra.Command {
	o := watcher.NewAROHCPWatcherOptions()
	cmd := &cobra.Command{
		Use:   "aro-hcp-watch",
		Short: "Start watcher for ARO HCP",
		Long:  "Start watcher for ARO HCP",
		Run: func(cmd *cobra.Command, args []string) {
			// handle SIGTERM and SIGINT by cancelling the context.
			ctx, cancel := context.WithCancel(context.Background())
			shutdownHandler := server.SetupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
				klog.Infof("\nShutting down aro-hcp-watch.")
			}()

			if err := o.Run(ctx); err != nil {
				klog.Errorf("failed to run aro-hcp-watch, %v", err)
			}

			<-ctx.Done()
		},
	}

	flags := cmd.Flags()
	o.AddFlags(flags)
	return cmd
}
