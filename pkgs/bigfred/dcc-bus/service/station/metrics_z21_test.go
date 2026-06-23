package station

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type fakeZ21MetricsSource struct {
	snap commandstation.Z21MetricsSnapshot
}

func (f *fakeZ21MetricsSource) Z21MetricsSnapshot() commandstation.Z21MetricsSnapshot { return f.snap }

func TestStartZ21Metrics_nilSource(t *testing.T) {
	reg, err := StartZ21Metrics(nil, Z21MetricsConfig{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if reg != nil {
		t.Fatalf("reg = %v, want nil for nil source", reg)
	}
}

func TestStartZ21Metrics_observesSnapshot(t *testing.T) {
	src := &fakeZ21MetricsSource{snap: commandstation.Z21MetricsSnapshot{
		TxPackets:      10,
		RxPackets:      6,
		TxBytes:        80,
		RxBytes:        48,
		TxByType:       map[byte]uint64{0xE4: 3},
		RxByType:       map[byte]uint64{0xEF: 2},
		ObsDropped:     1,
		SyncTimeouts:   2,
		CvNacks:        4,
		FnCacheEntries: 5,
		ObsQueueLen:    3,
	}}

	reader := metric.NewManualReader()
	meter := metric.NewMeterProvider(metric.WithReader(reader)).Meter("test")

	reg, err := StartZ21Metrics(src, Z21MetricsConfig{
		LayoutID:         7,
		CommandStationID: 2,
		Kind:             domain.CommandStationKindZ21,
		Meter:            meter,
	})
	if err != nil {
		t.Fatalf("StartZ21Metrics: %v", err)
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

	if got := sumInt64ForAttr(t, metrics[mZ21Packets], "direction", "tx"); got != 10 {
		t.Fatalf("%s tx = %d, want 10", mZ21Packets, got)
	}
	if got := sumInt64ForAttr(t, metrics[mZ21Packets], "direction", "rx"); got != 6 {
		t.Fatalf("%s rx = %d, want 6", mZ21Packets, got)
	}
	if got := sumInt64ForAttr(t, metrics[mZ21Messages], "type", "set_loco"); got != 3 {
		t.Fatalf("%s set_loco = %d, want 3", mZ21Messages, got)
	}
	if got := sumInt64ForAttr(t, metrics[mZ21Errors], "kind", "sync_timeout"); got != 2 {
		t.Fatalf("%s sync_timeout = %d, want 2", mZ21Errors, got)
	}
	if got := sumInt64ForAttr(t, metrics[mZ21CVResults], "kind", "nack"); got != 4 {
		t.Fatalf("%s nack = %d, want 4", mZ21CVResults, got)
	}
	if got := sumInt64Gauge(t, metrics[mZ21FnCache]); got != 5 {
		t.Fatalf("%s = %d, want 5", mZ21FnCache, got)
	}
}
