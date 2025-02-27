package integration

import (
	"flag"
	"os"
	"runtime"
	"testing"

	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/test"
)

var log = logger.GetLogger()

func TestMain(m *testing.M) {
	flag.Parse()
	log.Infof("Starting integration test using go version %s", runtime.Version())
	helper := test.NewHelper(&testing.T{})
	exitCode := m.Run()
	helper.Teardown()
	helper.CleanDB()
	os.Exit(exitCode)
}
