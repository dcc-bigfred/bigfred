package repo

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// RedisSudoElevations stores active sudo grants in Redis with key TTL.
// Grants do not survive a Redis flush or server restart.
type RedisSudoElevations struct {
	client *redis.Client
}

// NewRedisSudoElevations returns a Redis-backed sudo store.
func NewRedisSudoElevations(client *redis.Client) *RedisSudoElevations {
	return &RedisSudoElevations{client: client}
}

var _ SudoElevationStore = (*RedisSudoElevations)(nil)

func (r *RedisSudoElevations) RequiresJanitor() bool { return false }

func (r *RedisSudoElevations) FindActive(
	ctx context.Context, userID, layoutID uint, now time.Time,
) (domain.SudoElevation, error) {
	key := contract.SudoElevationKey(layoutID, userID)
	raw, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return domain.SudoElevation{}, ErrSudoElevationNotFound
	}
	if err != nil {
		return domain.SudoElevation{}, err
	}
	wire, err := contract.UnmarshalSudoElevation([]byte(raw))
	if err != nil {
		return domain.SudoElevation{}, err
	}
	row := domain.SudoElevation{
		UserID:    userID,
		LayoutID:  layoutID,
		GrantedAt: wire.GrantedAt,
		ExpiresAt: wire.ExpiresAt,
	}
	if !row.IsActive(now) {
		return domain.SudoElevation{}, ErrSudoElevationNotFound
	}
	return row, nil
}

func (r *RedisSudoElevations) Upsert(ctx context.Context, row *domain.SudoElevation) error {
	key := contract.SudoElevationKey(row.LayoutID, row.UserID)
	now := time.Now().UTC()

	if raw, err := r.client.Get(ctx, key).Result(); err == nil {
		if wire, err := contract.UnmarshalSudoElevation([]byte(raw)); err == nil {
			row.GrantedAt = wire.GrantedAt
		}
	} else if err != redis.Nil {
		return err
	}

	wire := contract.SudoElevationWire{
		GrantedAt: row.GrantedAt,
		ExpiresAt: row.ExpiresAt,
	}
	payload, err := contract.MarshalSudoElevation(wire)
	if err != nil {
		return err
	}
	ttl := row.ExpiresAt.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	if err := r.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		return err
	}
	row.ID = 0
	return nil
}

func (r *RedisSudoElevations) Delete(ctx context.Context, userID, layoutID uint) error {
	return r.client.Del(ctx, contract.SudoElevationKey(layoutID, userID)).Err()
}

func (r *RedisSudoElevations) ReapExpired(context.Context, time.Time) ([]domain.SudoElevation, error) {
	return nil, nil
}
