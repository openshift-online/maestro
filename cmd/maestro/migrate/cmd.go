package migrate

import (
	"context"

	"github.com/openshift-online/maestro/pkg/db/db_session"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/spf13/cobra"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
)

var log = logger.GetLogger()
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
	return cmd
}

func runMigration(_ *cobra.Command, _ []string) {
	err := dbConfig.ReadFiles()
	if err != nil {
		log.Fatal(err)
	}

	connection := db_session.NewProdFactory(dbConfig)
	if err := db.Migrate(connection.New(context.Background())); err != nil {
		log.Fatal(err)
	}
}
