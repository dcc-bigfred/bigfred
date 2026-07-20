package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	histogramRemoteICMPRTT   = "bigfred.remote_icmp.rtt"
	counterRemoteICMPTimeout = "bigfred.remote_icmp.timeouts"
	counterRemoteICMPProbes  = "bigfred.remote_icmp.probes"
)

// Metrics records ICMP probe outcomes.
type Metrics struct {
	rtt      metric.Float64Histogram
	timeouts metric.Int64Counter
	probes   metric.Int64Counter
}

// NewMetrics registers OTel instruments. Returns nil metrics only via error.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("github.com/keskad/loco/pkgs/bigfred/remote-icmp")
	rtt, err := meter.Float64Histogram(histogramRemoteICMPRTT,
		metric.WithDescription("ICMP Echo round-trip latency to handset IP"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("remote_icmp rtt histogram: %w", err)
	}
	timeouts, err := meter.Int64Counter(counterRemoteICMPTimeout,
		metric.WithDescription("ICMP Echo probes that timed out"),
		metric.WithUnit("{probe}"),
	)
	if err != nil {
		return nil, fmt.Errorf("remote_icmp timeouts counter: %w", err)
	}
	probes, err := meter.Int64Counter(counterRemoteICMPProbes,
		metric.WithDescription("ICMP Echo probes attempted"),
		metric.WithUnit("{probe}"),
	)
	if err != nil {
		return nil, fmt.Errorf("remote_icmp probes counter: %w", err)
	}
	return &Metrics{rtt: rtt, timeouts: timeouts, probes: probes}, nil
}

func (m *Metrics) attrs(t ProbeTarget) []attribute.KeyValue {
	login := t.Login
	if login == "" {
		login = "_"
	}
	proto := t.Protocol
	if proto == "" {
		proto = "_"
	}
	return []attribute.KeyValue{
		attribute.Int("layout.id", int(t.LayoutID)),
		attribute.Int("command_station.id", int(t.CommandStationID)),
		attribute.String("protocol", proto),
		attribute.String("user.login", login),
	}
}

// RecordProbe increments the attempt counter.
func (m *Metrics) RecordProbe(t ProbeTarget) {
	if m == nil {
		return
	}
	m.probes.Add(context.Background(), 1, metric.WithAttributes(m.attrs(t)...))
}

// RecordRTT stores a successful RTT sample.
func (m *Metrics) RecordRTT(t ProbeTarget, rtt time.Duration) {
	if m == nil || rtt <= 0 {
		return
	}
	m.rtt.Record(context.Background(), rtt.Seconds(), metric.WithAttributes(m.attrs(t)...))
}

// RecordTimeout stores a timed-out probe.
func (m *Metrics) RecordTimeout(t ProbeTarget) {
	if m == nil {
		return
	}
	m.timeouts.Add(context.Background(), 1, metric.WithAttributes(m.attrs(t)...))
}
