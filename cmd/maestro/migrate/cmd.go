package migrate

import (
	"context"
	"flag"

	"github.com/golang/glog"
	"github.com/openshift-online/maestro/pkg/db/db_session"
	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
)

var dbConfig = config.NewDatabaseConfig()

// migration sub-command handles running migrations
func NewMigrationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migration",
		Short: "Run maestro service data migrations",
		Long:  "Run maestro service data migrations",
		Run:   runMigration,
	}

	dbConfig.AddFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func runMigration(_ *cobra.Command, _ []string) {
	err := dbConfig.ReadFiles()
	if err != nil {
		glog.Fatal(err)
	}

	connection := db_session.NewProdFactory(dbConfig)
	if err := db.Migrate(connection.New(context.Background())); err != nil {
		glog.Fatal(err)
	}
}
