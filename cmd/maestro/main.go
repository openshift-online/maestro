package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/cmd/maestro/agent"
	"github.com/openshift-online/maestro/cmd/maestro/migrate"
	"github.com/openshift-online/maestro/cmd/maestro/servecmd"
)

// nolint
//
//go:generate go-bindata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi

func main() {
	// This is needed to make `glog` believe that the flags have already been parsed, otherwise
	// every log messages is prefixed by an error message stating the the flags haven't been
	// parsed.
	_ = flag.CommandLine.Parse([]string{})

	//pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Always log to stderr by default
	if err := flag.Set("logtostderr", "true"); err != nil {
		glog.Infof("Unable to set logtostderr to true")
	}

	rootCmd := &cobra.Command{
		Use:  "maestro",
		Long: "maestro is a multi-cluster resources orchestrator for Kubernetes",
	}

	// All subcommands under root
	migrateCmd := migrate.NewMigrationCommand()
	serveCmd := servecmd.NewServerCommand()
	agentCmd := agent.NewAgentCommand()

	// Add subcommand(s)
	rootCmd.AddCommand(migrateCmd, serveCmd, agentCmd)

	if err := rootCmd.Execute(); err != nil {
		glog.Fatalf("error running command: %v", err)
	}
}
