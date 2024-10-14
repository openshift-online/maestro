package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/go-logr/zapr"
	"github.com/openshift-online/maestro/cmd/maestro/agent"
	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/migrate"
	"github.com/openshift-online/maestro/cmd/maestro/servecmd"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

// nolint
//
//go:generate go-bindata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi

func main() {
	// check if the glog flag is already registered to avoid duplicate flag define error
	if flag.CommandLine.Lookup("alsologtostderr") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	// add klog flags
	klog.InitFlags(nil)

	// Set up klog backing logger
	cobra.OnInitialize(func() {
		// Retrieve log level from klog flags
		logLevel, err := strconv.ParseInt(flag.CommandLine.Lookup("v").Value.String(), 10, 8)
		if err != nil {
			klog.Fatalf("can't parse log level: %v", err)
		}

		// Initialize zap logger based on environment
		var zc zap.Config
		env := environments.GetEnvironmentStrFromEnv()
		switch env {
		case environments.DevelopmentEnv:
			zc = zap.NewDevelopmentConfig()
		case environments.ProductionEnv:
			zc = zap.NewProductionConfig()
		default:
			zc = zap.NewDevelopmentConfig()
		}

		// zap log level is the inverse of klog log level, for more details refer to:
		// https://github.com/go-logr/zapr?tab=readme-ov-file#increasing-verbosity
		zc.Level = zap.NewAtomicLevelAt(zapcore.Level(0 - logLevel))
		// Disable stacktrace for production environment
		zc.DisableStacktrace = true
		zapLog, err := zc.Build()
		if err != nil {
			klog.Fatalf("can't initialize zap logger: %v", err)
		}
		// Set backing logger for klog
		klog.SetLogger(zapr.NewLogger(zapLog))
	})

	// Initialize root command
	rootCmd := &cobra.Command{
		Use:  "maestro",
		Long: "maestro is a multi-cluster resources orchestrator for Kubernetes",
	}

	// Add klog flags to root command
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// All subcommands under root
	migrateCmd := migrate.NewMigrationCommand()
	serveCmd := servecmd.NewServerCommand()
	agentCmd := agent.NewAgentCommand()

	// Add subcommand(s)
	rootCmd.AddCommand(migrateCmd, serveCmd, agentCmd)

	if err := rootCmd.Execute(); err != nil {
		klog.Fatalf("error running command: %v", err)
	}
}
