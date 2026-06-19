package station

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// LocoNet metric names. Kept under the same bigfred.dcc_bus.* namespace as the
// WS and station-latency instruments so dashboards can group by daemon.
const (
	mFrames      = "bigfred.dcc_bus.loconet.frames"
	mBytes       = "bigfred.dcc_bus.loconet.bytes"
	mMessages    = "bigfred.dcc_bus.loconet.messages"
	mPaceWait    = "bigfred.dcc_bus.loconet.pace_wait.seconds"
	mCoalesced   = "bigfred.dcc_bus.loconet.coalesced"
	mDropped     = "bigfred.dcc_bus.loconet.dropped"
	mErrors      = "bigfred.dcc_bus.loconet.errors"
	mSlotOps     = "bigfred.dcc_bus.loconet.slot_ops"
	mSlotsActive = "bigfred.dcc_bus.loconet.slots.active"
	mQueueDepth  = "bigfred.dcc_bus.loconet.queue.depth"
)

// LocoNetMetricsConfig labels the LocoNet telemetry instruments.
type LocoNetMetricsConfig struct {
	LayoutID         uint
	CommandStationID uint
	Kind             domain.CommandStationKind
	Meter            metric.Meter
}

// StartLocoNetMetrics registers OpenTelemetry asynchronous instruments that, at
// each collection, read a snapshot from src and report it. Using observable
// instruments means there is no background goroutine and no flush timer: the
// OTLP reader's own export cadence drives collection, and a single registered
// callback reads one consistent snapshot per cycle.
//
// All OpenTelemetry usage lives here, never in the LocoNet driver: the driver
// only bumps lock-free atomic counters (commandstation.LnMetricsSnapshot),
// keeping the hot path allocation-free and telemetry-agnostic.
//
// The returned Registration must be unregistered on shutdown. When src is nil
// it returns (nil, nil) so callers can wire it unconditionally.
func StartLocoNetMetrics(src commandstation.MetricsSource, cfg LocoNetMetricsConfig) (metric.Registration, error) {
	if src == nil {
		return nil, nil
	}
	meter := cfg.Meter
	if meter == nil {
		meter = otel.Meter("github.com/keskad/loco/pkgs/bigfred/dcc-bus/station")
	}
	base := []attribute.KeyValue{
		attribute.String("station.kind", string(cfg.Kind)),
		attribute.Int("layout.id", int(cfg.LayoutID)),
		attribute.Int("command_station.id", int(cfg.CommandStationID)),
	}

	frames, err := meter.Int64ObservableCounter(mFrames,
		metric.WithDescription("LocoNet frames transferred, by direction"),
		metric.WithUnit("{frame}"))
	if err != nil {
		return nil, fmt.Errorf("loconet frames counter: %w", err)
	}
	bytesC, err := meter.Int64ObservableCounter(mBytes,
		metric.WithDescription("LocoNet bytes transferred, by direction"),
		metric.WithUnit("By"))
	if err != nil {
		return nil, fmt.Errorf("loconet bytes counter: %w", err)
	}
	messages, err := meter.Int64ObservableCounter(mMessages,
		metric.WithDescription("LocoNet messages by opcode and direction"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, fmt.Errorf("loconet messages counter: %w", err)
	}
	paceWait, err := meter.Float64ObservableCounter(mPaceWait,
		metric.WithDescription("Cumulative time the TX pacer blocked (bus saturation indicator)"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("loconet pace_wait counter: %w", err)
	}
	coalesced, err := meter.Int64ObservableCounter(mCoalesced,
		metric.WithDescription("Speed frames dropped because a newer SetSpeed superseded them"),
		metric.WithUnit("{frame}"))
	if err != nil {
		return nil, fmt.Errorf("loconet coalesced counter: %w", err)
	}
	dropped, err := meter.Int64ObservableCounter(mDropped,
		metric.WithDescription("Internal queue overflows, by queue"),
		metric.WithUnit("{event}"))
	if err != nil {
		return nil, fmt.Errorf("loconet dropped counter: %w", err)
	}
	errorsC, err := meter.Int64ObservableCounter(mErrors,
		metric.WithDescription("LocoNet errors, by kind"),
		metric.WithUnit("{error}"))
	if err != nil {
		return nil, fmt.Errorf("loconet errors counter: %w", err)
	}
	slotOps, err := meter.Int64ObservableCounter(mSlotOps,
		metric.WithDescription("LocoNet slot lifecycle operations, by op"),
		metric.WithUnit("{operation}"))
	if err != nil {
		return nil, fmt.Errorf("loconet slot_ops counter: %w", err)
	}
	slotsActive, err := meter.Int64ObservableGauge(mSlotsActive,
		metric.WithDescription("LocoNet slots currently owned by this daemon"),
		metric.WithUnit("{slot}"))
	if err != nil {
		return nil, fmt.Errorf("loconet slots_active gauge: %w", err)
	}
	queueDepth, err := meter.Int64ObservableGauge(mQueueDepth,
		metric.WithDescription("Internal channel occupancy, by queue"),
		metric.WithUnit("{item}"))
	if err != nil {
		return nil, fmt.Errorf("loconet queue_depth gauge: %w", err)
	}

	withBase := func(extra ...attribute.KeyValue) metric.ObserveOption {
		attrs := make([]attribute.KeyValue, 0, len(base)+len(extra))
		attrs = append(attrs, base...)
		attrs = append(attrs, extra...)
		return metric.WithAttributes(attrs...)
	}

	cb := func(_ context.Context, o metric.Observer) error {
		s := src.MetricsSnapshot()

		o.ObserveInt64(frames, int64(s.TxFrames), withBase(attribute.String("direction", "tx")))
		o.ObserveInt64(frames, int64(s.RxFrames), withBase(attribute.String("direction", "rx")))
		o.ObserveInt64(bytesC, int64(s.TxBytes), withBase(attribute.String("direction", "tx")))
		o.ObserveInt64(bytesC, int64(s.RxBytes), withBase(attribute.String("direction", "rx")))

		for op, n := range s.TxByOpcode {
			o.ObserveInt64(messages, int64(n),
				withBase(attribute.String("direction", "tx"), attribute.String("opcode", commandstation.LnOpcodeName(op))))
		}
		for op, n := range s.RxByOpcode {
			o.ObserveInt64(messages, int64(n),
				withBase(attribute.String("direction", "rx"), attribute.String("opcode", commandstation.LnOpcodeName(op))))
		}

		o.ObserveFloat64(paceWait, s.PaceWaitSeconds, withBase())
		o.ObserveInt64(coalesced, int64(s.TxCoalesced), withBase())

		o.ObserveInt64(dropped, int64(s.ObsDropped), withBase(attribute.String("queue", "obs")))
		o.ObserveInt64(dropped, int64(s.SyncDropped), withBase(attribute.String("queue", "sync")))

		o.ObserveInt64(errorsC, int64(s.TxErrors), withBase(attribute.String("kind", "tx")))
		o.ObserveInt64(errorsC, int64(s.BadChecksum), withBase(attribute.String("kind", "bad_checksum")))
		o.ObserveInt64(errorsC, int64(s.Reconnects), withBase(attribute.String("kind", "reconnect")))
		o.ObserveInt64(errorsC, int64(s.WriteTimeouts), withBase(attribute.String("kind", "write_timeout")))
		o.ObserveInt64(errorsC, int64(s.LackRejections), withBase(attribute.String("kind", "lack_reject")))

		o.ObserveInt64(slotOps, int64(s.SlotAcquires), withBase(attribute.String("op", "acquire")))
		o.ObserveInt64(slotOps, int64(s.SlotAcquireFails), withBase(attribute.String("op", "acquire_fail")))
		o.ObserveInt64(slotOps, int64(s.SlotRetries), withBase(attribute.String("op", "retry")))
		o.ObserveInt64(slotOps, int64(s.SlotReleases), withBase(attribute.String("op", "release")))
		o.ObserveInt64(slotOps, int64(s.SlotDispatches), withBase(attribute.String("op", "dispatch")))
		o.ObserveInt64(slotOps, int64(s.KeepaliveRefresh), withBase(attribute.String("op", "keepalive")))

		o.ObserveInt64(slotsActive, s.SlotsActive, withBase())

		o.ObserveInt64(queueDepth, s.RxQueueLen, withBase(attribute.String("queue", "rx")))
		o.ObserveInt64(queueDepth, s.ObsQueueLen, withBase(attribute.String("queue", "obs")))
		o.ObserveInt64(queueDepth, s.SyncQueueLen, withBase(attribute.String("queue", "sync")))
		return nil
	}

	reg, err := meter.RegisterCallback(cb,
		frames, bytesC, messages, paceWait, coalesced, dropped, errorsC,
		slotOps, slotsActive, queueDepth,
	)
	if err != nil {
		return nil, fmt.Errorf("register loconet metrics callback: %w", err)
	}
	return reg, nil
}
