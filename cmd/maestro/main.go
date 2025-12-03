package main

import (
	"flag"
	"log"
	"os"

	"github.com/openshift-online/maestro/cmd/maestro/agent"
	"github.com/openshift-online/maestro/cmd/maestro/migrate"
	"github.com/openshift-online/maestro/cmd/maestro/servecmd"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// nolint
//
//go:generate go-bindata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi
const (
	logConfigFile = "/configs/logging/config.yaml"
	varLogLevel   = "log_level"
)

func main() {
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
