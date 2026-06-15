package ws

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
)

func TestMetrics_disabledReturnsNil(t *testing.T) {
	got, err := NewMetrics(MetricsConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil metrics when disabled")
	}
}

func TestMetrics_recordsCommand(t *testing.T) {
	reader := metric.NewManualReader()
	m, err := NewMetrics(MetricsConfig{
		Enabled:          true,
		LayoutID:         3,
		CommandStationID: 9,
		Meter:            metric.NewMeterProvider(metric.WithReader(reader)).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Record("loco.setSpeed", OK(), 12*time.Millisecond)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	if len(rm.ScopeMetrics) != 1 || len(rm.ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("metrics: %+v", rm)
	}
	var hist metricdata.Histogram[float64]
	for _, sm := range rm.ScopeMetrics[0].Metrics {
		if sm.Name == histogramWSCommandDuration {
			hist = sm.Data.(metricdata.Histogram[float64])
			break
		}
	}
	if len(hist.DataPoints) != 1 || hist.DataPoints[0].Count != 1 {
		t.Fatalf("histogram datapoints: %+v", hist.DataPoints)
	}
	dp := hist.DataPoints[0]
	if v, ok := dp.Attributes.Value(attribute.Key("command")); !ok || v.AsString() != "loco.setSpeed" {
		t.Fatalf("command attr: %+v ok=%v", v, ok)
	}
}

func TestMetrics_sessionLifecycle(t *testing.T) {
	reader := metric.NewManualReader()
	m, err := NewMetrics(MetricsConfig{
		Enabled:          true,
		LayoutID:         1,
		CommandStationID: 2,
		Meter:            metric.NewMeterProvider(metric.WithReader(reader)).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.RecordSessionOpened()
	m.RecordDeadmanTriggered()
	m.RecordSessionClosed(errors.WsCodeSessionDeadman)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	var active int64
	var closed int64
	var deadman int64
	for _, sm := range rm.ScopeMetrics[0].Metrics {
		switch sm.Name {
		case upDownWSSessionsActive:
			active = sm.Data.(metricdata.Sum[int64]).DataPoints[0].Value
		case counterWSSessionsClosed:
			closed = sm.Data.(metricdata.Sum[int64]).DataPoints[0].Value
		case counterWSDeadmanTriggered:
			deadman = sm.Data.(metricdata.Sum[int64]).DataPoints[0].Value
		}
	}
	if active != 0 {
		t.Fatalf("active sessions = %d, want 0", active)
	}
	if closed != 1 {
		t.Fatalf("closed sessions = %d, want 1", closed)
	}
	if deadman != 1 {
		t.Fatalf("deadman triggers = %d, want 1", deadman)
	}
}

func TestMetrics_nilReceiverNoop(t *testing.T) {
	var m *Metrics
	m.Record("ping", OK(), time.Millisecond)
	m.RecordInvalidEnvelope()
	m.RecordSessionOpened()
	m.RecordSessionClosed("ws_closed")
	m.RecordDeadmanTriggered()
}
