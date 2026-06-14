// Package state is the dcc-bus Redis client: loco state cache,
// pub/sub command and roster snapshot channels.
package state

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// Redis is the daemon's typed wrapper around go-redis. Key and channel
// names come from pkgs/bigfred/contract (see contract/redis.go).
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
	return contract.LocoStateKey(r.layoutID, addr)
}

// EventChannel returns the pub/sub channel daemons publish onto and
// loco-server consumes from.
func (r *Redis) EventChannel() string {
	return contract.DccBusEventChannel(r.layoutID, r.commandStationID)
}

// CommandChannel is the inverse: loco-server publishes, this daemon
// subscribes. Used for in-process train.setSpeed → DCC writes and
// for cross-process estop fan-out (§7e.3).
func (r *Redis) CommandChannel() string {
	return contract.DccBusCommandChannel(r.layoutID, r.commandStationID)
}

// StoreState writes one snapshot atomically and publishes it on the
// event channel. The TTL keeps stale rows from accumulating after
// a roster removal (§7e.3 — server bumps the TTL via the same key
// on takeover).
//
// The two operations live inside a TxPipeline so a reader that
// SUBSCRIBE'd to the event channel never sees a state event for a
// key it cannot GET back.
func (r *Redis) StoreState(ctx context.Context, snap contract.LocoStateWire, ttl time.Duration) error {
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
func (r *Redis) LoadState(ctx context.Context, addr uint16) (contract.LocoStateWire, bool, error) {
	raw, err := r.client.Get(ctx, r.StateKey(addr)).Result()
	if err == redis.Nil {
		return contract.LocoStateWire{}, false, nil
	}
	if err != nil {
		return contract.LocoStateWire{}, false, err
	}
	var snap contract.LocoStateWire
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return contract.LocoStateWire{}, false, err
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

// SubscribeLayoutRadioStop listens for layout-wide Radio Stop commands
// published by loco-server (§4.6.4).
func (r *Redis) SubscribeLayoutRadioStop(ctx context.Context) (*redis.PubSub, error) {
	channel := contract.LayoutRadioStopChannel(r.layoutID)
	sub := r.client.Subscribe(ctx, channel)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, err
	}
	return sub, nil
}

