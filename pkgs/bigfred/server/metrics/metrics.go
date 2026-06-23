// Package metrics records loco-server OpenTelemetry instruments exported
// via OTLP when telemetry is enabled at bootstrap.
package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	histogramWSCommandDuration       = "bigfred.server.ws.command.duration"
	histogramWSClientPingRTT         = "bigfred.server.ws.client_ping_rtt"
	counterWSSessionsClosed          = "bigfred.server.ws.sessions.closed"
	upDownWSSessionsActive           = "bigfred.server.ws.sessions.active"
	counterWSBroadcastDropped        = "bigfred.server.ws.broadcast.dropped"
	counterDccBusProxyUpgrades       = "bigfred.server.dcc_bus.proxy.upgrades"
	upDownDccBusProxySessionsActive  = "bigfred.server.dcc_bus.proxy.sessions.active"
	histogramDccBusProxySession      = "bigfred.server.dcc_bus.proxy.session.duration"
	histogramDccBusEnsureRunning   = "bigfred.server.dcc_bus.ensure_running.duration"
	counterDccBusEnsureRunningErrors = "bigfred.server.dcc_bus.ensure_running.errors"
	gaugeDccBusDaemonsActive         = "bigfred.server.dcc_bus.daemons.active"
	gaugeDccBusPortsAllocated        = "bigfred.server.dcc_bus.ports.allocated"
	histogramHTTPRequestDuration     = "bigfred.server.http.request.duration"
	counterHTTPRequestTotal          = "bigfred.server.http.request.total"
	counterAuthLogin                 = "bigfred.server.auth.login.total"
	counterAuthTokenVerifyErrors     = "bigfred.server.auth.token_verify.errors"
	counterAuthUnauthorized          = "bigfred.server.auth.unauthorized.total"
	counterEStopTriggered            = "bigfred.server.estop.triggered"
	counterRadioStopTriggered        = "bigfred.server.radio_stop.triggered"
	counterTakeoverRequests          = "bigfred.server.takeover.requests"
	gaugePresenceUsersOnline         = "bigfred.server.presence.users.online"
	histogramDBQueryDuration         = "bigfred.server.db.query.duration"
	counterDccBusConsumerEvents      = "bigfred.server.dcc_bus_consumer.events"

	maxClientPingRTTMs = 30_000
)

var latencyBuckets = []float64{
	0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
}

// Config toggles loco-server metric export.
type Config struct {
	Enabled bool
	Meter   metric.Meter
}

// PresenceReader supplies online-user counts per layout for observable gauges.
type PresenceReader interface {
	LayoutOnlineCounts() map[uint]int
}

// DccBusStatsReader supplies dcc-bus orchestration gauges.
type DccBusStatsReader interface {
	AllocatedPortCount() int
}

// Outcome is the observable result of handling one inbound WS command.
type Outcome struct {
	Success   bool
	ErrorCode string
}

// OK returns a successful command outcome.
func OK() Outcome { return Outcome{Success: true} }

// Fail returns a failed command outcome with a machine-readable code.
func Fail(code string) Outcome { return Outcome{Success: false, ErrorCode: code} }

// Metrics records loco-server operational signals.
type Metrics struct {
	wsCommandDuration       metric.Float64Histogram
	wsClientPingRTT         metric.Float64Histogram
	wsSessionsActive        metric.Int64UpDownCounter
	wsSessionsClosed        metric.Int64Counter
	wsBroadcastDropped      metric.Int64Counter
	dccBusProxyUpgrades        metric.Int64Counter
	dccBusProxySessionsActive  metric.Int64UpDownCounter
	dccBusProxySession         metric.Float64Histogram
	dccBusEnsureRunning     metric.Float64Histogram
	dccBusEnsureRunningErrs metric.Int64Counter
	httpRequestDuration     metric.Float64Histogram
	httpRequestTotal        metric.Int64Counter
	authLogin               metric.Int64Counter
	authTokenVerifyErrors   metric.Int64Counter
	authUnauthorized        metric.Int64Counter
	estopTriggered          metric.Int64Counter
	radioStopTriggered      metric.Int64Counter
	takeoverRequests        metric.Int64Counter
	dbQueryDuration         metric.Float64Histogram
	dccBusConsumerEvents    metric.Int64Counter

	presence    PresenceReader
	dccBusStats DccBusStatsReader
}

// New builds a recorder. When cfg.Enabled is false it returns (nil, nil).
func New(cfg Config) (*Metrics, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	meter := cfg.Meter
	if meter == nil {
		meter = otel.Meter("github.com/keskad/loco/pkgs/bigfred/server/metrics")
	}
	m := &Metrics{}
	var err error

	m.wsCommandDuration, err = meter.Float64Histogram(histogramWSCommandDuration,
		metric.WithDescription("Control-plane WebSocket command handler latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("ws command histogram: %w", err)
	}
	m.wsClientPingRTT, err = meter.Float64Histogram(histogramWSClientPingRTT,
		metric.WithDescription("Client-reported control-plane WebSocket ping RTT"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("ws client ping rtt histogram: %w", err)
	}
	m.wsSessionsActive, err = meter.Int64UpDownCounter(upDownWSSessionsActive,
		metric.WithDescription("Live control-plane WebSocket sessions"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws sessions active: %w", err)
	}
	m.wsSessionsClosed, err = meter.Int64Counter(counterWSSessionsClosed,
		metric.WithDescription("Control-plane WebSocket sessions closed"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws sessions closed: %w", err)
	}
	m.wsBroadcastDropped, err = meter.Int64Counter(counterWSBroadcastDropped,
		metric.WithDescription("Control-plane broadcast frames dropped because the outbound buffer was full"),
		metric.WithUnit("{frame}"),
	)
	if err != nil {
		return nil, fmt.Errorf("ws broadcast dropped: %w", err)
	}
	m.dccBusProxyUpgrades, err = meter.Int64Counter(counterDccBusProxyUpgrades,
		metric.WithDescription("dcc-bus reverse-proxy WebSocket upgrade attempts"),
		metric.WithUnit("{upgrade}"),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus proxy upgrades: %w", err)
	}
	m.dccBusProxySessionsActive, err = meter.Int64UpDownCounter(upDownDccBusProxySessionsActive,
		metric.WithDescription("Live dcc-bus reverse-proxy WebSocket sessions"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus proxy sessions active: %w", err)
	}
	m.dccBusProxySession, err = meter.Float64Histogram(histogramDccBusProxySession,
		metric.WithDescription("dcc-bus reverse-proxy WebSocket session lifetime"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus proxy session histogram: %w", err)
	}
	m.dccBusEnsureRunning, err = meter.Float64Histogram(histogramDccBusEnsureRunning,
		metric.WithDescription("dcc-bus EnsureRunning latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus ensure running histogram: %w", err)
	}
	m.dccBusEnsureRunningErrs, err = meter.Int64Counter(counterDccBusEnsureRunningErrors,
		metric.WithDescription("dcc-bus EnsureRunning failures"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus ensure running errors: %w", err)
	}
	m.httpRequestDuration, err = meter.Float64Histogram(histogramHTTPRequestDuration,
		metric.WithDescription("HTTP request latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("http request histogram: %w", err)
	}
	m.httpRequestTotal, err = meter.Int64Counter(counterHTTPRequestTotal,
		metric.WithDescription("HTTP requests served"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("http request counter: %w", err)
	}
	m.authLogin, err = meter.Int64Counter(counterAuthLogin,
		metric.WithDescription("Login attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return nil, fmt.Errorf("auth login counter: %w", err)
	}
	m.authTokenVerifyErrors, err = meter.Int64Counter(counterAuthTokenVerifyErrors,
		metric.WithDescription("JWT verification failures"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, fmt.Errorf("auth token verify counter: %w", err)
	}
	m.authUnauthorized, err = meter.Int64Counter(counterAuthUnauthorized,
		metric.WithDescription("Unauthorized HTTP responses"),
		metric.WithUnit("{response}"),
	)
	if err != nil {
		return nil, fmt.Errorf("auth unauthorized counter: %w", err)
	}
	m.estopTriggered, err = meter.Int64Counter(counterEStopTriggered,
		metric.WithDescription("E-stop target triggers"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("estop counter: %w", err)
	}
	m.radioStopTriggered, err = meter.Int64Counter(counterRadioStopTriggered,
		metric.WithDescription("Radio stop triggers"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("radio stop counter: %w", err)
	}
	m.takeoverRequests, err = meter.Int64Counter(counterTakeoverRequests,
		metric.WithDescription("Takeover lifecycle events"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("takeover counter: %w", err)
	}
	m.dbQueryDuration, err = meter.Float64Histogram(histogramDBQueryDuration,
		metric.WithDescription("SQLite query latency via go-rel"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("db query histogram: %w", err)
	}
	m.dccBusConsumerEvents, err = meter.Int64Counter(counterDccBusConsumerEvents,
		metric.WithDescription("Events received from dcc-bus Redis pub/sub"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("dcc bus consumer counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge(gaugeDccBusDaemonsActive,
		metric.WithDescription("dcc-bus daemons with an allocated port"),
		metric.WithUnit("{daemon}"),
		metric.WithInt64Callback(m.observeDccBusDaemons),
	); err != nil {
		return nil, fmt.Errorf("dcc bus daemons gauge: %w", err)
	}
	if _, err := meter.Int64ObservableGauge(gaugeDccBusPortsAllocated,
		metric.WithDescription("dcc-bus TCP ports allocated from the pool"),
		metric.WithUnit("{port}"),
		metric.WithInt64Callback(m.observeDccBusPorts),
	); err != nil {
		return nil, fmt.Errorf("dcc bus ports gauge: %w", err)
	}
	if _, err := meter.Int64ObservableGauge(gaugePresenceUsersOnline,
		metric.WithDescription("Distinct online users per layout on the control-plane WebSocket"),
		metric.WithUnit("{user}"),
		metric.WithInt64Callback(m.observePresence),
	); err != nil {
		return nil, fmt.Errorf("presence gauge: %w", err)
	}

	return m, nil
}

// SetPresenceReader wires the hub for presence gauges. Safe to call once after bootstrap.
func (m *Metrics) SetPresenceReader(r PresenceReader) {
	if m != nil {
		m.presence = r
	}
}

// SetDccBusStatsReader wires dcc-bus orchestration gauges.
func (m *Metrics) SetDccBusStatsReader(r DccBusStatsReader) {
	if m != nil {
		m.dccBusStats = r
	}
}

func layoutAttr(layoutID uint) attribute.KeyValue {
	return attribute.Int("layout.id", int(layoutID))
}

func csAttr(commandStationID uint) attribute.KeyValue {
	return attribute.Int("command_station.id", int(commandStationID))
}

func (m *Metrics) observeDccBusDaemons(_ context.Context, o metric.Int64Observer) error {
	if m == nil || m.dccBusStats == nil {
		return nil
	}
	o.Observe(int64(m.dccBusStats.AllocatedPortCount()))
	return nil
}

func (m *Metrics) observeDccBusPorts(_ context.Context, o metric.Int64Observer) error {
	if m == nil || m.dccBusStats == nil {
		return nil
	}
	o.Observe(int64(m.dccBusStats.AllocatedPortCount()))
	return nil
}

func (m *Metrics) observePresence(_ context.Context, o metric.Int64Observer) error {
	if m == nil || m.presence == nil {
		return nil
	}
	for layoutID, count := range m.presence.LayoutOnlineCounts() {
		o.Observe(int64(count), metric.WithAttributes(layoutAttr(layoutID)))
	}
	return nil
}

// RecordWSCommand stores one control-plane command observation.
func (m *Metrics) RecordWSCommand(layoutID uint, command string, o Outcome, dur time.Duration) {
	if m == nil {
		return
	}
	errCode := o.ErrorCode
	if errCode == "" {
		errCode = "_"
	}
	m.wsCommandDuration.Record(context.Background(), dur.Seconds(), metric.WithAttributes(
		layoutAttr(layoutID),
		attribute.String("command", command),
		attribute.Bool("success", o.Success),
		attribute.String("error_code", errCode),
	))
}

// RecordWSClientPingRTT stores one client-reported ping RTT sample.
func (m *Metrics) RecordWSClientPingRTT(layoutID uint, login string, latencyMs float64) {
	if m == nil || latencyMs <= 0 || latencyMs > maxClientPingRTTMs {
		return
	}
	if login == "" {
		login = "_"
	}
	m.wsClientPingRTT.Record(context.Background(), latencyMs/1000, metric.WithAttributes(
		layoutAttr(layoutID),
		attribute.String("user.login", login),
	))
}

// RecordWSSessionOpened increments the active session gauge.
func (m *Metrics) RecordWSSessionOpened(layoutID uint) {
	if m == nil {
		return
	}
	m.wsSessionsActive.Add(context.Background(), 1, metric.WithAttributes(layoutAttr(layoutID)))
}

// RecordWSSessionClosed decrements the active gauge and counts one closure.
func (m *Metrics) RecordWSSessionClosed(layoutID uint, reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = "closed"
	}
	attrs := []attribute.KeyValue{layoutAttr(layoutID), attribute.String("reason", reason)}
	m.wsSessionsActive.Add(context.Background(), -1, metric.WithAttributes(layoutAttr(layoutID)))
	m.wsSessionsClosed.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordWSBroadcastDropped counts one dropped outbound frame.
func (m *Metrics) RecordWSBroadcastDropped(layoutID uint, eventType string) {
	if m == nil {
		return
	}
	if eventType == "" {
		eventType = "_"
	}
	m.wsBroadcastDropped.Add(context.Background(), 1, metric.WithAttributes(
		layoutAttr(layoutID),
		attribute.String("event_type", eventType),
	))
}

// RecordDccBusProxyUpgrade counts one reverse-proxy upgrade attempt.
func (m *Metrics) RecordDccBusProxyUpgrade(layoutID, commandStationID uint, success bool, errorCode string) {
	if m == nil {
		return
	}
	if errorCode == "" {
		errorCode = "_"
	}
	m.dccBusProxyUpgrades.Add(context.Background(), 1, metric.WithAttributes(
		layoutAttr(layoutID),
		csAttr(commandStationID),
		attribute.Bool("success", success),
		attribute.String("error_code", errorCode),
	))
}

// RecordDccBusProxySessionOpened increments the live reverse-proxy session gauge.
func (m *Metrics) RecordDccBusProxySessionOpened(layoutID, commandStationID uint) {
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{layoutAttr(layoutID), csAttr(commandStationID)}
	m.dccBusProxySessionsActive.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordDccBusProxySessionClosed decrements the live gauge and records session lifetime.
func (m *Metrics) RecordDccBusProxySessionClosed(layoutID, commandStationID uint, dur time.Duration) {
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{layoutAttr(layoutID), csAttr(commandStationID)}
	m.dccBusProxySessionsActive.Add(context.Background(), -1, metric.WithAttributes(attrs...))
	m.dccBusProxySession.Record(context.Background(), dur.Seconds(), metric.WithAttributes(attrs...))
}

// RecordDccBusEnsureRunning stores EnsureRunning latency and optional error.
func (m *Metrics) RecordDccBusEnsureRunning(layoutID, commandStationID uint, dur time.Duration, err error) {
	if m == nil {
		return
	}
	m.dccBusEnsureRunning.Record(context.Background(), dur.Seconds(), metric.WithAttributes(
		layoutAttr(layoutID),
		csAttr(commandStationID),
	))
	if err != nil {
		code := "ensure_failed"
		m.dccBusEnsureRunningErrs.Add(context.Background(), 1, metric.WithAttributes(
			layoutAttr(layoutID),
			csAttr(commandStationID),
			attribute.String("error_code", code),
		))
	}
}

// RecordHTTPRequest stores one HTTP request.
func (m *Metrics) RecordHTTPRequest(route, method string, status int, dur time.Duration) {
	if m == nil {
		return
	}
	if route == "" {
		route = "_"
	}
	statusClass := strconv.Itoa(status/100) + "xx"
	if status == 0 {
		statusClass = "0xx"
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.route", route),
		attribute.String("http.method", method),
		attribute.String("http.status_class", statusClass),
	}
	m.httpRequestDuration.Record(context.Background(), dur.Seconds(), metric.WithAttributes(attrs...))
	m.httpRequestTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordAuthLogin counts one login attempt.
func (m *Metrics) RecordAuthLogin(success bool) {
	if m == nil {
		return
	}
	m.authLogin.Add(context.Background(), 1, metric.WithAttributes(attribute.Bool("success", success)))
}

// RecordAuthTokenVerifyError counts a JWT verification failure.
func (m *Metrics) RecordAuthTokenVerifyError(reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = "_"
	}
	m.authTokenVerifyErrors.Add(context.Background(), 1, metric.WithAttributes(attribute.String("reason", reason)))
}

// RecordAuthUnauthorized counts an unauthorized response.
func (m *Metrics) RecordAuthUnauthorized(endpoint string) {
	if m == nil {
		return
	}
	if endpoint == "" {
		endpoint = "_"
	}
	m.authUnauthorized.Add(context.Background(), 1, metric.WithAttributes(attribute.String("endpoint", endpoint)))
}

// RecordEStopTriggered counts one e-stop target trigger.
func (m *Metrics) RecordEStopTriggered(layoutID uint) {
	if m == nil {
		return
	}
	m.estopTriggered.Add(context.Background(), 1, metric.WithAttributes(layoutAttr(layoutID)))
}

// RecordRadioStopTriggered counts one radio stop trigger.
func (m *Metrics) RecordRadioStopTriggered(layoutID uint) {
	if m == nil {
		return
	}
	m.radioStopTriggered.Add(context.Background(), 1, metric.WithAttributes(layoutAttr(layoutID)))
}

// RecordTakeover counts one takeover lifecycle event.
func (m *Metrics) RecordTakeover(action string) {
	if m == nil {
		return
	}
	if action == "" {
		action = "_"
	}
	m.takeoverRequests.Add(context.Background(), 1, metric.WithAttributes(attribute.String("action", action)))
}

// RecordDBQuery stores one SQLite query observation.
func (m *Metrics) RecordDBQuery(op string, dur time.Duration, err error) {
	if m == nil {
		return
	}
	if op == "" {
		op = "_"
	}
	m.dbQueryDuration.Record(context.Background(), dur.Seconds(), metric.WithAttributes(
		attribute.String("db.operation", op),
		attribute.Bool("success", err == nil),
	))
}

// RecordDccBusConsumerEvent counts one Redis pub/sub event from a dcc-bus daemon.
func (m *Metrics) RecordDccBusConsumerEvent(eventType string) {
	if m == nil {
		return
	}
	if eventType == "" {
		eventType = "_"
	}
	m.dccBusConsumerEvents.Add(context.Background(), 1, metric.WithAttributes(attribute.String("event_type", eventType)))
}
