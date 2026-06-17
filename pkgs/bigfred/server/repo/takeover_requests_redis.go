package repo

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// RedisTakeoverRequests stores active takeover rows in Redis.
type RedisTakeoverRequests struct {
	client *redis.Client
}

// NewRedisTakeoverRequests returns a Redis-backed takeover store.
func NewRedisTakeoverRequests(client *redis.Client) *RedisTakeoverRequests {
	return &RedisTakeoverRequests{client: client}
}

var _ TakeoverRequestStore = (*RedisTakeoverRequests)(nil)

func (r *RedisTakeoverRequests) RequiresJanitor() bool { return false }

func (r *RedisTakeoverRequests) Insert(ctx context.Context, row *domain.TakeoverRequest) error {
	id, err := r.client.Incr(ctx, contract.TakeoverNextIDKey()).Result()
	if err != nil {
		return err
	}
	row.ID = uint(id)
	return r.saveActive(ctx, *row)
}

func (r *RedisTakeoverRequests) Update(ctx context.Context, row *domain.TakeoverRequest) error {
	switch row.State {
	case domain.TakeoverStatePending, domain.TakeoverStateGranted:
		return r.saveActive(ctx, *row)
	default:
		return r.deleteActive(ctx, *row)
	}
}

func (r *RedisTakeoverRequests) FindByID(ctx context.Context, id uint) (domain.TakeoverRequest, error) {
	raw, err := r.client.Get(ctx, contract.TakeoverRequestKey(id)).Result()
	if err == redis.Nil {
		return domain.TakeoverRequest{}, ErrTakeoverRequestNotFound
	}
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	wire, err := contract.UnmarshalTakeoverRequest([]byte(raw))
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	return contract.TakeoverRequestFromWire(wire), nil
}

func (r *RedisTakeoverRequests) ListPending(ctx context.Context) ([]domain.TakeoverRequest, error) {
	return r.loadBySet(ctx, contract.TakeoverPendingSetKey)
}

func (r *RedisTakeoverRequests) ListGranted(ctx context.Context) ([]domain.TakeoverRequest, error) {
	// Granted rows are indexed per signalman; scan is unnecessary when
	// lease-release timers drive expiry. Return empty for janitor skip.
	return nil, nil
}

func (r *RedisTakeoverRequests) ListGrantedBySignalman(ctx context.Context, signalmanID uint) ([]domain.TakeoverRequest, error) {
	return r.loadBySet(ctx, contract.TakeoverSignalmanGrantedKey(signalmanID))
}

func (r *RedisTakeoverRequests) FindPendingForTarget(
	ctx context.Context,
	target domain.TakeoverTarget,
	targetID uint,
) (domain.TakeoverRequest, error) {
	raw, err := r.client.Get(ctx, contract.TakeoverPendingTargetKey(target, targetID)).Result()
	if err == redis.Nil {
		return domain.TakeoverRequest{}, ErrTakeoverRequestNotFound
	}
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	return r.FindByID(ctx, uint(id))
}

func (r *RedisTakeoverRequests) saveActive(ctx context.Context, row domain.TakeoverRequest) error {
	payload, ttl, err := r.encodeRow(row)
	if err != nil {
		return err
	}
	key := contract.TakeoverRequestKey(row.ID)
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, key, payload, ttl)
	idStr := strconv.FormatUint(uint64(row.ID), 10)
	switch row.State {
	case domain.TakeoverStatePending:
		pipe.Set(ctx, contract.TakeoverPendingTargetKey(row.Target, row.TargetID), idStr, ttl)
		pipe.SAdd(ctx, contract.TakeoverPendingSetKey, idStr)
	case domain.TakeoverStateGranted:
		pipe.Del(ctx, contract.TakeoverPendingTargetKey(row.Target, row.TargetID))
		pipe.SRem(ctx, contract.TakeoverPendingSetKey, idStr)
		pipe.SAdd(ctx, contract.TakeoverSignalmanGrantedKey(row.SignalmanUserID), idStr)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisTakeoverRequests) deleteActive(ctx context.Context, row domain.TakeoverRequest) error {
	idStr := strconv.FormatUint(uint64(row.ID), 10)
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, contract.TakeoverRequestKey(row.ID))
	pipe.Del(ctx, contract.TakeoverPendingTargetKey(row.Target, row.TargetID))
	pipe.SRem(ctx, contract.TakeoverPendingSetKey, idStr)
	pipe.SRem(ctx, contract.TakeoverSignalmanGrantedKey(row.SignalmanUserID), idStr)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisTakeoverRequests) loadBySet(ctx context.Context, setKey string) ([]domain.TakeoverRequest, error) {
	ids, err := r.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, err
	}
	out := make([]domain.TakeoverRequest, 0, len(ids))
	for _, idStr := range ids {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		row, err := r.FindByID(ctx, uint(id))
		if err != nil {
			_, _ = r.client.SRem(ctx, setKey, idStr).Result()
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *RedisTakeoverRequests) encodeRow(row domain.TakeoverRequest) ([]byte, time.Duration, error) {
	wire := contract.TakeoverRequestToWire(row)
	payload, err := contract.MarshalTakeoverRequest(wire)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().UTC()
	var until time.Time
	switch row.State {
	case domain.TakeoverStatePending:
		until = row.AutoGrantAt.Add(30 * time.Second)
	case domain.TakeoverStateGranted:
		base := now
		if row.DecisionAt != nil {
			base = *row.DecisionAt
		}
		until = base.Add(domain.TakeoverLeaseDuration).Add(30 * time.Second)
	default:
		until = now.Add(time.Minute)
	}
	ttl := until.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	return payload, ttl, nil
}
