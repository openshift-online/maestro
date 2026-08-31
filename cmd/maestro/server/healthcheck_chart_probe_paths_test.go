package server

import (
	"os"
	"strings"
	"testing"
)

func TestMaestroServerChartProbePaths(t *testing.T) {
	content, err := os.ReadFile("../../../charts/maestro-server/templates/deployment.yaml")
	if err != nil {
		t.Fatalf("failed to read chart deployment template: %v", err)
	}

	tpl := string(content)
	if !strings.Contains(tpl, "livenessProbe:\n          httpGet:\n            path: /livez") {
		t.Fatalf("expected liveness probe path to be /livez")
	}

	if !strings.Contains(tpl, "readinessProbe:\n          httpGet:\n            path: /healthcheck") {
		t.Fatalf("expected readiness probe path to remain /healthcheck")
	}
}
