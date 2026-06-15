package station

import (
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type fakeStation struct {
	setSpeedDelay time.Duration
	setSpeedCalls int
}

func (f *fakeStation) WriteCV(commandstation.Mode, commandstation.LocoCV, ...commandstation.Option) error {
	return nil
}
func (f *fakeStation) ReadCV(commandstation.Mode, commandstation.LocoCV, ...commandstation.Option) (int, error) {
	return 0, nil
}
func (f *fakeStation) SendFn(commandstation.Mode, commandstation.LocoAddr, commandstation.FuncNum, bool) error {
	return nil
}
func (f *fakeStation) ListFunctions(commandstation.LocoAddr) ([]int, error) {
	return nil, nil
}
func (f *fakeStation) SetSpeed(commandstation.LocoAddr, uint8, bool, uint8) error {
	f.setSpeedCalls++
	if f.setSpeedDelay > 0 {
		time.Sleep(f.setSpeedDelay)
	}
	return nil
}
func (f *fakeStation) GetSpeed(commandstation.LocoAddr) (uint8, bool, error) {
	return 0, true, nil
}
func (f *fakeStation) CleanUp() error { return nil }

func (f *fakeStation) ObserveStates() <-chan commandstation.LocoObservation {
	ch := make(chan commandstation.LocoObservation)
	close(ch)
	return ch
}

func TestWrap_disabledReturnsInner(t *testing.T) {
	inner := &fakeStation{}
	got, err := Wrap(inner, InstrumentConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if got != inner {
		t.Fatal("expected same instance when disabled")
	}
}

func TestWrap_recordsSetSpeedHistogram(t *testing.T) {
	inner := &fakeStation{setSpeedDelay: 2 * time.Millisecond}
	reader := metric.NewManualReader()
	meter := metric.NewMeterProvider(metric.WithReader(reader)).Meter("test")

	wrapped, err := Wrap(inner, InstrumentConfig{
		Enabled:          true,
		LayoutID:         7,
		CommandStationID: 2,
		Kind:             domain.CommandStationKindZ21,
		Meter:            meter,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := wrapped.SetSpeed(3, 10, true, 128); err != nil {
		t.Fatal(err)
	}
	if inner.setSpeedCalls != 1 {
		t.Fatalf("setSpeedCalls = %d", inner.setSpeedCalls)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	if len(rm.ScopeMetrics) != 1 || len(rm.ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("metrics: %+v", rm)
	}
	hist := rm.ScopeMetrics[0].Metrics[0].Data.(metricdata.Histogram[float64])
	if len(hist.DataPoints) != 1 || hist.DataPoints[0].Count != 1 {
		t.Fatalf("histogram datapoints: %+v", hist.DataPoints)
	}
	if v, ok := hist.DataPoints[0].Attributes.Value(attribute.Key("operation")); !ok || v.AsString() != "set_speed" {
		t.Fatalf("operation attr: %+v ok=%v", v, ok)
	}
}

func TestAsStateObserver_unwrapsDecorator(t *testing.T) {
	inner := &fakeStation{}
	wrapped, err := Wrap(inner, InstrumentConfig{
		Enabled: true,
		Kind:    domain.CommandStationKindZ21,
		Meter:   metric.NewMeterProvider(metric.WithReader(metric.NewManualReader())).Meter("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	obs, ok := AsStateObserver(wrapped)
	if !ok {
		t.Fatal("expected StateObserver through wrapper")
	}
	if obs == nil {
		t.Fatal("nil observer")
	}
}
