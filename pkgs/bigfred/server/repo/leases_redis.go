package repo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// leaseCreateNX atomically sets a lease key and updates owner/lessee indexes.
var leaseCreateNXScript = redis.NewScript(`
if redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2], 'NX') then
  redis.call('SADD', ARGV[3], KEYS[1])
  redis.call('SADD', ARGV[4], KEYS[1])
  return 1
end
return 0
`)

type leaseIndexes struct {
	ownerKey  func(uint) string
	lesseeKey func(uint) string
}

var (
	vehicleLeaseIndexes = leaseIndexes{
		ownerKey:  contract.VehicleLeaseByOwnerKey,
		lesseeKey: contract.VehicleLeaseByLesseeKey,
	}
	trainLeaseIndexes = leaseIndexes{
		ownerKey:  contract.TrainLeaseByOwnerKey,
		lesseeKey: contract.TrainLeaseByLesseeKey,
	}
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

func (r *RedisVehicleLeases) Get(ctx context.Context, vehicleID domain.VehicleID) (domain.VehicleLease, bool, error) {
	row, ok, err := getVehicleLease(ctx, r.client, contract.VehicleLeaseKey(vehicleID.String()), vehicleID)
	return row, ok, err
}

func (r *RedisVehicleLeases) ListActive(ctx context.Context, vehicleIDs []domain.VehicleID, now time.Time) ([]domain.VehicleLease, error) {
	return listActiveVehicleLeases(ctx, r.client, vehicleIDs, now)
}

func (r *RedisVehicleLeases) ListByOwner(ctx context.Context, ownerID uint) ([]domain.VehicleLease, error) {
	return listVehicleLeasesFromIndex(ctx, r.client, contract.VehicleLeaseByOwnerKey(ownerID))
}

func (r *RedisVehicleLeases) ListByLessee(ctx context.Context, lesseeID uint) ([]domain.VehicleLease, error) {
	return listVehicleLeasesFromIndex(ctx, r.client, contract.VehicleLeaseByLesseeKey(lesseeID))
}

func (r *RedisVehicleLeases) ListAll(ctx context.Context) ([]domain.VehicleLease, error) {
	return scanVehicleLeases(ctx, r.client, contract.VehicleLeaseKeyScanPattern)
}

func (r *RedisVehicleLeases) Create(ctx context.Context, row *domain.VehicleLease, overwrite bool) (bool, error) {
	key := contract.VehicleLeaseKey(row.VehicleID.String())
	payload, ttl, err := marshalVehicleLease(row)
	if err != nil {
		return false, err
	}
	return createLease(ctx, r.client, key, payload, ttl, vehicleLeaseIndexes, row.FromUserID, row.ToUserID, overwrite)
}

func (r *RedisVehicleLeases) Update(ctx context.Context, row *domain.VehicleLease) error {
	key := contract.VehicleLeaseKey(row.VehicleID.String())
	payload, ttl, err := marshalVehicleLease(row)
	if err != nil {
		return err
	}
	return updateLeaseKey(ctx, r.client, key, payload, ttl)
}

func (r *RedisVehicleLeases) Revoke(ctx context.Context, vehicleID domain.VehicleID) error {
	return revokeLeaseKey(ctx, r.client, contract.VehicleLeaseKey(vehicleID.String()), vehicleLeaseIndexes)
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

func (r *RedisTrainLeases) Get(ctx context.Context, trainID domain.TrainID) (domain.TrainLease, bool, error) {
	row, ok, err := getTrainLease(ctx, r.client, contract.TrainLeaseKey(trainID.String()), trainID)
	return row, ok, err
}

func (r *RedisTrainLeases) ListActive(ctx context.Context, trainIDs []domain.TrainID, now time.Time) ([]domain.TrainLease, error) {
	return listActiveTrainLeases(ctx, r.client, trainIDs, now)
}

func (r *RedisTrainLeases) ListByOwner(ctx context.Context, ownerID uint) ([]domain.TrainLease, error) {
	return listTrainLeasesFromIndex(ctx, r.client, contract.TrainLeaseByOwnerKey(ownerID))
}

func (r *RedisTrainLeases) ListByLessee(ctx context.Context, lesseeID uint) ([]domain.TrainLease, error) {
	return listTrainLeasesFromIndex(ctx, r.client, contract.TrainLeaseByLesseeKey(lesseeID))
}

func (r *RedisTrainLeases) ListAll(ctx context.Context) ([]domain.TrainLease, error) {
	return scanTrainLeases(ctx, r.client, contract.TrainLeaseKeyScanPattern)
}

func (r *RedisTrainLeases) Create(ctx context.Context, row *domain.TrainLease, overwrite bool) (bool, error) {
	key := contract.TrainLeaseKey(row.TrainID.String())
	payload, ttl, err := marshalTrainLease(row)
	if err != nil {
		return false, err
	}
	return createLease(ctx, r.client, key, payload, ttl, trainLeaseIndexes, row.FromUserID, row.ToUserID, overwrite)
}

func (r *RedisTrainLeases) Update(ctx context.Context, row *domain.TrainLease) error {
	key := contract.TrainLeaseKey(row.TrainID.String())
	payload, ttl, err := marshalTrainLease(row)
	if err != nil {
		return err
	}
	return updateLeaseKey(ctx, r.client, key, payload, ttl)
}

func (r *RedisTrainLeases) Revoke(ctx context.Context, trainID domain.TrainID) error {
	return revokeLeaseKey(ctx, r.client, contract.TrainLeaseKey(trainID.String()), trainLeaseIndexes)
}

var errLeaseNotFound = errors.New("lease_not_found")

// ErrLeaseNotFound is returned when a lease key does not exist in Redis.
var ErrLeaseNotFound = errLeaseNotFound

func createLease(
	ctx context.Context,
	client *redis.Client,
	key string,
	payload []byte,
	ttl time.Duration,
	idx leaseIndexes,
	fromUserID, toUserID uint,
	overwrite bool,
) (bool, error) {
	if overwrite {
		if err := createLeaseOverwrite(ctx, client, key, payload, ttl, idx, fromUserID, toUserID); err != nil {
			return false, err
		}
		return true, nil
	}
	res, err := leaseCreateNXScript.Run(
		ctx,
		client,
		[]string{key},
		payload,
		ttlMilliseconds(ttl),
		idx.ownerKey(fromUserID),
		idx.lesseeKey(toUserID),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func createLeaseOverwrite(
	ctx context.Context,
	client *redis.Client,
	key string,
	payload []byte,
	ttl time.Duration,
	idx leaseIndexes,
	fromUserID, toUserID uint,
) error {
	ownerKey := idx.ownerKey(fromUserID)
	lesseeKey := idx.lesseeKey(toUserID)
	for {
		err := client.Watch(ctx, func(tx *redis.Tx) error {
			oldRaw, err := tx.Get(ctx, key).Bytes()
			pipe := tx.TxPipeline()
			if err == nil {
				if wire, err := contract.UnmarshalLease(oldRaw); err == nil {
					pipe.SRem(ctx, idx.ownerKey(wire.FromUserID), key)
					pipe.SRem(ctx, idx.lesseeKey(wire.ToUserID), key)
				}
			} else if !errors.Is(err, redis.Nil) {
				return err
			}
			pipe.Set(ctx, key, payload, ttl)
			pipe.SAdd(ctx, ownerKey, key)
			pipe.SAdd(ctx, lesseeKey, key)
			_, err = pipe.Exec(ctx)
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return err
	}
}

func updateLeaseKey(ctx context.Context, client *redis.Client, key string, payload []byte, ttl time.Duration) error {
	ok, err := client.SetXX(ctx, key, payload, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return errLeaseNotFound
	}
	return nil
}

func revokeLeaseKey(ctx context.Context, client *redis.Client, key string, idx leaseIndexes) error {
	for {
		err := client.Watch(ctx, func(tx *redis.Tx) error {
			oldRaw, err := tx.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				return nil
			}
			if err != nil {
				return err
			}
			wire, err := contract.UnmarshalLease(oldRaw)
			if err != nil {
				return err
			}
			pipe := tx.TxPipeline()
			pipe.Del(ctx, key)
			pipe.SRem(ctx, idx.ownerKey(wire.FromUserID), key)
			pipe.SRem(ctx, idx.lesseeKey(wire.ToUserID), key)
			_, err = pipe.Exec(ctx)
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return err
	}
}

func ttlMilliseconds(ttl time.Duration) string {
	ms := ttl.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return strconv.FormatInt(ms, 10)
}

func marshalVehicleLease(row *domain.VehicleLease) ([]byte, time.Duration, error) {
	now := time.Now().UTC()
	src := row.Source
	if src == "" {
		src = "manual"
	}
	wire := contract.LeaseWire{
		FromUserID: row.FromUserID,
		ToUserID:   row.ToUserID,
		SpeedLimit: row.SpeedLimit,
		StartedAt:  row.StartedAt,
		ExpiresAt:  row.ExpiresAt,
		Source:     src,
	}
	payload, err := contract.MarshalLease(wire)
	if err != nil {
		return nil, 0, err
	}
	ttl := row.ExpiresAt.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	return payload, ttl, nil
}

func marshalTrainLease(row *domain.TrainLease) ([]byte, time.Duration, error) {
	now := time.Now().UTC()
	src := row.Source
	if src == "" {
		src = "manual"
	}
	wire := contract.LeaseWire{
		FromUserID: row.FromUserID,
		ToUserID:   row.ToUserID,
		SpeedLimit: row.SpeedLimit,
		StartedAt:  row.StartedAt,
		ExpiresAt:  row.ExpiresAt,
		Source:     src,
	}
	payload, err := contract.MarshalLease(wire)
	if err != nil {
		return nil, 0, err
	}
	ttl := row.ExpiresAt.Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}
	return payload, ttl, nil
}

func vehicleLeaseFromWire(vehicleID domain.VehicleID, raw []byte) (domain.VehicleLease, error) {
	wire, err := contract.UnmarshalLease(raw)
	if err != nil {
		return domain.VehicleLease{}, err
	}
	return domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: wire.FromUserID,
		ToUserID:   wire.ToUserID,
		SpeedLimit: wire.SpeedLimit,
		StartedAt:  wire.StartedAt,
		ExpiresAt:  wire.ExpiresAt,
		Source:     wire.Source,
	}, nil
}

func trainLeaseFromWire(trainID domain.TrainID, raw []byte) (domain.TrainLease, error) {
	wire, err := contract.UnmarshalLease(raw)
	if err != nil {
		return domain.TrainLease{}, err
	}
	return domain.TrainLease{
		TrainID:    trainID,
		FromUserID: wire.FromUserID,
		ToUserID:   wire.ToUserID,
		SpeedLimit: wire.SpeedLimit,
		StartedAt:  wire.StartedAt,
		ExpiresAt:  wire.ExpiresAt,
		Source:     wire.Source,
	}, nil
}

func getVehicleLease(ctx context.Context, client *redis.Client, key string, vehicleID domain.VehicleID) (domain.VehicleLease, bool, error) {
	raw, err := client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.VehicleLease{}, false, nil
	}
	if err != nil {
		return domain.VehicleLease{}, false, err
	}
	row, err := vehicleLeaseFromWire(vehicleID, raw)
	if err != nil {
		return domain.VehicleLease{}, false, err
	}
	return row, true, nil
}

func getTrainLease(ctx context.Context, client *redis.Client, key string, trainID domain.TrainID) (domain.TrainLease, bool, error) {
	raw, err := client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.TrainLease{}, false, nil
	}
	if err != nil {
		return domain.TrainLease{}, false, err
	}
	row, err := trainLeaseFromWire(trainID, raw)
	if err != nil {
		return domain.TrainLease{}, false, err
	}
	return row, true, nil
}

func listActiveVehicleLeases(ctx context.Context, client *redis.Client, vehicleIDs []domain.VehicleID, now time.Time) ([]domain.VehicleLease, error) {
	if len(vehicleIDs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(vehicleIDs))
	for i, id := range vehicleIDs {
		keys[i] = contract.VehicleLeaseKey(id.String())
	}
	vals, err := client.MGet(ctx, keys...).Result()
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
		row, err := vehicleLeaseFromWire(vehicleIDs[i], []byte(s))
		if err != nil {
			continue
		}
		if row.IsActive(now) {
			out = append(out, row)
		}
	}
	return out, nil
}

func listActiveTrainLeases(ctx context.Context, client *redis.Client, trainIDs []domain.TrainID, now time.Time) ([]domain.TrainLease, error) {
	if len(trainIDs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(trainIDs))
	for i, id := range trainIDs {
		keys[i] = contract.TrainLeaseKey(id.String())
	}
	vals, err := client.MGet(ctx, keys...).Result()
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
		row, err := trainLeaseFromWire(trainIDs[i], []byte(s))
		if err != nil {
			continue
		}
		if row.IsActive(now) {
			out = append(out, row)
		}
	}
	return out, nil
}

func listVehicleLeasesFromIndex(ctx context.Context, client *redis.Client, indexKey string) ([]domain.VehicleLease, error) {
	members, err := client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	vals, err := client.MGet(ctx, members...).Result()
	if err != nil {
		return nil, err
	}
	pipe := client.Pipeline()
	out := make([]domain.VehicleLease, 0, len(members))
	for i, raw := range vals {
		if raw == nil {
			pipe.SRem(ctx, indexKey, members[i])
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		vehicleID, ok := vehicleIDFromLeaseKey(members[i])
		if !ok {
			continue
		}
		row, err := vehicleLeaseFromWire(vehicleID, []byte(s))
		if err != nil {
			continue
		}
		out = append(out, row)
	}
	_, _ = pipe.Exec(ctx)
	return out, nil
}

func listTrainLeasesFromIndex(ctx context.Context, client *redis.Client, indexKey string) ([]domain.TrainLease, error) {
	members, err := client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	vals, err := client.MGet(ctx, members...).Result()
	if err != nil {
		return nil, err
	}
	pipe := client.Pipeline()
	out := make([]domain.TrainLease, 0, len(members))
	for i, raw := range vals {
		if raw == nil {
			pipe.SRem(ctx, indexKey, members[i])
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		trainID, ok := trainIDFromLeaseKey(members[i])
		if !ok {
			continue
		}
		row, err := trainLeaseFromWire(trainID, []byte(s))
		if err != nil {
			continue
		}
		out = append(out, row)
	}
	_, _ = pipe.Exec(ctx)
	return out, nil
}

func scanVehicleLeases(ctx context.Context, client *redis.Client, pattern string) ([]domain.VehicleLease, error) {
	var cursor uint64
	out := make([]domain.VehicleLease, 0)
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		if len(keys) > 0 {
			vals, err := client.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}
			for i, raw := range vals {
				if raw == nil {
					continue
				}
				s, ok := raw.(string)
				if !ok {
					continue
				}
				vehicleID, ok := vehicleIDFromLeaseKey(keys[i])
				if !ok {
					continue
				}
				row, err := vehicleLeaseFromWire(vehicleID, []byte(s))
				if err != nil {
					continue
				}
				out = append(out, row)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func scanTrainLeases(ctx context.Context, client *redis.Client, pattern string) ([]domain.TrainLease, error) {
	var cursor uint64
	out := make([]domain.TrainLease, 0)
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		if len(keys) > 0 {
			vals, err := client.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}
			for i, raw := range vals {
				if raw == nil {
					continue
				}
				s, ok := raw.(string)
				if !ok {
					continue
				}
				trainID, ok := trainIDFromLeaseKey(keys[i])
				if !ok {
					continue
				}
				row, err := trainLeaseFromWire(trainID, []byte(s))
				if err != nil {
					continue
				}
				out = append(out, row)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func vehicleIDFromLeaseKey(key string) (domain.VehicleID, bool) {
	const prefix = "bigfred:lease:vehicle:"
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return "", false
	}
	return domain.VehicleID(key[len(prefix):]), true
}

func trainIDFromLeaseKey(key string) (domain.TrainID, bool) {
	const prefix = "bigfred:lease:train:"
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return "", false
	}
	return domain.TrainID(key[len(prefix):]), true
}
