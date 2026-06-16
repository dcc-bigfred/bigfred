package ws

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
)

const (
	histogramWSCommandDuration = "bigfred.dcc_bus.ws.command.duration"
	histogramWSClientPingRTT     = "bigfred.dcc_bus.ws.client_ping_rtt"
	counterWSSessionsClosed      = "bigfred.dcc_bus.ws.sessions.closed"
	counterWSDeadmanTriggered    = "bigfred.dcc_bus.ws.deadman.triggered"
	upDownWSSessionsActive       = "bigfred.dcc_bus.ws.sessions.active"
	commandInvalidEnvelope       = "envelope.invalid"
	ErrorCodeSendFailed          = "send_failed"
	maxClientPingRTTMs           = 30_000
)

// Outcome is the observable result of handling one inbound WS command.
type Outcome struct {
	Success   bool
	ErrorCode string
}

// OK returns a successful command outcome.
func OK() Outcome {
	return Outcome{Success: true}
}

// Fail returns a failed command outcome with a machine-readable code.
func Fail(code string) Outcome {
	return Outcome{Success: false, ErrorCode: code}
}

// MetricsConfig toggles dcc-bus WebSocket metric export.
type MetricsConfig struct {
	Enabled          bool
	LayoutID         uint
	CommandStationID uint
	Meter            metric.Meter
}

// Metrics records WebSocket commands and session lifecycle events.
type Metrics struct {
	hist             metric.Float64Histogram
	clientPingRTT    metric.Float64Histogram
	sessionsActive   metric.Int64UpDownCounter
	sessionsClosed   metric.Int64Counter
	deadmanTriggered metric.Int64Counter
	base             []attribute.KeyValue
}

// CommandMetrics is an alias kept for callers that only refer to command
// latency instrumentation.
type CommandMetrics = Metrics

// CommandMetricsConfig is an alias for MetricsConfig.
type CommandMetricsConfig = MetricsConfig

// NewMetrics builds a recorder. When cfg.Enabled is false it returns (nil, nil).
func NewMetrics(cfg MetricsConfig) (*Metrics, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	meter := cfg.Meter
	if meter == nil {
		meter = otel.Meter("github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws")
	}
	hist, err := meter.Float64Histogram(histogramWSCommandDuration,
		metric.WithDescription("WebSocket command handler round-trip latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("ws command metrics histogram: %w", err)
	}
	clientPingRTT, err := meter.Float64Histogram(histogramWSClientPingRTT,
		metric.WithDescription("Client-reported WebSocket ping/pong round-trip latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("ws client ping rtt histogram: %w", err)
	}
	sessionsActive, err := meter.Int64UpDownCounter(upDownWSSessionsActive,
		metric.WithDescription("Live WebSocket sessions on this dcc-bus daemon"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws sessions active counter: %w", err)
	}
	sessionsClosed, err := meter.Int64Counter(counterWSSessionsClosed,
		metric.WithDescription("WebSocket sessions closed on this dcc-bus daemon"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws sessions closed counter: %w", err)
	}
	deadmanTriggered, err := meter.Int64Counter(counterWSDeadmanTriggered,
		metric.WithDescription("Dead-man switch triggers on this dcc-bus daemon"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws deadman counter: %w", err)
	}
	return &Metrics{
		hist:             hist,
		clientPingRTT:    clientPingRTT,
		sessionsActive:   sessionsActive,
		sessionsClosed:   sessionsClosed,
		deadmanTriggered: deadmanTriggered,
		base: []attribute.KeyValue{
			attribute.Int("layout.id", int(cfg.LayoutID)),
			attribute.Int("command_station.id", int(cfg.CommandStationID)),
		},
	}, nil
}

// NewCommandMetrics is an alias for NewMetrics.
func NewCommandMetrics(cfg CommandMetricsConfig) (*Metrics, error) {
	return NewMetrics(cfg)
}

// Record stores one command observation. No-op when receiver is nil.
func (m *Metrics) Record(command string, o Outcome, dur time.Duration) {
	if m == nil {
		return
	}
	errCode := o.ErrorCode
	if errCode == "" {
		errCode = "_"
	}
	attrs := make([]attribute.KeyValue, 0, len(m.base)+3)
	attrs = append(attrs, m.base...)
	attrs = append(attrs,
		attribute.String("command", command),
		attribute.Bool("success", o.Success),
		attribute.String("error_code", errCode),
	)
	m.hist.Record(context.Background(), dur.Seconds(), metric.WithAttributes(attrs...))
}

// RecordClientPingRTT stores one client-reported ping/pong RTT sample
// (milliseconds on the wire, seconds in the histogram). Login comes
// from the verified WS session, not the ping payload. No-op when
// receiver is nil or latency is out of range.
func (m *Metrics) RecordClientPingRTT(login string, latencyMs float64) {
	if m == nil || latencyMs <= 0 || latencyMs > maxClientPingRTTMs {
		return
	}
	if login == "" {
		login = "_"
	}
	attrs := make([]attribute.KeyValue, 0, len(m.base)+1)
	attrs = append(attrs, m.base...)
	attrs = append(attrs, attribute.String("user.login", login))
	m.clientPingRTT.Record(context.Background(), latencyMs/1000,
		metric.WithAttributes(attrs...))
}

// RecordInvalidEnvelope records a malformed inbound envelope before
// dispatch. No-op when receiver is nil.
func (m *Metrics) RecordInvalidEnvelope() {
	m.Record(commandInvalidEnvelope, Fail(errors.WsCodeBadEnvelope), 0)
}

// RecordSessionOpened increments the active session gauge. Call after the
// dcc-bus.opened welcome frame is sent.
func (m *Metrics) RecordSessionOpened() {
	if m == nil {
		return
	}
	m.sessionsActive.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

// RecordSessionClosed decrements the active session gauge and counts one
// closure with the given reason.
func (m *Metrics) RecordSessionClosed(reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = errors.WsCodeSessionWsClosed
	}
	attrs := make([]attribute.KeyValue, 0, len(m.base)+1)
	attrs = append(attrs, m.base...)
	attrs = append(attrs, attribute.String("reason", reason))
	m.sessionsActive.Add(context.Background(), -1, metric.WithAttributes(m.base...))
	m.sessionsClosed.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordDeadmanTriggered counts one dead-man switch firing.
func (m *Metrics) RecordDeadmanTriggered() {
	if m == nil {
		return
	}
	m.deadmanTriggered.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}
