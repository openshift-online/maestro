package util

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const clusterNamePrefix = "maestro-cluster"

type Func func() error

func ClusterName(index int) string {
	return fmt.Sprintf("%s-%d", clusterNamePrefix, index)
}

func UsedTime(start time.Time, unit time.Duration) time.Duration {
	used := time.Since(start)
	return used / unit
}

func Eventually(fn Func, timeout time.Duration, interval time.Duration) error {
	after := time.After(timeout)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	var err error
	for {
		select {
		case <-after:
			return fmt.Errorf("timeout with error %v", err)
		case <-tick.C:
			err = fn()

			if err == nil {
				return nil
			}
		}
	}
}

func Render(name string, data []byte, config interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, err
	}

	return yaml.YAMLToJSON(buf.Bytes())
}

func NewMaestroAPIClient(maestroServerAddress string) *openapi.APIClient {
	cfg := &openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/1.0.0/go",
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         maestroServerAddress,
				Description: "current domain",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	return openapi.NewAPIClient(cfg)
}

func CreateConsumer(ctx context.Context, client *openapi.APIClient, consumerName string) error {
	_, _, err := client.DefaultApi.ApiMaestroV1ConsumersPost(ctx).
		Consumer(openapi.Consumer{Name: openapi.PtrString(consumerName)}).
		Execute()
	return err
}

func MonitorMem(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			printMemUsage()
		}
	}
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	klog.Infof("#### Alloc=%d,TotalAlloc=%d,Sys=%d,NumGC=%d",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
