package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/dcc-bus/protocol"
)

// Redis is the daemon's typed wrapper around go-redis. It owns the
// dcc-bus-side key layout documented in §7e.3:
//
//   - loco:state:<layoutId>:<addr>        — last-known per-loco snapshot
//   - dcc-bus:evt:<layoutId>:<csId>       — daemon → server event channel
//   - dcc-bus:cmd:<layoutId>:<csId>       — server → daemon command channel
//   - bigfred:layout:<layoutId>:allowed_vehicles — roster snapshots
//   - bigfred:layout:<layoutId>:defined_trains
type Redis struct {
	client           *redis.Client
	layoutID         uint
	commandStationID uint
}

// NewRedis returns a typed wrapper bound to the daemon's (layout,
// command-station) pair. The client is not closed by this struct;
// the caller owns the connection lifecycle.
func NewRedis(client *redis.Client, layoutID, commandStationID uint) *Redis {
	return &Redis{client: client, layoutID: layoutID, commandStationID: commandStationID}
}

// Client exposes the raw connection for low-level pub/sub callers.
func (r *Redis) Client() *redis.Client { return r.client }

// StateKey returns the Redis key holding the last-known state for
// one locomotive on this daemon's layout.
func (r *Redis) StateKey(addr uint16) string {
	return fmt.Sprintf("loco:state:%d:%d", r.layoutID, addr)
}

// EventChannel returns the pub/sub channel daemons publish onto and
// loco-server consumes from.
func (r *Redis) EventChannel() string {
	return fmt.Sprintf("dcc-bus:evt:%d:%d", r.layoutID, r.commandStationID)
}

// CommandChannel is the inverse: loco-server publishes, this daemon
// subscribes. Used for in-process train.setSpeed → DCC writes and
// for cross-process estop fan-out (§7e.3).
func (r *Redis) CommandChannel() string {
	return fmt.Sprintf("dcc-bus:cmd:%d:%d", r.layoutID, r.commandStationID)
}

// StoreState writes one snapshot atomically and publishes it on the
// event channel. The TTL keeps stale rows from accumulating after
// a roster removal (§7e.3 — server bumps the TTL via the same key
// on takeover).
//
// The two operations live inside a TxPipeline so a reader that
// SUBSCRIBE'd to the event channel never sees a state event for a
// key it cannot GET back.
func (r *Redis) StoreState(ctx context.Context, snap protocol.LocoStatePayload, ttl time.Duration) error {
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, r.StateKey(snap.Address), payload, ttl)
	pipe.Publish(ctx, r.EventChannel(), payload)
	_, err = pipe.Exec(ctx)
	return err
}

// LoadState reads the latest snapshot for one locomotive. Returns
// (zero, false, nil) when the key is missing.
func (r *Redis) LoadState(ctx context.Context, addr uint16) (protocol.LocoStatePayload, bool, error) {
	raw, err := r.client.Get(ctx, r.StateKey(addr)).Result()
	if err == redis.Nil {
		return protocol.LocoStatePayload{}, false, nil
	}
	if err != nil {
		return protocol.LocoStatePayload{}, false, err
	}
	var snap protocol.LocoStatePayload
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return protocol.LocoStatePayload{}, false, err
	}
	return snap, true, nil
}

// Publish emits a typed event on the daemon's event channel without
// touching the state cache. Used for non-state-bearing audit frames
// (`dcc-bus.opened`, `system.estop` echoes).
func (r *Redis) Publish(ctx context.Context, eventType string, payload any) error {
	env, err := protocol.Frame(eventType, payload)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return r.client.Publish(ctx, r.EventChannel(), raw).Err()
}

// SubscribeCommands opens a pub/sub subscription on the daemon's
// command channel. The returned *redis.PubSub MUST be Close'd by
// the caller when ctx is cancelled.
func (r *Redis) SubscribeCommands(ctx context.Context) (*redis.PubSub, error) {
	sub := r.client.Subscribe(ctx, r.CommandChannel())
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, err
	}
	return sub, nil
}

