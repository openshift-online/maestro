package environments

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/spf13/pflag"
)

func BenchmarkGetResources(b *testing.B) {
	b.ReportAllocs()
	fn := func(b *testing.B) {
		cmd := exec.Command("ocm", "get", "/api/maestro/v1/resources", "params='size=2'")
		_, err := cmd.CombinedOutput()
		if err != nil {
			b.Errorf("ERROR %+v", err)
		}
	}
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}

func TestLoadServices(t *testing.T) {
	env := Environment()
	// Override environment name
	env.Name = "testing"
	err := env.AddFlags(pflag.CommandLine)
	if err != nil {
		t.Errorf("Unable to add flags for testing environment: %s", err.Error())
		return
	}
	pflag.Parse()
	mqttBroker := startMQTTBroker(t)
	if err != nil {
		t.Errorf("Unable to start MQTT broker: %s", err.Error())
		return
	}
	err = env.Initialize()
	if err != nil {
		t.Errorf("Unable to load testing environment: %s", err.Error())
		return
	}

	s := reflect.ValueOf(env.Services)

	for i := 0; i < s.NumField(); i++ {
		if s.Field(i).IsNil() {
			t.Errorf("Service %v is nil", s)
		}
	}

	if err := stopMQTTBroker(mqttBroker); err != nil {
		t.Errorf("Unable to stop MQTT broker: %s", err.Error())
		return
	}
}

func startMQTTBroker(t *testing.T) *mqtt.Server {
	pass := genRandomStr(13)
	err := os.WriteFile(filepath.Join(config.GetProjectRootDir(), "secrets/mqtt.password"), []byte(pass), 0644)
	if err != nil {
		t.Errorf("Unable to write mqtt password file: %s", err.Error())
	}

	authRules := &auth.Ledger{
		Auth: auth.AuthRules{
			{Username: "maestro", Password: auth.RString(pass), Allow: true},
		},
	}

	mqttBroker := mqtt.New(nil)
	if err := mqttBroker.AddHook(new(auth.AllowHook), authRules); err != nil {
		t.Errorf("Unable to add auth hook to mqtt broker: %s", err)
	}
	if err := mqttBroker.AddListener(listeners.NewTCP("tcp1", ":1883", nil)); err != nil {
		t.Errorf("Unable to add listener to mqtt broker: %s", err)
	}

	go func() {
		if err := mqttBroker.Serve(); err != nil {
			t.Errorf("Unable to start MQTT broker: %s", err)
		}
	}()

	return mqttBroker
}

func stopMQTTBroker(mqttBroker *mqtt.Server) error {
	if err := os.Remove(filepath.Join(config.GetProjectRootDir(), "secrets/mqtt.password")); err != nil {
		return fmt.Errorf("unable to remove mqtt password file: %s", err)
	}
	if err := mqttBroker.Close(); err != nil {
		return fmt.Errorf("unable to close MQTT broker: %s", err.Error())
	}
	return nil
}

func genRandomStr(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
