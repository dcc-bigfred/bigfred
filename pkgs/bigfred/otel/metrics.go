// Package otel wires optional OpenTelemetry metric export for BigFred
// daemons. When no endpoint is configured, callers keep the default noop
// global MeterProvider.
package otel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitMetrics configures a periodic OTLP/gRPC metric exporter when
// endpoint is non-empty. Returns a shutdown hook that flushes and
// releases the exporter; the hook is a no-op when endpoint is empty.
func InitMetrics(ctx context.Context, serviceName, endpoint string) (func(context.Context) error, error) {
	endpoint = normalizeOTLPEndpoint(endpoint)
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	if serviceName == "" {
		serviceName = "bigfred"
	}

	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(exp,
		sdkmetric.WithInterval(15*time.Second),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

func normalizeOTLPEndpoint(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
