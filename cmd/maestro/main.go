package main

import (
	"flag"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/openshift-online/maestro/cmd/maestro/agent"
	"github.com/openshift-online/maestro/cmd/maestro/migrate"
	"github.com/openshift-online/maestro/cmd/maestro/servecmd"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

// nolint
//
//go:generate go-bindata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi
const (
	logConfigFile = "/configs/logging/config.yaml"
	varLogLevel   = "log_level"
)

var log = logger.GetLogger()

func main() {
	defer logger.SyncLogger() // flush the logger

	// check if the glog flag is already registered to avoid duplicate flag define error
	if flag.CommandLine.Lookup("alsologtostderr") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	// add klog flags
	klog.InitFlags(nil)

	// Initialize root command
	rootCmd := &cobra.Command{
		Use:  "maestro",
		Long: "maestro is a multi-cluster resources orchestrator for Kubernetes",
	}

	// set the logging config file
	viper.SetConfigFile(logConfigFile)
	// default log level is info
	logger.SetLogLevel("info")
	if err := viper.ReadInConfig(); err != nil {
		log.Warnf("failed to read the log config file '%s', using info as default log level, %v", logConfigFile, err)
	} else {
		logger.SetLogLevel(viper.GetString(varLogLevel))
	}
	// monitor the changes in the config file
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Infof("log config file changed: %s, new log level: %s", e.Name, viper.GetString(varLogLevel))
		logger.SetLogLevel(viper.GetString(varLogLevel))
	})

	// Add klog flags to root command
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// All subcommands under root
	migrateCmd := migrate.NewMigrationCommand()
	serveCmd := servecmd.NewServerCommand()
	agentCmd := agent.NewAgentCommand()

	// Add subcommand(s)
	rootCmd.AddCommand(migrateCmd, serveCmd, agentCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("error running command: %v", err)
	}
}
