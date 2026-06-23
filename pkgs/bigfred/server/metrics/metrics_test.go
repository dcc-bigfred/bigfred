package metrics

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNew_disabledReturnsNil(t *testing.T) {
	m, err := New(Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil metrics when disabled")
	}
}

func TestRecordWSCommand(t *testing.T) {
	reader := metric.NewManualReader()
	m, err := New(Config{
		Enabled: true,
		Meter:   metric.NewMeterProvider(metric.WithReader(reader)).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.RecordWSCommand(1, "session.setCommandStation", OK(), 12*time.Millisecond)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	if len(rm.ScopeMetrics) == 0 || len(rm.ScopeMetrics[0].Metrics) == 0 {
		t.Fatal("expected histogram data")
	}
}

func TestRecordAuthLogin(t *testing.T) {
	reader := metric.NewManualReader()
	m, err := New(Config{
		Enabled: true,
		Meter:   metric.NewMeterProvider(metric.WithReader(reader)).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.RecordAuthLogin(true)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			if met.Name == counterAuthLogin {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("missing %s", counterAuthLogin)
	}
}

func TestObservePresence(t *testing.T) {
	reader := metric.NewManualReader()
	m, err := New(Config{
		Enabled: true,
		Meter:   metric.NewMeterProvider(metric.WithReader(reader)).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.SetPresenceReader(presenceStub{counts: map[uint]int{3: 2}})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
}

type presenceStub struct {
	counts map[uint]int
}

func (p presenceStub) LayoutOnlineCounts() map[uint]int { return p.counts }
