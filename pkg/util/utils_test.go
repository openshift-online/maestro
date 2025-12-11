package util

import (
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestFormatEventContext(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() cloudevents.Event
		expected string
	}{
		{
			name: "basic event without extensions",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("test-id-123")
				event.SetType("io.maestro.test")
				event.SetSource("test-source")
				return event
			},
			expected: "id=test-id-123 type=io.maestro.test source=test-source",
		},
		{
			name: "event with single extension",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("event-001")
				event.SetType("resource.update")
				event.SetSource("/maestro/server")
				event.SetExtension("clustername", "prod-cluster")
				return event
			},
			expected: "id=event-001 type=resource.update source=/maestro/server clustername=prod-cluster",
		},
		{
			name: "event with multiple extensions sorted alphabetically",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("multi-ext-001")
				event.SetType("resource.created")
				event.SetSource("/api/v1")
				event.SetExtension("resourceid", "res-123")
				event.SetExtension("clustername", "dev-cluster")
				event.SetExtension("action", "create")
				return event
			},
			expected: "id=multi-ext-001 type=resource.created source=/api/v1 action=create clustername=dev-cluster resourceid=res-123",
		},
		{
			name: "event with numeric extension values",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("numeric-001")
				event.SetType("metric.report")
				event.SetSource("/metrics")
				event.SetExtension("count", 42)
				event.SetExtension("version", 1)
				return event
			},
			expected: "id=numeric-001 type=metric.report source=/metrics count=42 version=1",
		},
		{
			name: "event with mixed extension value types",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("mixed-001")
				event.SetType("test.event")
				event.SetSource("/test")
				event.SetExtension("stringval", "hello")
				event.SetExtension("intval", 100)
				event.SetExtension("boolval", true)
				return event
			},
			expected: "id=mixed-001 type=test.event source=/test boolval=true intval=100 stringval=hello",
		},
		{
			name: "real production event with manifestbundle status update",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("43b54816-b566-4067-bcc6-6078ad403e92")
				event.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
				event.SetSource("79b36e63-6026-4267-b7aa-0dda3882519c-work-agent")
				event.SetExtension("clustername", "79b36e63-6026-4267-b7aa-0dda3882519c")
				event.SetExtension("logtracing", "{}")
				event.SetExtension("metadata", `{"creationTimestamp":"2025-12-11T06:52:51Z","name":"work-9kln4","namespace":"79b36e63-6026-4267-b7aa-0dda3882519c","resourceVersion":"0","uid":"22305ac3-ed9b-5973-8566-7abbdf12d0a0"}`)
				event.SetExtension("originalsource", "maestro")
				event.SetExtension("resourceid", "22305ac3-ed9b-5973-8566-7abbdf12d0a0")
				event.SetExtension("resourceversion", "1")
				event.SetExtension("sequenceid", "1999009558613200896")
				event.SetExtension("statushash", "e8b4885504782a40f0dbb3de83806bce6667e7bfd368d8be75e9972c42da6870")
				return event
			},
			expected: `id=43b54816-b566-4067-bcc6-6078ad403e92 type=io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request source=79b36e63-6026-4267-b7aa-0dda3882519c-work-agent clustername=79b36e63-6026-4267-b7aa-0dda3882519c logtracing={} metadata={"creationTimestamp":"2025-12-11T06:52:51Z","name":"work-9kln4","namespace":"79b36e63-6026-4267-b7aa-0dda3882519c","resourceVersion":"0","uid":"22305ac3-ed9b-5973-8566-7abbdf12d0a0"} originalsource=maestro resourceid=22305ac3-ed9b-5973-8566-7abbdf12d0a0 resourceversion=1 sequenceid=1999009558613200896 statushash=e8b4885504782a40f0dbb3de83806bce6667e7bfd368d8be75e9972c42da6870`,
		},
		{
			name: "real production event with complex metadata including labels and finalizers",
			setup: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetID("43b54816-b566-4067-bcc6-6078ad403e92")
				event.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request")
				event.SetSource("79b36e63-6026-4267-b7aa-0dda3882519c-work-agent")
				event.SetExtension("clustername", "79b36e63-6026-4267-b7aa-0dda3882519c")
				event.SetExtension("logtracing", "{}")
				event.SetExtension("metadata", `{"name":"work-9kln4","namespace":"79b36e63-6026-4267-b7aa-0dda3882519c","uid":"22305ac3-ed9b-5973-8566-7abbdf12d0a0","generation":1,"creationTimestamp":"2025-12-11T06:52:51Z","labels":{"cloudevents.open-cluster-management.io/originalsource":"maestro"},"annotations":{"cloudevents.open-cluster-management.io/datatype":"io.open-cluster-management.works.v1alpha1.manifestbundles"},"finalizers":["cluster.open-cluster-management.io/manifest-work-cleanup"]}`)
				event.SetExtension("originalsource", "maestro")
				event.SetExtension("resourceid", "22305ac3-ed9b-5973-8566-7abbdf12d0a0")
				event.SetExtension("resourceversion", "1")
				event.SetExtension("sequenceid", "1999009558613200896")
				event.SetExtension("statushash", "e8b4885504782a40f0dbb3de83806bce6667e7bfd368d8be75e9972c42da6870")
				return event
			},
			expected: `id=43b54816-b566-4067-bcc6-6078ad403e92 type=io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request source=79b36e63-6026-4267-b7aa-0dda3882519c-work-agent clustername=79b36e63-6026-4267-b7aa-0dda3882519c logtracing={} metadata={"name":"work-9kln4","namespace":"79b36e63-6026-4267-b7aa-0dda3882519c","uid":"22305ac3-ed9b-5973-8566-7abbdf12d0a0","generation":1,"creationTimestamp":"2025-12-11T06:52:51Z","labels":{"cloudevents.open-cluster-management.io/originalsource":"maestro"},"annotations":{"cloudevents.open-cluster-management.io/datatype":"io.open-cluster-management.works.v1alpha1.manifestbundles"},"finalizers":["cluster.open-cluster-management.io/manifest-work-cleanup"]} originalsource=maestro resourceid=22305ac3-ed9b-5973-8566-7abbdf12d0a0 resourceversion=1 sequenceid=1999009558613200896 statushash=e8b4885504782a40f0dbb3de83806bce6667e7bfd368d8be75e9972c42da6870`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tt.setup()
			result := FormatEventContext(event.Context)
			if result != tt.expected {
				t.Errorf("FormatEventContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}
