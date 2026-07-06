package slotlease

import (
	"context"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	counterSlotLeased              = "bigfred.dcc_bus.slot.leased"
	counterSlotReleased            = "bigfred.dcc_bus.slot.released"
	counterSlotReleaseEstop        = "bigfred.dcc_bus.slot.release_estop"
	counterSlotReleasePending      = "bigfred.dcc_bus.slot.release_pending"
	counterSlotNoFreeSlot          = "bigfred.dcc_bus.slot.no_free_slot"
	counterSlotBudgetExceeded      = "bigfred.dcc_bus.slot.budget_exceeded"
	counterSlotCapEvict            = "bigfred.dcc_bus.slot.cap_evict"
	counterSlotNotAllowed          = "bigfred.dcc_bus.slot.not_allowed"
	counterSlotSelect              = "bigfred.dcc_bus.slot.select"
	counterSlotDeselect            = "bigfred.dcc_bus.slot.deselect"
	counterLocoSubscribeCap        = "bigfred.dcc_bus.loco.subscribe_cap"
	gaugeSlotActive                = "bigfred.dcc_bus.slot.active"
	gaugeSlotBudgetUsed            = "bigfred.dcc_bus.slot.budget_used"
	gaugeSlotDiagnosticSubscribers = "bigfred.dcc_bus.slot.diagnostic_subscribers"
)

// MetricsConfig toggles slot-lease OTel export.
type MetricsConfig struct {
	Enabled          bool
	LayoutID         uint
	CommandStationID uint
	Meter            metric.Meter
}

// Recorder exports slot-lease OpenTelemetry counters and gauges. Callers
// should always hold a non-nil Recorder — use NoopRecorder when export is off.
type Recorder interface {
	RegisterGauges(leaser *Leaser) (metric.Registration, error)
	RecordSelect(source string)
	RecordDeselect(source string)
	RecordLeased(source string)
	RecordReleased(reason ReleaseReason)
	RecordReleaseEstop()
	RecordReleasePending()
	RecordNoFreeSlot()
	RecordBudgetExceeded()
	RecordCapEvict()
	RecordNotAllowed()
	RecordSubscribeCap()
	RecordDiagnosticSubscriberOpened()
	RecordDiagnosticSubscriberClosed()
}

type noopRecorder struct{}

// NoopRecorder is a Recorder that discards all events.
var NoopRecorder Recorder = noopRecorder{}

func (noopRecorder) RegisterGauges(*Leaser) (metric.Registration, error) {
	return nil, nil
}
func (noopRecorder) RecordSelect(string)                          {}
func (noopRecorder) RecordDeselect(string)                        {}
func (noopRecorder) RecordLeased(string)                          {}
func (noopRecorder) RecordReleased(ReleaseReason)                 {}
func (noopRecorder) RecordReleaseEstop()                          {}
func (noopRecorder) RecordReleasePending()                        {}
func (noopRecorder) RecordNoFreeSlot()                            {}
func (noopRecorder) RecordBudgetExceeded()                        {}
func (noopRecorder) RecordCapEvict()                              {}
func (noopRecorder) RecordNotAllowed()                            {}
func (noopRecorder) RecordSubscribeCap()                          {}
func (noopRecorder) RecordDiagnosticSubscriberOpened()            {}
func (noopRecorder) RecordDiagnosticSubscriberClosed()            {}

func recorderOrNoop(r Recorder) Recorder {
	if r == nil {
		return NoopRecorder
	}
	return r
}

// RecorderOrNoop is an alias for recorderOrNoop for callers outside this package.
func RecorderOrNoop(r Recorder) Recorder { return recorderOrNoop(r) }

// Metrics records slot lease lifecycle and subscription-cap drops.
type Metrics struct {
	leased              metric.Int64Counter
	released            metric.Int64Counter
	releaseEstop        metric.Int64Counter
	releasePending      metric.Int64Counter
	noFreeSlot          metric.Int64Counter
	budgetExceeded      metric.Int64Counter
	capEvict            metric.Int64Counter
	notAllowed          metric.Int64Counter
	selectTotal         metric.Int64Counter
	deselectTotal       metric.Int64Counter
	subscribeCap        metric.Int64Counter
	active              metric.Int64ObservableGauge
	budgetUsed          metric.Int64ObservableGauge
	diagnosticSubs      metric.Int64UpDownCounter
	base                []attribute.KeyValue
	gaugeLeaser         *Leaser
	diagSubs            atomic.Int64
	meter               metric.Meter
}

// NewMetrics builds a recorder. When cfg.Enabled is false it returns
// NoopRecorder.
func NewMetrics(cfg MetricsConfig) (Recorder, error) {
	if !cfg.Enabled {
		return NoopRecorder, nil
	}
	meter := cfg.Meter
	if meter == nil {
		meter = otel.Meter("github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease")
	}
	base := []attribute.KeyValue{
		attribute.Int("layout.id", int(cfg.LayoutID)),
		attribute.Int("command_station.id", int(cfg.CommandStationID)),
	}
	m := &Metrics{base: base, meter: meter}

	var err error
	if m.leased, err = meter.Int64Counter(counterSlotLeased,
		metric.WithDescription("Command-station slots acquired by the leaser"),
		metric.WithUnit("{slot}"),
	); err != nil {
		return nil, fmt.Errorf("slot leased counter: %w", err)
	}
	if m.released, err = meter.Int64Counter(counterSlotReleased,
		metric.WithDescription("Command-station slots released by the leaser"),
		metric.WithUnit("{slot}"),
	); err != nil {
		return nil, fmt.Errorf("slot released counter: %w", err)
	}
	if m.releaseEstop, err = meter.Int64Counter(counterSlotReleaseEstop,
		metric.WithDescription("E-stop-then-release sequences before slot release"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot release estop counter: %w", err)
	}
	if m.releasePending, err = meter.Int64Counter(counterSlotReleasePending,
		metric.WithDescription("ReleaseSlot write failures queued for retry"),
		metric.WithUnit("{slot}"),
	); err != nil {
		return nil, fmt.Errorf("slot release pending counter: %w", err)
	}
	if m.noFreeSlot, err = meter.Int64Counter(counterSlotNoFreeSlot,
		metric.WithDescription("AcquireSlot rejections when the command station is full"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot no free slot counter: %w", err)
	}
	if m.budgetExceeded, err = meter.Int64Counter(counterSlotBudgetExceeded,
		metric.WithDescription("Select rejections when max_loconet_slots is exhausted"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot budget exceeded counter: %w", err)
	}
	if m.capEvict, err = meter.Int64Counter(counterSlotCapEvict,
		metric.WithDescription("Per-user vehicle-cap evictions during Select"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot cap evict counter: %w", err)
	}
	if m.notAllowed, err = meter.Int64Counter(counterSlotNotAllowed,
		metric.WithDescription("Select rejections from the CanDrive gate"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot not allowed counter: %w", err)
	}
	if m.selectTotal, err = meter.Int64Counter(counterSlotSelect,
		metric.WithDescription("loco.select / remote drive Select calls"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot select counter: %w", err)
	}
	if m.deselectTotal, err = meter.Int64Counter(counterSlotDeselect,
		metric.WithDescription("loco.deselect / holder drop calls"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("slot deselect counter: %w", err)
	}
	if m.subscribeCap, err = meter.Int64Counter(counterLocoSubscribeCap,
		metric.WithDescription("Subscription-cap drops (D16)"),
		metric.WithUnit("{event}"),
	); err != nil {
		return nil, fmt.Errorf("loco subscribe cap counter: %w", err)
	}
	if m.active, err = meter.Int64ObservableGauge(gaugeSlotActive,
		metric.WithDescription("Active slot leases with at least one holder of the given source"),
		metric.WithUnit("{slot}"),
	); err != nil {
		return nil, fmt.Errorf("slot active gauge: %w", err)
	}
	if m.budgetUsed, err = meter.Int64ObservableGauge(gaugeSlotBudgetUsed,
		metric.WithDescription("Total slot leases held by BigFred"),
		metric.WithUnit("{slot}"),
	); err != nil {
		return nil, fmt.Errorf("slot budget used gauge: %w", err)
	}
	if m.diagnosticSubs, err = meter.Int64UpDownCounter(gaugeSlotDiagnosticSubscribers,
		metric.WithDescription("Live admin slot-diagnostic WebSocket clients"),
		metric.WithUnit("{client}"),
	); err != nil {
		return nil, fmt.Errorf("slot diagnostic subscribers gauge: %w", err)
	}
	return m, nil
}

// RegisterGauges wires observable gauges to leaser state. Call once after the
// leaser is constructed.
func (m *Metrics) RegisterGauges(leaser *Leaser) (metric.Registration, error) {
	m.gaugeLeaser = leaser
	return m.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		if m.gaugeLeaser == nil {
			return nil
		}
		used, bySource := m.gaugeLeaser.gaugeSnapshot()
		o.ObserveInt64(m.budgetUsed, int64(used), metric.WithAttributes(m.base...))
		for src, n := range bySource {
			attrs := append([]attribute.KeyValue{}, m.base...)
			attrs = append(attrs, attribute.String("source", src))
			o.ObserveInt64(m.active, int64(n), metric.WithAttributes(attrs...))
		}
		return nil
	}, m.active, m.budgetUsed)
}

func (m *Metrics) RecordSelect(source string) {
	if source == "" {
		source = "_"
	}
	attrs := append([]attribute.KeyValue{}, m.base...)
	attrs = append(attrs, attribute.String("source", source))
	m.selectTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

func (m *Metrics) RecordDeselect(source string) {
	if source == "" {
		source = "_"
	}
	attrs := append([]attribute.KeyValue{}, m.base...)
	attrs = append(attrs, attribute.String("source", source))
	m.deselectTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

func (m *Metrics) RecordLeased(source string) {
	if source == "" {
		source = "_"
	}
	attrs := append([]attribute.KeyValue{}, m.base...)
	attrs = append(attrs, attribute.String("source", source))
	m.leased.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

func (m *Metrics) RecordReleased(reason ReleaseReason) {
	r := string(reason)
	if r == "" {
		r = "_"
	}
	attrs := append([]attribute.KeyValue{}, m.base...)
	attrs = append(attrs, attribute.String("reason", r))
	m.released.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

func (m *Metrics) RecordReleaseEstop() {
	m.releaseEstop.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordReleasePending() {
	m.releasePending.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordNoFreeSlot() {
	m.noFreeSlot.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordBudgetExceeded() {
	m.budgetExceeded.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordCapEvict() {
	m.capEvict.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordNotAllowed() {
	m.notAllowed.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordSubscribeCap() {
	m.subscribeCap.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordDiagnosticSubscriberOpened() {
	m.diagSubs.Add(1)
	m.diagnosticSubs.Add(context.Background(), 1, metric.WithAttributes(m.base...))
}

func (m *Metrics) RecordDiagnosticSubscriberClosed() {
	if m.diagSubs.Add(-1) < 0 {
		// Clamp: never let the exported counter go negative if a close
		// races ahead of an open (shouldn't happen, but stay safe).
		m.diagSubs.Store(0)
		return
	}
	m.diagnosticSubs.Add(context.Background(), -1, metric.WithAttributes(m.base...))
}
