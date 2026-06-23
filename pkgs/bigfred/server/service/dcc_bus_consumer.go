package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// DccBusEventConsumer subscribes to every `dcc-bus:evt:*` channel
// and fans the resulting events out to relevant control-plane WS
// clients. It is the inverse of DccBusService.PublishCommand: the
// daemon talks, loco-server (a) audits the event and (b) re-broadcasts
// audit-class envelopes onto the user's control session so the
// dashboard / sudo HUD can react.
type DccBusEventConsumer struct {
	redis *RedisService
	hub   *ws.Hub
	log   *logrus.Logger
	metrics *metrics.Metrics

	mu      sync.Mutex
	cancel  context.CancelFunc
	stopped bool
}

// NewDccBusEventConsumer assembles the consumer. Call Start exactly
// once during bootstrap; it spins a dedicated goroutine that lives
// until ctx is cancelled.
func NewDccBusEventConsumer(redis *RedisService, hub *ws.Hub, log *logrus.Logger) *DccBusEventConsumer {
	if log == nil {
		log = logrus.New()
	}
	return &DccBusEventConsumer{redis: redis, hub: hub, log: log}
}

// SetMetrics wires optional OpenTelemetry recorders for Redis pub/sub events.
func (c *DccBusEventConsumer) SetMetrics(m *metrics.Metrics) {
	c.metrics = m
}

// Start spins the pub/sub subscriber. The pattern `dcc-bus:evt:*`
// catches every running daemon. The goroutine exits when ctx is
// cancelled or the Redis connection breaks irrecoverably.
func (c *DccBusEventConsumer) Start(ctx context.Context) error {
	if c.redis == nil {
		return nil
	}
	subCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	sub := c.redis.Client().PSubscribe(subCtx, contract.DccBusEventChannelPattern)
	if _, err := sub.Receive(subCtx); err != nil {
		cancel()
		return err
	}
	go c.run(subCtx, sub)
	return nil
}

// Stop tears the subscriber down. Idempotent.
func (c *DccBusEventConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.stopped = true
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *DccBusEventConsumer) run(ctx context.Context, sub *redis.PubSub) {
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.dispatch(ctx, msg)
		}
	}
}

// dispatch routes one daemon-published frame. The channel name
// follows `dcc-bus:evt:<layoutId>:<csId>` so we can scope the
// broadcast to a single layout.
func (c *DccBusEventConsumer) dispatch(ctx context.Context, msg *redis.Message) {
	layoutID, _ := parseEventChannel(msg.Channel)
	if layoutID == 0 {
		return
	}

	// The daemon publishes either:
	//   - raw contract.EnvelopeWire JSON (state events, errors)
	//   - bare JSON object (audit events from the router)
	// Try the envelope shape first; fall through to log-only on miss.
	var env struct {
		Type    string          `json:"type"`
		ID      string          `json:"id,omitempty"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &env); err == nil && env.Type != "" {
		if c.metrics != nil {
			c.metrics.RecordDccBusConsumerEvent(env.Type)
		}
		c.fanEnvelope(layoutID, env.Type, env.ID, env.Payload, msg.Payload)
		return
	}
	c.log.WithField("channel", msg.Channel).Debug("dcc-bus evt: ignored non-envelope")
}

// fanEnvelope re-broadcasts the daemon's frame to every control-plane
// WS client pinned to the layout. We deliberately strip data-plane
// frames (loco.state, loco.error) so the control plane stays lean —
// throttles dial the daemon directly for state. What stays:
//
//   - system.estop.audit
//   - daemon.started / .stopped / .degraded
//   - takeover events the daemon may emit in a future milestone
func (c *DccBusEventConsumer) fanEnvelope(layoutID uint, eventType, id string, payload json.RawMessage, raw string) {
	switch eventType {
	case "system.estop.audit", "daemon.started", "daemon.stopped", "daemon.degraded":
		c.hub.BroadcastToLayout(layoutID, "dcc-bus."+strings.TrimPrefix(eventType, "system."), payload)
	default:
		// Unrouted frames are not broadcast but still logged so a
		// daemon emitting a new event type is visible in dev logs.
		c.log.WithField("type", eventType).Debug("dcc-bus evt: not broadcast")
		_ = raw
	}
	_ = time.Now() // placeholder for future audit fan-in
	_ = id
}

// parseEventChannel pulls layoutID + commandStationID out of a
// channel string like "dcc-bus:evt:7:2". Returns (0,0) on any
// formatting mismatch — the caller swallows those frames.
func parseEventChannel(ch string) (uint, uint) {
	if !strings.HasPrefix(ch, contract.DccBusEventChannelPrefix) {
		return 0, 0
	}
	rest := strings.TrimPrefix(ch, contract.DccBusEventChannelPrefix)
	parts := strings.Split(rest, ":")
	if len(parts) != 2 {
		return 0, 0
	}
	var layoutID, commandStationID uint
	for _, p := range parts[:1] {
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return 0, 0
			}
			layoutID = layoutID*10 + uint(ch-'0')
		}
	}
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			return 0, 0
		}
		commandStationID = commandStationID*10 + uint(ch-'0')
	}
	return layoutID, commandStationID
}
