// Package z21pairing is the shared Redis store for Z21 handset pairing.
// loco-server creates pending requests; dcc-bus completes pairing and
// maintains active sessions.
package z21pairing

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

var (
	ErrDuplicatePair    = errors.New("z21pairing: duplicate CV pair among pending requests")
	ErrInvalidPairingCV = errors.New("z21pairing: invalid pairing CV value")
)

const maxPairGenerationAttempts = 32

var pairViaCV3CV4Script = redis.NewScript(`
local reqKey = KEYS[1]
local activeKey = KEYS[2]
local byUserKey = KEYS[3]
local reqPairsKey = KEYS[4]
local reqByUserKey = KEYS[5]
local pairLabel = ARGV[1]
local clientKey = ARGV[2]
local activePayload = ARGV[3]

if redis.call('GET', reqKey) == false then
  return 0
end
redis.call('SET', activeKey, activePayload)
redis.call('DEL', reqKey)
redis.call('SREM', reqPairsKey, pairLabel)
redis.call('DEL', reqByUserKey)
redis.call('SADD', byUserKey, clientKey)
return 1
`)

// Store is a Redis-backed Z21 pairing session store.
type Store struct {
	client *redis.Client
	rng    *rand.Rand
}

// NewStore returns a store backed by client.
func NewStore(client *redis.Client) *Store {
	return &Store{client: client, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// CreatePairingRequestInput carries the user-selected vehicle scope.
type CreatePairingRequestInput struct {
	LayoutID         uint
	CommandStationID uint
	UserID           uint
	VehicleIDs       []string
	AllowedAddrs     []uint16
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// CreatePairingRequest deletes any prior pending req for the user, generates a
// unique CV3/CV4 pair, and stores it with Z21PairingReqTTL.
func (s *Store) CreatePairingRequest(ctx context.Context, in CreatePairingRequestInput) (contract.Z21PairingReqWire, error) {
	if err := s.clearUserPending(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.Z21PairingReqWire{}, err
	}

	reqPairsKey := contract.Z21PairingReqPairsKey(in.LayoutID, in.CommandStationID)
	now := contract.NowMS()

	for attempt := 0; attempt < maxPairGenerationAttempts; attempt++ {
		cv3 := contract.RandomPairingCV(s.rng)
		cv4 := contract.RandomPairingCV(s.rng)
		label := contract.Z21PairLabel(cv3, cv4)

		added, err := s.client.SAdd(ctx, reqPairsKey, label).Result()
		if err != nil {
			return contract.Z21PairingReqWire{}, err
		}
		if added == 0 {
			continue
		}

		req := contract.Z21PairingReqWire{
			LayoutID:         in.LayoutID,
			CommandStationID: in.CommandStationID,
			UserID:           in.UserID,
			PairingCV3:       cv3,
			PairingCV4:       cv4,
			DisplayLabel:     contract.Z21PairingDisplayLabel(cv3, cv4),
			VehicleIDs:       append([]string(nil), in.VehicleIDs...),
			AllowedAddrs:     append([]uint16(nil), in.AllowedAddrs...),
			AllowAllVehicles: in.AllowAllVehicles,
			HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(in.HandsetBrakeSecs),
			CreatedAt:        now,
		}
		payload, err := contract.MarshalZ21PairingReq(req)
		if err != nil {
			_, _ = s.client.SRem(ctx, reqPairsKey, label).Result()
			return contract.Z21PairingReqWire{}, err
		}

		reqKey := contract.Z21PairingReqKey(in.LayoutID, in.CommandStationID, cv3, cv4)
		reqByUserKey := contract.Z21PairingReqByUserKey(in.LayoutID, in.CommandStationID, in.UserID)
		pipe := s.client.TxPipeline()
		pipe.Set(ctx, reqKey, payload, contract.Z21PairingReqTTL)
		pipe.Set(ctx, reqByUserKey, reqKey, contract.Z21PairingReqTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			_, _ = s.client.SRem(ctx, reqPairsKey, label).Result()
			return contract.Z21PairingReqWire{}, err
		}
		return req, nil
	}
	return contract.Z21PairingReqWire{}, ErrDuplicatePair
}

// GetPendingByUser returns the user's current pending pairing request, if any.
func (s *Store) GetPendingByUser(ctx context.Context, layoutID, commandStationID, userID uint) (contract.Z21PairingReqWire, bool, error) {
	reqByUserKey := contract.Z21PairingReqByUserKey(layoutID, commandStationID, userID)
	reqKey, err := s.client.Get(ctx, reqByUserKey).Result()
	if err == redis.Nil {
		return contract.Z21PairingReqWire{}, false, nil
	}
	if err != nil {
		return contract.Z21PairingReqWire{}, false, err
	}
	raw, err := s.client.Get(ctx, reqKey).Result()
	if err == redis.Nil {
		_, _ = s.client.Del(ctx, reqByUserKey).Result()
		return contract.Z21PairingReqWire{}, false, nil
	}
	if err != nil {
		return contract.Z21PairingReqWire{}, false, err
	}
	req, err := contract.UnmarshalZ21PairingReq([]byte(raw))
	if err != nil {
		return contract.Z21PairingReqWire{}, false, err
	}
	return req, true, nil
}

// GetActiveByClientKey loads one paired session.
func (s *Store) GetActiveByClientKey(ctx context.Context, layoutID, commandStationID uint, clientKey string) (contract.Z21PairingActiveWire, bool, error) {
	raw, err := s.client.Get(ctx, contract.Z21PairingActiveKey(layoutID, commandStationID, clientKey)).Result()
	if err == redis.Nil {
		return contract.Z21PairingActiveWire{}, false, nil
	}
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	active, err := contract.UnmarshalZ21PairingActive([]byte(raw))
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	return active, true, nil
}

// ListActiveByUser returns every active session owned by userID.
func (s *Store) ListActiveByUser(ctx context.Context, layoutID, commandStationID, userID uint) ([]contract.Z21PairingActiveWire, error) {
	byUserKey := contract.Z21PairingByUserKey(layoutID, commandStationID, userID)
	keys, err := s.client.SMembers(ctx, byUserKey).Result()
	if err != nil {
		return nil, err
	}
	out := make([]contract.Z21PairingActiveWire, 0, len(keys))
	for _, clientKey := range keys {
		active, ok, err := s.GetActiveByClientKey(ctx, layoutID, commandStationID, clientKey)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, active)
			continue
		}
		_, _ = s.client.SRem(ctx, byUserKey, clientKey).Result()
	}
	return out, nil
}

// PairViaCV3CV4 atomically promotes a pending req to an active session.
func (s *Store) PairViaCV3CV4(ctx context.Context, layoutID, commandStationID uint, pairingCV3, pairingCV4 int, clientKey string, pairedAt int64) (contract.Z21PairingActiveWire, bool, error) {
	if !contract.ValidPairingCV(pairingCV3) || !contract.ValidPairingCV(pairingCV4) {
		return contract.Z21PairingActiveWire{}, false, ErrInvalidPairingCV
	}

	reqKey := contract.Z21PairingReqKey(layoutID, commandStationID, pairingCV3, pairingCV4)
	raw, err := s.client.Get(ctx, reqKey).Result()
	if err == redis.Nil {
		return contract.Z21PairingActiveWire{}, false, nil
	}
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	req, err := contract.UnmarshalZ21PairingReq([]byte(raw))
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}

	active := contract.Z21PairingActiveWire{
		UserID:           req.UserID,
		VehicleIDs:       append([]string(nil), req.VehicleIDs...),
		AllowedAddrs:     append([]uint16(nil), req.AllowedAddrs...),
		AllowAllVehicles: req.AllowAllVehicles,
		PairedAt:         pairedAt,
		PairingCV3:       pairingCV3,
		PairingCV4:       pairingCV4,
		LastSeenAt:       pairedAt,
		ClientKey:        clientKey,
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(req.HandsetBrakeSecs),
	}
	payload, err := contract.MarshalZ21PairingActive(active)
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}

	label := contract.Z21PairLabel(pairingCV3, pairingCV4)
	res, err := pairViaCV3CV4Script.Run(
		ctx,
		s.client,
		[]string{
			reqKey,
			contract.Z21PairingActiveKey(layoutID, commandStationID, clientKey),
			contract.Z21PairingByUserKey(layoutID, commandStationID, req.UserID),
			contract.Z21PairingReqPairsKey(layoutID, commandStationID),
			contract.Z21PairingReqByUserKey(layoutID, commandStationID, req.UserID),
		},
		label,
		clientKey,
		string(payload),
	).Int()
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	if res == 0 {
		return contract.Z21PairingActiveWire{}, false, nil
	}
	return active, true, nil
}

// TouchSeen updates lastSeenAt for a paired client. sessionTTL, when
// positive, refreshes the Redis key expiry (IP-sticky sessions).
func (s *Store) TouchSeen(ctx context.Context, layoutID, commandStationID uint, clientKey string, lastSeenAt int64, sessionTTL time.Duration) error {
	key := contract.Z21PairingActiveKey(layoutID, commandStationID, clientKey)
	raw, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	active, err := contract.UnmarshalZ21PairingActive([]byte(raw))
	if err != nil {
		return err
	}
	active.LastSeenAt = lastSeenAt
	payload, err := contract.MarshalZ21PairingActive(active)
	if err != nil {
		return err
	}
	if sessionTTL > 0 {
		return s.client.Set(ctx, key, payload, sessionTTL).Err()
	}
	return s.client.Set(ctx, key, payload, 0).Err()
}

// UpdateSessionScope changes vehicle scope on an active session without re-pairing.
func (s *Store) UpdateSessionScope(ctx context.Context, layoutID, commandStationID uint, clientKey string, vehicleIDs []string, allowedAddrs []uint16, allowAll bool) (contract.Z21PairingActiveWire, bool, error) {
	key := contract.Z21PairingActiveKey(layoutID, commandStationID, clientKey)
	raw, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return contract.Z21PairingActiveWire{}, false, nil
	}
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	active, err := contract.UnmarshalZ21PairingActive([]byte(raw))
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	active.VehicleIDs = append([]string(nil), vehicleIDs...)
	active.AllowedAddrs = append([]uint16(nil), allowedAddrs...)
	active.AllowAllVehicles = allowAll
	payload, err := contract.MarshalZ21PairingActive(active)
	if err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	if err := s.client.Set(ctx, key, payload, 0).Err(); err != nil {
		return contract.Z21PairingActiveWire{}, false, err
	}
	return active, true, nil
}

// Unpair removes one active session and drops it from the user index.
func (s *Store) Unpair(ctx context.Context, layoutID, commandStationID uint, clientKey string) error {
	key := contract.Z21PairingActiveKey(layoutID, commandStationID, clientKey)
	raw, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	active, err := contract.UnmarshalZ21PairingActive([]byte(raw))
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, contract.Z21PairingByUserKey(layoutID, commandStationID, active.UserID), clientKey)
	_, err = pipe.Exec(ctx)
	return err
}

// UnpairAllForUser removes every active session for one user on a command station.
func (s *Store) UnpairAllForUser(ctx context.Context, layoutID, commandStationID, userID uint) error {
	byUserKey := contract.Z21PairingByUserKey(layoutID, commandStationID, userID)
	keys, err := s.client.SMembers(ctx, byUserKey).Result()
	if err != nil {
		return err
	}
	for _, clientKey := range keys {
		if err := s.Unpair(ctx, layoutID, commandStationID, clientKey); err != nil {
			return err
		}
	}
	return nil
}

// CancelPendingPairing removes the user's pending pairing request, if any.
func (s *Store) CancelPendingPairing(ctx context.Context, layoutID, commandStationID, userID uint) error {
	return s.clearUserPending(ctx, layoutID, commandStationID, userID)
}

func (s *Store) clearUserPending(ctx context.Context, layoutID, commandStationID, userID uint) error {
	req, ok, err := s.GetPendingByUser(ctx, layoutID, commandStationID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	reqKey := contract.Z21PairingReqKey(layoutID, commandStationID, req.PairingCV3, req.PairingCV4)
	label := contract.Z21PairLabel(req.PairingCV3, req.PairingCV4)
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, reqKey)
	pipe.Del(ctx, contract.Z21PairingReqByUserKey(layoutID, commandStationID, userID))
	pipe.SRem(ctx, contract.Z21PairingReqPairsKey(layoutID, commandStationID), label)
	_, err = pipe.Exec(ctx)
	return err
}

// PendingExpiresAt returns the absolute expiry for a pending req.
func PendingExpiresAt(req contract.Z21PairingReqWire) time.Time {
	return time.UnixMilli(req.CreatedAt).UTC().Add(contract.Z21PairingReqTTL)
}

// ClientKeyFromAddr formats the Redis session key for a UDP endpoint.
func ClientKeyFromAddr(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

// GetClientsSnapshot reads the latest handset presence snapshot written by dcc-bus.
func (s *Store) GetClientsSnapshot(ctx context.Context, layoutID, commandStationID uint) (contract.Z21ClientsSnapshotWire, bool, error) {
	raw, err := s.client.Get(ctx, contract.Z21ClientsSnapshotKey(layoutID, commandStationID)).Result()
	if err == redis.Nil {
		return contract.Z21ClientsSnapshotWire{}, false, nil
	}
	if err != nil {
		return contract.Z21ClientsSnapshotWire{}, false, err
	}
	snap, err := contract.UnmarshalZ21ClientsSnapshot([]byte(raw))
	if err != nil {
		return contract.Z21ClientsSnapshotWire{}, false, err
	}
	return snap, true, nil
}
