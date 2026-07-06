package slotlease

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewMetrics_disabledReturnsNoop(t *testing.T) {
	got, err := NewMetrics(MetricsConfig{Enabled: false})
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	if got != NoopRecorder {
		t.Fatalf("got %T, want NoopRecorder", got)
	}
}

func TestNoopRecorder_noop(t *testing.T) {
	r := NoopRecorder
	r.RecordSelect("ws")
	r.RecordDeselect("ws")
	r.RecordLeased("ws")
	r.RecordReleased(ReleaseSessionClose)
	r.RecordReleaseEstop()
	r.RecordReleasePending()
	r.RecordNoFreeSlot()
	r.RecordBudgetExceeded()
	r.RecordCapEvict()
	r.RecordNotAllowed()
	r.RecordSubscribeCap()
	r.RecordDiagnosticSubscriberOpened()
	r.RecordDiagnosticSubscriberClosed()
	reg, err := r.RegisterGauges(nil)
	if err != nil || reg != nil {
		t.Fatalf("RegisterGauges = %v, %v; want nil, nil", reg, err)
	}
}

func TestRecorderOrNoop_nilUsesNoop(t *testing.T) {
	if RecorderOrNoop(nil) != NoopRecorder {
		t.Fatal("RecorderOrNoop(nil) should return NoopRecorder")
	}
}

func TestMetrics_recordsSelect(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	m, err := NewMetrics(MetricsConfig{
		Enabled:          true,
		LayoutID:         1,
		CommandStationID: 2,
		Meter:            provider.Meter("test"),
	})
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	m.RecordSelect("ws")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(rm.ScopeMetrics) != 1 || len(rm.ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("metrics: %+v", rm.ScopeMetrics)
	}
}
