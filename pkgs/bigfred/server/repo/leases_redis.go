package repo

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// RedisVehicleLeases stores active vehicle leases in Redis with key TTL.
type RedisVehicleLeases struct {
	client *redis.Client
}

// NewRedisVehicleLeases returns a Redis-backed vehicle lease store.
func NewRedisVehicleLeases(client *redis.Client) *RedisVehicleLeases {
	return &RedisVehicleLeases{client: client}
}

var _ VehicleLeaseStore = (*RedisVehicleLeases)(nil)

func (r *RedisVehicleLeases) RequiresJanitor() bool { return false }

func (r *RedisVehicleLeases) ListActive(ctx context.Context, vehicleIDs []uint, now time.Time) ([]domain.VehicleLease, error) {
	if len(vehicleIDs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(vehicleIDs))
	for i, id := range vehicleIDs {
		keys[i] = contract.VehicleLeaseKey(id)
	}
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]domain.VehicleLease, 0, len(vals))
	for i, raw := range vals {
		if raw == nil {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		wire, err := contract.UnmarshalLease([]byte(s))
		if err != nil {
			continue
		}
		row := domain.VehicleLease{
			VehicleID:  vehicleIDs[i],
			FromUserID: wire.FromUserID,
			ToUserID:   wire.ToUserID,
			StartedAt:  wire.StartedAt,
			ExpiresAt:  wire.ExpiresAt,
		}
		if row.IsActive(now) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *RedisVehicleLeases) Insert(ctx context.Context, row *domain.VehicleLease) error {
	now := time.Now().UTC()
	wire := contract.LeaseWire{
		FromUserID: row.FromUserID,
		ToUserID:   row.ToUserID,
		StartedAt:  row.StartedAt,
		ExpiresAt:  row.ExpiresAt,
		Source:     "takeover",
	}
	payload, err := contract.MarshalLease(wire)
	if err != nil {
		return err
	}
	ttl := row.ExpiresAt.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	if err := r.client.Set(ctx, contract.VehicleLeaseKey(row.VehicleID), payload, ttl).Err(); err != nil {
		return err
	}
	row.ID = row.VehicleID
	return nil
}

func (r *RedisVehicleLeases) Revoke(ctx context.Context, vehicleID uint, _ time.Time) error {
	return r.client.Del(ctx, contract.VehicleLeaseKey(vehicleID)).Err()
}

// RedisTrainLeases stores active train leases in Redis with key TTL.
type RedisTrainLeases struct {
	client *redis.Client
}

// NewRedisTrainLeases returns a Redis-backed train lease store.
func NewRedisTrainLeases(client *redis.Client) *RedisTrainLeases {
	return &RedisTrainLeases{client: client}
}

var _ TrainLeaseStore = (*RedisTrainLeases)(nil)

func (r *RedisTrainLeases) RequiresJanitor() bool { return false }

func (r *RedisTrainLeases) ListActive(ctx context.Context, trainIDs []uint, now time.Time) ([]domain.TrainLease, error) {
	if len(trainIDs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(trainIDs))
	for i, id := range trainIDs {
		keys[i] = contract.TrainLeaseKey(id)
	}
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]domain.TrainLease, 0, len(vals))
	for i, raw := range vals {
		if raw == nil {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		wire, err := contract.UnmarshalLease([]byte(s))
		if err != nil {
			continue
		}
		row := domain.TrainLease{
			TrainID:    trainIDs[i],
			FromUserID: wire.FromUserID,
			ToUserID:   wire.ToUserID,
			StartedAt:  wire.StartedAt,
			ExpiresAt:  wire.ExpiresAt,
		}
		if row.IsActive(now) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *RedisTrainLeases) Insert(ctx context.Context, row *domain.TrainLease) error {
	now := time.Now().UTC()
	wire := contract.LeaseWire{
		FromUserID: row.FromUserID,
		ToUserID:   row.ToUserID,
		StartedAt:  row.StartedAt,
		ExpiresAt:  row.ExpiresAt,
		Source:     "takeover",
	}
	payload, err := contract.MarshalLease(wire)
	if err != nil {
		return err
	}
	ttl := row.ExpiresAt.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	if err := r.client.Set(ctx, contract.TrainLeaseKey(row.TrainID), payload, ttl).Err(); err != nil {
		return err
	}
	row.ID = row.TrainID
	return nil
}

func (r *RedisTrainLeases) Revoke(ctx context.Context, trainID uint, _ time.Time) error {
	return r.client.Del(ctx, contract.TrainLeaseKey(trainID)).Err()
}
