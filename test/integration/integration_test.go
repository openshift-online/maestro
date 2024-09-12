package integration

import (
	"flag"
	"os"
	"runtime"
	"testing"

	"github.com/openshift-online/maestro/test"
	"k8s.io/klog/v2"
)

func TestMain(m *testing.M) {
	flag.Parse()
	klog.Infof("Starting integration test using go version %s", runtime.Version())
	helper := test.NewHelper(&testing.T{})
	exitCode := m.Run()
	helper.Teardown()
	helper.CleanDB()
	os.Exit(exitCode)
}
