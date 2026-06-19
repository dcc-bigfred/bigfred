package station

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type fakeMetricsSource struct {
	snap commandstation.LnMetricsSnapshot
}

func (f *fakeMetricsSource) MetricsSnapshot() commandstation.LnMetricsSnapshot { return f.snap }

func TestStartLocoNetMetrics_nilSource(t *testing.T) {
	reg, err := StartLocoNetMetrics(nil, LocoNetMetricsConfig{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if reg != nil {
		t.Fatalf("reg = %v, want nil for nil source", reg)
	}
}

func TestStartLocoNetMetrics_observesSnapshot(t *testing.T) {
	src := &fakeMetricsSource{snap: commandstation.LnMetricsSnapshot{
		TxFrames:     12,
		RxFrames:     8,
		TxBytes:      48,
		RxBytes:      20,
		TxByOpcode:   map[byte]uint64{0xA0: 5}, // LOCO_SPD
		RxByOpcode:   map[byte]uint64{0xE7: 2}, // SL_RD_DATA
		TxCoalesced:  3,
		BadChecksum:  1,
		SlotAcquires: 4,
		SlotsActive:  6,
		RxQueueLen:   2,
	}}

	reader := metric.NewManualReader()
	meter := metric.NewMeterProvider(metric.WithReader(reader)).Meter("test")

	reg, err := StartLocoNetMetrics(src, LocoNetMetricsConfig{
		LayoutID:         7,
		CommandStationID: 2,
		Kind:             domain.CommandStationKindLocoNetSerial,
		Meter:            meter,
	})
	if err != nil {
		t.Fatalf("StartLocoNetMetrics: %v", err)
	}
	if reg == nil {
		t.Fatal("reg is nil")
	}
	t.Cleanup(func() { _ = reg.Unregister() })

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	metrics := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			metrics[m.Name] = m
		}
	}

	// frames counter has a tx datapoint of 12.
	if got := sumInt64ForAttr(t, metrics[mFrames], "direction", "tx"); got != 12 {
		t.Fatalf("%s tx = %d, want 12", mFrames, got)
	}
	if got := sumInt64ForAttr(t, metrics[mFrames], "direction", "rx"); got != 8 {
		t.Fatalf("%s rx = %d, want 8", mFrames, got)
	}
	// per-opcode messages: tx LOCO_SPD labelled "loco_spd".
	if got := sumInt64ForAttr(t, metrics[mMessages], "opcode", "loco_spd"); got != 5 {
		t.Fatalf("%s loco_spd = %d, want 5", mMessages, got)
	}
	// errors counter, kind=bad_checksum.
	if got := sumInt64ForAttr(t, metrics[mErrors], "kind", "bad_checksum"); got != 1 {
		t.Fatalf("%s bad_checksum = %d, want 1", mErrors, got)
	}
	// slot_ops, op=acquire.
	if got := sumInt64ForAttr(t, metrics[mSlotOps], "op", "acquire"); got != 4 {
		t.Fatalf("%s acquire = %d, want 4", mSlotOps, got)
	}
	// slots.active gauge = 6.
	if got := sumInt64Gauge(t, metrics[mSlotsActive]); got != 6 {
		t.Fatalf("%s = %d, want 6", mSlotsActive, got)
	}
}

func sumInt64ForAttr(t *testing.T, m metricdata.Metrics, key, val string) int64 {
	t.Helper()
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("metric %q is not an int64 sum: %T", m.Name, m.Data)
	}
	var total int64
	for _, dp := range sum.DataPoints {
		if v, ok := dp.Attributes.Value(attribute.Key(key)); ok && v.AsString() == val {
			total += dp.Value
		}
	}
	return total
}

func sumInt64Gauge(t *testing.T, m metricdata.Metrics) int64 {
	t.Helper()
	g, ok := m.Data.(metricdata.Gauge[int64])
	if !ok {
		t.Fatalf("metric %q is not an int64 gauge: %T", m.Name, m.Data)
	}
	var total int64
	for _, dp := range g.DataPoints {
		total += dp.Value
	}
	return total
}
