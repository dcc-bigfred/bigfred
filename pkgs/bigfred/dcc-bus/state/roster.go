package state

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// LoadAllowedVehicles reads the latest snapshot from Redis. Returns
// (zero, false, nil) when the key is missing (daemon starts empty
// until loco-server publishes).
func (r *Redis) LoadAllowedVehicles(ctx context.Context) (contract.AllowedVehicles, bool, error) {
	return loadRosterSnapshot(ctx, r.client, contract.AllowedVehiclesKey(r.layoutID), contract.UnmarshalAllowedVehicles)
}

// LoadDefinedTrains reads the latest train roster snapshot.
func (r *Redis) LoadDefinedTrains(ctx context.Context) (contract.DefinedTrains, bool, error) {
	return loadRosterSnapshot(ctx, r.client, contract.DefinedTrainsKey(r.layoutID), contract.UnmarshalDefinedTrains)
}

// SubscribeAllowedVehicles listens for full snapshot updates.
func (r *Redis) SubscribeAllowedVehicles(ctx context.Context) (*redis.PubSub, error) {
	return subscribeRoster(ctx, r.client, contract.AllowedVehiclesKey(r.layoutID))
}

// SubscribeDefinedTrains listens for train roster updates.
func (r *Redis) SubscribeDefinedTrains(ctx context.Context) (*redis.PubSub, error) {
	return subscribeRoster(ctx, r.client, contract.DefinedTrainsKey(r.layoutID))
}

func loadRosterSnapshot[T any](ctx context.Context, client *redis.Client, key string, decode func([]byte) (T, error)) (T, bool, error) {
	var zero T
	raw, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	out, err := decode([]byte(raw))
	if err != nil {
		return zero, false, err
	}
	return out, true, nil
}

func subscribeRoster(ctx context.Context, client *redis.Client, channel string) (*redis.PubSub, error) {
	sub := client.Subscribe(ctx, channel)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, err
	}
	return sub, nil
}
