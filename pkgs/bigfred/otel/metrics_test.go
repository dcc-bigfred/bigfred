package otel

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

func TestResourceMergeMatchesSDKDefaultSchema(t *testing.T) {
	_, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("loco-server"),
		),
	)
	if err != nil {
		t.Fatalf("resource.Merge: %v", err)
	}
}
