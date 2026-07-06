package station

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const histogramName = "bigfred.dcc_bus.station.operation.duration"

// InstrumentConfig toggles and labels the command-station latency
// histogram. When Enabled is false, Wrap returns the inner driver
// unchanged.
type InstrumentConfig struct {
	Enabled          bool
	LayoutID         uint
	CommandStationID uint
	Kind             domain.CommandStationKind
	SpeedSteps       uint
	Meter            metric.Meter
}

type instrumented struct {
	inner      commandstation.Station
	hist       metric.Float64Histogram
	base       []attribute.KeyValue
	speedSteps uint8
}

// Wrap decorates inner with a latency histogram when cfg.Enabled.
// Meter defaults to the global provider when nil.
func Wrap(inner commandstation.Station, cfg InstrumentConfig) (commandstation.Station, error) {
	if !cfg.Enabled || inner == nil {
		return inner, nil
	}
	meter := cfg.Meter
	if meter == nil {
		meter = otel.Meter("github.com/keskad/loco/pkgs/bigfred/dcc-bus/station")
	}
	hist, err := meter.Float64Histogram(histogramName,
		metric.WithDescription("Command station driver round-trip latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("station metrics histogram: %w", err)
	}
	speedSteps := cfg.SpeedSteps
	if speedSteps == 0 {
		speedSteps = 128
	}
	return &instrumented{
		inner: inner,
		hist:  hist,
		speedSteps: uint8(speedSteps),
		base: []attribute.KeyValue{
			attribute.String("station.kind", string(cfg.Kind)),
			attribute.Int("layout.id", int(cfg.LayoutID)),
			attribute.Int("command_station.id", int(cfg.CommandStationID)),
		},
	}, nil
}

func (w *instrumented) Inner() commandstation.Station {
	return w.inner
}

func (w *instrumented) WriteCV(mode commandstation.Mode, lcv commandstation.LocoCV, options ...commandstation.Option) error {
	addr := uint16(lcv.LocoId)
	return w.observe("write_cv", &addr, func() error {
		return w.inner.WriteCV(mode, lcv, options...)
	})
}

func (w *instrumented) ReadCV(mode commandstation.Mode, lcv commandstation.LocoCV, options ...commandstation.Option) (int, error) {
	addr := uint16(lcv.LocoId)
	return w.observeInt("read_cv", &addr, func() (int, error) {
		return w.inner.ReadCV(mode, lcv, options...)
	})
}

func (w *instrumented) SendFn(mode commandstation.Mode, addr commandstation.LocoAddr, num commandstation.FuncNum, toggle bool) error {
	a := uint16(addr)
	return w.observe("send_fn", &a, func() error {
		return w.inner.SendFn(mode, addr, num, toggle)
	})
}

func (w *instrumented) ListFunctions(addr commandstation.LocoAddr) ([]int, error) {
	a := uint16(addr)
	return w.observeSlice("list_functions", &a, func() ([]int, error) {
		return w.inner.ListFunctions(addr)
	})
}

func (w *instrumented) SetSpeed(addr commandstation.LocoAddr, speed uint8, forward bool, speedSteps uint8) error {
	a := uint16(addr)
	return w.observe("set_speed", &a, func() error {
		return w.inner.SetSpeed(addr, speed, forward, speedSteps)
	})
}

func (w *instrumented) EmergencyStop(addr commandstation.LocoAddr, forward bool) error {
	a := uint16(addr)
	return w.observe("emergency_stop", &a, func() error {
		if estopper, ok := w.inner.(commandstation.EmergencyStopper); ok {
			return estopper.EmergencyStop(addr, forward)
		}
		return w.inner.SetSpeed(addr, 1, forward, w.speedSteps)
	})
}

func (w *instrumented) GetSpeed(addr commandstation.LocoAddr) (uint8, bool, error) {
	a := uint16(addr)
	return w.observeSpeed("get_speed", &a, func() (uint8, bool, error) {
		return w.inner.GetSpeed(addr)
	})
}

func (w *instrumented) CleanUp() error {
	return w.observe("clean_up", nil, func() error {
		return w.inner.CleanUp()
	})
}

func (w *instrumented) observe(op string, locoAddr *uint16, fn func() error) error {
	start := time.Now()
	err := fn()
	w.record(op, time.Since(start), err, locoAddr)
	return err
}

func (w *instrumented) observeInt(op string, locoAddr *uint16, fn func() (int, error)) (int, error) {
	start := time.Now()
	v, err := fn()
	w.record(op, time.Since(start), err, locoAddr)
	return v, err
}

func (w *instrumented) observeSlice(op string, locoAddr *uint16, fn func() ([]int, error)) ([]int, error) {
	start := time.Now()
	v, err := fn()
	w.record(op, time.Since(start), err, locoAddr)
	return v, err
}

func (w *instrumented) observeSpeed(op string, locoAddr *uint16, fn func() (uint8, bool, error)) (uint8, bool, error) {
	start := time.Now()
	speed, forward, err := fn()
	w.record(op, time.Since(start), err, locoAddr)
	return speed, forward, err
}

func (w *instrumented) record(op string, dur time.Duration, err error, locoAddr *uint16) {
	attrs := make([]attribute.KeyValue, 0, len(w.base)+3)
	attrs = append(attrs, w.base...)
	attrs = append(attrs,
		attribute.String("operation", op),
		attribute.Bool("success", err == nil),
	)
	if locoAddr != nil {
		attrs = append(attrs, attribute.Int("loco.addr", int(*locoAddr)))
	}
	w.hist.Record(context.Background(), dur.Seconds(), metric.WithAttributes(attrs...))
}
