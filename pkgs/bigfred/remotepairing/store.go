// Package remotepairing is the shared Redis store for inbound handset pairing.
// loco-server creates pending requests; dcc-bus completes pairing and
// maintains active sessions (one pilot per user per command station).
package remotepairing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

var (
	ErrDuplicatePair      = errors.New("remotepairing: duplicate pairing code among pending requests")
	ErrInvalidPairingCV   = errors.New("remotepairing: invalid pairing CV value")
	ErrUserAlreadyPaired  = errors.New("remotepairing: user already has an active handset session")
)

const maxPairGenerationAttempts = 32

// dedupTTLSlack extends the dedup SET TTL past the pending req TTL so the
// SET auto-cleans once the last pending req on a command station expires.
const dedupTTLSlack = 30 * time.Second

var completePairingScript = redis.NewScript(`
local reqKey = KEYS[1]
local activeKey = KEYS[2]
local activeByUserKey = KEYS[3]
local pendingByUserKey = KEYS[4]
local dedupKey = KEYS[5]
local activePrefix = ARGV[1]
local pairLabel = ARGV[2]
local clientKey = ARGV[3]
local activePayload = ARGV[4]

if redis.call('GET', reqKey) == false then
  return {0, ''}
end

local priorClientKey = redis.call('GET', activeByUserKey)
local evicted = ''
if priorClientKey ~= false and priorClientKey ~= clientKey then
  redis.call('DEL', activePrefix .. priorClientKey)
  evicted = priorClientKey
end

redis.call('SET', activeKey, activePayload)
redis.call('SET', activeByUserKey, clientKey)
redis.call('DEL', reqKey)
redis.call('DEL', pendingByUserKey)
if dedupKey ~= '' and pairLabel ~= '' then
  redis.call('SREM', dedupKey, pairLabel)
end
return {1, evicted}
`)

// touchSeenScript atomically updates lastSeenAt on an active session and
// refreshes its TTL. Using cjson inside Lua avoids the client-side
// read-modify-write race between TouchSeen (per-packet) and
// UpdateSessionScope (PATCH). It also preserves an existing TTL so a
// scope update on a sticky session does not strip its expiry.
var touchSeenScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v == false then return 0 end
local s = cjson.decode(v)
s['lastSeenAt'] = tonumber(ARGV[1])
local ttl = redis.call('PTTL', KEYS[1])
if ARGV[2] ~= '' then
  ttl = tonumber(ARGV[2])
end
if ttl and ttl > 0 then
  redis.call('SET', KEYS[1], cjson.encode(s), 'PX', ttl)
else
  redis.call('SET', KEYS[1], cjson.encode(s))
end
return 1
`)

// touchSeenBatchScript atomically updates lastSeenAt on many active
// sessions in one round-trip. KEYS are active keys; ARGV[1] is the TTL
// in ms (or '' to preserve each key's existing TTL); ARGV[i+1] is the
// lastSeenAt for KEYS[i]. Missing keys are skipped silently (evicted
// concurrently). Used by the coordinator's batched seen-flusher (WS-1b).
var touchSeenBatchScript = redis.NewScript(`
local ttl = ARGV[1]
for i = 1, #KEYS do
  local v = redis.call('GET', KEYS[i])
  if v then
    local s = cjson.decode(v)
    s['lastSeenAt'] = tonumber(ARGV[i+1])
    if ttl ~= '' then
      redis.call('SET', KEYS[i], cjson.encode(s), 'PX', ttl)
    else
      local pttl = redis.call('PTTL', KEYS[i])
      if pttl and pttl > 0 then
        redis.call('SET', KEYS[i], cjson.encode(s), 'PX', pttl)
      else
        redis.call('SET', KEYS[i], cjson.encode(s))
      end
    end
  end
end
return #KEYS
`)

// updateSessionScopeScript atomically rewrites the vehicle scope on an
// active session while preserving its TTL (sticky sessions keep their
// idle expiry). Returns 1 when the session existed, 0 otherwise.
var updateSessionScopeScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v == false then return 0 end
local s = cjson.decode(v)
s['vehicleIds'] = cjson.decode(ARGV[1])
s['allowedAddrs'] = cjson.decode(ARGV[2])
s['allowAllVehicles'] = (ARGV[3] == '1')
local ttl = redis.call('PTTL', KEYS[1])
if ttl and ttl > 0 then
  redis.call('SET', KEYS[1], cjson.encode(s), 'PX', ttl)
else
  redis.call('SET', KEYS[1], cjson.encode(s))
end
return 1
`)

// Store is a Redis-backed remote pairing session store.
type Store struct {
	client *redis.Client
	mu     sync.Mutex
	rng    *rand.Rand
}

// NewStore returns a store backed by client.
func NewStore(client *redis.Client) *Store {
	return &Store{client: client, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// CreatePendingInput carries the user-selected vehicle scope for a new pending req.
type CreatePendingInput struct {
	LayoutID         uint
	CommandStationID uint
	Protocol         string
	UserID           uint
	ReqID            string
	DisplayLabel     string
	VehicleIDs       []string
	AllowedAddrs     []uint16
	AllowAllVehicles bool
	HandsetBrakeSecs uint
	PairingCV3       int
	PairingCV4       int
}

// CreatePending deletes any prior pending req for the user, rejects when the
// user already has an active session, and stores the new pending request.
func (s *Store) CreatePending(ctx context.Context, in CreatePendingInput) (contract.RemotePendingWire, error) {
	if in.Protocol == "" {
		return contract.RemotePendingWire{}, errors.New("remotepairing: protocol is required")
	}
	if in.ReqID == "" {
		return contract.RemotePendingWire{}, errors.New("remotepairing: req id is required")
	}

	activeByUser := contract.RemotePairingByUserKey(in.LayoutID, in.CommandStationID, in.UserID)
	if prior, err := s.client.Get(ctx, activeByUser).Result(); err == nil && prior != "" {
		return contract.RemotePendingWire{}, ErrUserAlreadyPaired
	} else if err != nil && err != redis.Nil {
		return contract.RemotePendingWire{}, err
	}

	if err := s.clearUserPending(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.RemotePendingWire{}, err
	}

	now := contract.NowMS()
	req := contract.RemotePendingWire{
		LayoutID:         in.LayoutID,
		CommandStationID: in.CommandStationID,
		Protocol:         in.Protocol,
		UserID:           in.UserID,
		ReqID:            in.ReqID,
		DisplayLabel:     in.DisplayLabel,
		VehicleIDs:       append([]string(nil), in.VehicleIDs...),
		AllowedAddrs:     append([]uint16(nil), in.AllowedAddrs...),
		AllowAllVehicles: in.AllowAllVehicles,
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(in.HandsetBrakeSecs),
		CreatedAt:        now,
		PairingCV3:       in.PairingCV3,
		PairingCV4:       in.PairingCV4,
	}
	payload, err := contract.MarshalRemotePending(req)
	if err != nil {
		return contract.RemotePendingWire{}, err
	}

	reqKey := contract.RemotePairingReqKey(in.LayoutID, in.CommandStationID, in.ReqID)
	reqByUserKey := contract.RemotePairingReqByUserKey(in.LayoutID, in.CommandStationID, in.UserID)
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, reqKey, payload, contract.RemotePairingReqTTL)
	pipe.Set(ctx, reqByUserKey, reqKey, contract.RemotePairingReqTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return contract.RemotePendingWire{}, err
	}
	return req, nil
}

// CreateZ21PairingRequest generates a unique CV3/CV4 pair and stores a Z21 pending req.
func (s *Store) CreateZ21PairingRequest(ctx context.Context, in CreateZ21PairingInput) (contract.RemotePendingWire, error) {
	if err := s.rejectIfUserPaired(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.RemotePendingWire{}, err
	}
	if err := s.clearUserPending(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.RemotePendingWire{}, err
	}

	dedupKey := contract.RemotePairingReqDedupKey(in.LayoutID, in.CommandStationID, contract.RemoteProtocolZ21)
	now := contract.NowMS()

	for attempt := 0; attempt < maxPairGenerationAttempts; attempt++ {
		s.mu.Lock()
		cv3 := contract.RandomPairingCV(s.rng)
		cv4 := contract.RandomPairingCV(s.rng)
		s.mu.Unlock()
		label := contract.Z21PairLabel(cv3, cv4)
		reqID := contract.Z21PairReqID(cv3, cv4)

		added, err := s.client.SAdd(ctx, dedupKey, label).Result()
		if err != nil {
			return contract.RemotePendingWire{}, err
		}
		if added == 0 {
			continue
		}

		req := contract.RemotePendingWire{
			LayoutID:         in.LayoutID,
			CommandStationID: in.CommandStationID,
			Protocol:         contract.RemoteProtocolZ21,
			UserID:           in.UserID,
			ReqID:            reqID,
			DisplayLabel:     contract.Z21PairingDisplayLabel(cv3, cv4),
			VehicleIDs:       append([]string(nil), in.VehicleIDs...),
			AllowedAddrs:     append([]uint16(nil), in.AllowedAddrs...),
			AllowAllVehicles: in.AllowAllVehicles,
			HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(in.HandsetBrakeSecs),
			CreatedAt:        now,
			PairingCV3:       cv3,
			PairingCV4:       cv4,
		}
		payload, err := contract.MarshalRemotePending(req)
		if err != nil {
			_, _ = s.client.SRem(ctx, dedupKey, label).Result()
			return contract.RemotePendingWire{}, err
		}

		reqKey := contract.RemotePairingReqKey(in.LayoutID, in.CommandStationID, reqID)
		reqByUserKey := contract.RemotePairingReqByUserKey(in.LayoutID, in.CommandStationID, in.UserID)
		pipe := s.client.TxPipeline()
		pipe.Set(ctx, reqKey, payload, contract.RemotePairingReqTTL)
		pipe.Set(ctx, reqByUserKey, reqKey, contract.RemotePairingReqTTL)
		// Refresh the dedup SET TTL so expired pending reqs that were
		// never completed/cancelled do not accumulate labels forever
		// (which would eventually exhaust the 2500-pair space and make
		// pairing impossible). The slack covers the gap between the req
		// TTL and the sweep that reaps expired labels.
		pipe.Expire(ctx, dedupKey, contract.RemotePairingReqTTL+dedupTTLSlack)
		if _, err := pipe.Exec(ctx); err != nil {
			_, _ = s.client.SRem(ctx, dedupKey, label).Result()
			return contract.RemotePendingWire{}, err
		}
		return req, nil
	}
	return contract.RemotePendingWire{}, ErrDuplicatePair
}

// CreateZ21PairingInput carries the user-selected vehicle scope for Z21 pairing.
type CreateZ21PairingInput struct {
	LayoutID         uint
	CommandStationID uint
	UserID           uint
	VehicleIDs       []string
	AllowedAddrs     []uint16
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// CreateWithrottlePairingInput carries the user-selected vehicle scope for WiThrottle pairing.
type CreateWithrottlePairingInput struct {
	LayoutID         uint
	CommandStationID uint
	UserID           uint
	VehicleIDs       []string
	AllowedAddrs     []uint16
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// CreateWithrottlePairingRequest generates a unique 6-digit code and stores a WiThrottle pending req.
func (s *Store) CreateWithrottlePairingRequest(ctx context.Context, in CreateWithrottlePairingInput) (contract.RemotePendingWire, error) {
	if err := s.rejectIfUserPaired(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.RemotePendingWire{}, err
	}
	if err := s.clearUserPending(ctx, in.LayoutID, in.CommandStationID, in.UserID); err != nil {
		return contract.RemotePendingWire{}, err
	}

	dedupKey := contract.RemotePairingReqDedupKey(in.LayoutID, in.CommandStationID, contract.RemoteProtocolWithrottle)
	now := contract.NowMS()

	for attempt := 0; attempt < maxPairGenerationAttempts; attempt++ {
		s.mu.Lock()
		code := contract.RandomPairingCode(s.rng)
		s.mu.Unlock()
		label := contract.WithrottlePairLabel(code)
		reqID := contract.WithrottlePairReqID(code)

		added, err := s.client.SAdd(ctx, dedupKey, label).Result()
		if err != nil {
			return contract.RemotePendingWire{}, err
		}
		if added == 0 {
			continue
		}

		req := contract.RemotePendingWire{
			LayoutID:         in.LayoutID,
			CommandStationID: in.CommandStationID,
			Protocol:         contract.RemoteProtocolWithrottle,
			UserID:           in.UserID,
			ReqID:            reqID,
			DisplayLabel:     contract.WithrottlePairingDisplayLabel(code),
			VehicleIDs:       append([]string(nil), in.VehicleIDs...),
			AllowedAddrs:     append([]uint16(nil), in.AllowedAddrs...),
			AllowAllVehicles: in.AllowAllVehicles,
			HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(in.HandsetBrakeSecs),
			CreatedAt:        now,
			PairingCode:      code,
		}
		payload, err := contract.MarshalRemotePending(req)
		if err != nil {
			_, _ = s.client.SRem(ctx, dedupKey, label).Result()
			return contract.RemotePendingWire{}, err
		}

		reqKey := contract.RemotePairingReqKey(in.LayoutID, in.CommandStationID, reqID)
		reqByUserKey := contract.RemotePairingReqByUserKey(in.LayoutID, in.CommandStationID, in.UserID)
		pipe := s.client.TxPipeline()
		pipe.Set(ctx, reqKey, payload, contract.RemotePairingReqTTL)
		pipe.Set(ctx, reqByUserKey, reqKey, contract.RemotePairingReqTTL)
		pipe.Expire(ctx, dedupKey, contract.RemotePairingReqTTL+dedupTTLSlack)
		if _, err := pipe.Exec(ctx); err != nil {
			_, _ = s.client.SRem(ctx, dedupKey, label).Result()
			return contract.RemotePendingWire{}, err
		}
		return req, nil
	}
	return contract.RemotePendingWire{}, ErrDuplicatePair
}

func (s *Store) rejectIfUserPaired(ctx context.Context, layoutID, commandStationID, userID uint) error {
	activeByUser := contract.RemotePairingByUserKey(layoutID, commandStationID, userID)
	if prior, err := s.client.Get(ctx, activeByUser).Result(); err == nil && prior != "" {
		return ErrUserAlreadyPaired
	} else if err != nil && err != redis.Nil {
		return err
	}
	return nil
}

// GetPendingByUser returns the user's current pending pairing request, if any.
func (s *Store) GetPendingByUser(ctx context.Context, layoutID, commandStationID, userID uint) (contract.RemotePendingWire, bool, error) {
	reqByUserKey := contract.RemotePairingReqByUserKey(layoutID, commandStationID, userID)
	reqKey, err := s.client.Get(ctx, reqByUserKey).Result()
	if err == redis.Nil {
		return contract.RemotePendingWire{}, false, nil
	}
	if err != nil {
		return contract.RemotePendingWire{}, false, err
	}
	raw, err := s.client.Get(ctx, reqKey).Result()
	if err == redis.Nil {
		_, _ = s.client.Del(ctx, reqByUserKey).Result()
		return contract.RemotePendingWire{}, false, nil
	}
	if err != nil {
		return contract.RemotePendingWire{}, false, err
	}
	req, err := contract.UnmarshalRemotePending([]byte(raw))
	if err != nil {
		return contract.RemotePendingWire{}, false, err
	}
	return req, true, nil
}

// GetActiveByClientKey loads one paired session.
func (s *Store) GetActiveByClientKey(ctx context.Context, layoutID, commandStationID uint, clientKey string) (contract.RemoteSessionWire, bool, error) {
	raw, err := s.client.Get(ctx, contract.RemotePairingActiveKey(layoutID, commandStationID, clientKey)).Result()
	if err == redis.Nil {
		return contract.RemoteSessionWire{}, false, nil
	}
	if err != nil {
		return contract.RemoteSessionWire{}, false, err
	}
	active, err := contract.UnmarshalRemoteSession([]byte(raw))
	if err != nil {
		return contract.RemoteSessionWire{}, false, err
	}
	return active, true, nil
}

// ListActiveByUser returns the active session for userID, if any (at most one).
func (s *Store) ListActiveByUser(ctx context.Context, layoutID, commandStationID, userID uint) ([]contract.RemoteSessionWire, error) {
	byUserKey := contract.RemotePairingByUserKey(layoutID, commandStationID, userID)
	clientKey, err := s.client.Get(ctx, byUserKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	active, ok, err := s.GetActiveByClientKey(ctx, layoutID, commandStationID, clientKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		_, _ = s.client.Del(ctx, byUserKey).Result()
		return nil, nil
	}
	return []contract.RemoteSessionWire{active}, nil
}

// CompletePairing atomically promotes a pending req to an active session.
// priorEvicted is set when a previous session for the same user was removed.
func (s *Store) CompletePairing(ctx context.Context, layoutID, commandStationID uint, reqID, clientKey string, pairedAt int64, dedupLabel string) (contract.RemoteSessionWire, bool, string, error) {
	reqKey := contract.RemotePairingReqKey(layoutID, commandStationID, reqID)
	raw, err := s.client.Get(ctx, reqKey).Result()
	if err == redis.Nil {
		return contract.RemoteSessionWire{}, false, "", nil
	}
	if err != nil {
		return contract.RemoteSessionWire{}, false, "", err
	}
	req, err := contract.UnmarshalRemotePending([]byte(raw))
	if err != nil {
		return contract.RemoteSessionWire{}, false, "", err
	}

	active := contract.RemoteSessionWire{
		Protocol:         req.Protocol,
		UserID:           req.UserID,
		VehicleIDs:       append([]string(nil), req.VehicleIDs...),
		AllowedAddrs:     append([]uint16(nil), req.AllowedAddrs...),
		AllowAllVehicles: req.AllowAllVehicles,
		PairedAt:         pairedAt,
		PairingCV3:       req.PairingCV3,
		PairingCV4:       req.PairingCV4,
		PairingCode:      req.PairingCode,
		LastSeenAt:       pairedAt,
		ClientKey:        clientKey,
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(req.HandsetBrakeSecs),
	}
	payload, err := contract.MarshalRemoteSession(active)
	if err != nil {
		return contract.RemoteSessionWire{}, false, "", err
	}

	activePrefix := fmt.Sprintf("bigfred:remote:active:%d:%d:", layoutID, commandStationID)

	dedupKey := ""
	if req.Protocol != "" && dedupLabel != "" {
		dedupKey = contract.RemotePairingReqDedupKey(layoutID, commandStationID, req.Protocol)
	}

	res, err := completePairingScript.Run(
		ctx,
		s.client,
		[]string{
			reqKey,
			contract.RemotePairingActiveKey(layoutID, commandStationID, clientKey),
			contract.RemotePairingByUserKey(layoutID, commandStationID, req.UserID),
			contract.RemotePairingReqByUserKey(layoutID, commandStationID, req.UserID),
			dedupKey,
		},
		activePrefix,
		dedupLabel,
		clientKey,
		string(payload),
	).Slice()
	if err != nil {
		return contract.RemoteSessionWire{}, false, "", err
	}
	if len(res) < 2 {
		return contract.RemoteSessionWire{}, false, "", nil
	}
	ok, _ := res[0].(int64)
	if ok == 0 {
		return contract.RemoteSessionWire{}, false, "", nil
	}
	evicted, _ := res[1].(string)
	return active, true, evicted, nil
}

// PairViaCV3CV4 completes Z21 pairing from CV3/CV4 values. The returned
// evictedClientKey is the prior session's clientKey when pairing evicted
// a previous handset for the same user (empty otherwise); the caller is
// responsible for cleaning up that client's in-process state.
func (s *Store) PairViaCV3CV4(ctx context.Context, layoutID, commandStationID uint, pairingCV3, pairingCV4 int, clientKey string, pairedAt int64) (contract.RemoteSessionWire, bool, string, error) {
	if !contract.ValidPairingCV(pairingCV3) || !contract.ValidPairingCV(pairingCV4) {
		return contract.RemoteSessionWire{}, false, "", ErrInvalidPairingCV
	}
	reqID := contract.Z21PairReqID(pairingCV3, pairingCV4)
	label := contract.Z21PairLabel(pairingCV3, pairingCV4)
	active, ok, evicted, err := s.CompletePairing(ctx, layoutID, commandStationID, reqID, clientKey, pairedAt, label)
	return active, ok, evicted, err
}

// PairViaWithrottleCode completes WiThrottle pairing from a 6-digit code.
func (s *Store) PairViaWithrottleCode(ctx context.Context, layoutID, commandStationID uint, code, clientKey string, pairedAt int64) (contract.RemoteSessionWire, bool, string, error) {
	if !contract.ValidWithrottleCode(code) {
		return contract.RemoteSessionWire{}, false, "", errors.New("remotepairing: invalid withrottle pairing code")
	}
	reqID := contract.WithrottlePairReqID(code)
	label := contract.WithrottlePairLabel(code)
	active, ok, evicted, err := s.CompletePairing(ctx, layoutID, commandStationID, reqID, clientKey, pairedAt, label)
	return active, ok, evicted, err
}

// TouchSeen updates lastSeenAt for a paired client atomically and refreshes
// the session TTL. When sessionTTL > 0 it is applied (sticky idle window);
// otherwise the existing TTL is preserved. The update runs inside Lua to
// avoid racing with concurrent UpdateSessionScope writes.
func (s *Store) TouchSeen(ctx context.Context, layoutID, commandStationID uint, clientKey string, lastSeenAt int64, sessionTTL time.Duration) error {
	key := contract.RemotePairingActiveKey(layoutID, commandStationID, clientKey)
	ttlArg := ""
	if sessionTTL > 0 {
		ttlArg = strconv.FormatInt(sessionTTL.Milliseconds(), 10)
	}
	res, err := touchSeenScript.Run(ctx, s.client, []string{key}, lastSeenAt, ttlArg).Int()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}
	_ = res
	return nil
}

// TouchSeenBatch updates lastSeenAt for many paired clients in one
// round-trip and refreshes their TTLs. When ttl > 0 it is applied to
// every key (sticky idle window); otherwise each key's existing TTL is
// preserved. Used by the coordinator's batched seen-flusher (WS-1b) to
// replace a per-packet Redis SET.
func (s *Store) TouchSeenBatch(ctx context.Context, layoutID, commandStationID uint, clientKeys []string, lastSeenAt []int64, ttl time.Duration) error {
	if len(clientKeys) == 0 {
		return nil
	}
	keys := make([]string, len(clientKeys))
	for i, ck := range clientKeys {
		keys[i] = contract.RemotePairingActiveKey(layoutID, commandStationID, ck)
	}
	ttlArg := ""
	if ttl > 0 {
		ttlArg = strconv.FormatInt(ttl.Milliseconds(), 10)
	}
	args := make([]any, 0, 1+len(lastSeenAt))
	args = append(args, ttlArg)
	for _, ts := range lastSeenAt {
		args = append(args, ts)
	}
	if _, err := touchSeenBatchScript.Run(ctx, s.client, keys, args...).Int(); err != nil && err != redis.Nil {
		return err
	}
	return nil
}

// UpdateSessionScope changes vehicle scope on an active session without
// re-pairing. The rewrite is atomic (Lua) and preserves the existing TTL so
// sticky sessions do not lose their idle expiry on a PATCH.
func (s *Store) UpdateSessionScope(ctx context.Context, layoutID, commandStationID uint, clientKey string, vehicleIDs []string, allowedAddrs []uint16, allowAll bool) (contract.RemoteSessionWire, bool, error) {
	// Normalise nil slices to empty so cjson stores `[]` rather than
	// dropping the field entirely (cjson.decode("null") yields nil,
	// which would delete the key from the Lua table).
	if vehicleIDs == nil {
		vehicleIDs = []string{}
	}
	if allowedAddrs == nil {
		allowedAddrs = []uint16{}
	}
	key := contract.RemotePairingActiveKey(layoutID, commandStationID, clientKey)
	vehicleJSON, err := json.Marshal(vehicleIDs)
	if err != nil {
		return contract.RemoteSessionWire{}, false, err
	}
	addrsJSON, err := json.Marshal(allowedAddrs)
	if err != nil {
		return contract.RemoteSessionWire{}, false, err
	}
	allowArg := "0"
	if allowAll {
		allowArg = "1"
	}
	updated, err := updateSessionScopeScript.Run(ctx, s.client, []string{key}, string(vehicleJSON), string(addrsJSON), allowArg).Int()
	if err != nil {
		if err == redis.Nil {
			return contract.RemoteSessionWire{}, false, nil
		}
		return contract.RemoteSessionWire{}, false, err
	}
	if updated == 0 {
		return contract.RemoteSessionWire{}, false, nil
	}
	active, ok, err := s.GetActiveByClientKey(ctx, layoutID, commandStationID, clientKey)
	if err != nil || !ok {
		return contract.RemoteSessionWire{}, false, err
	}
	return active, true, nil
}

// Unpair removes one active session and drops it from the user index.
func (s *Store) Unpair(ctx context.Context, layoutID, commandStationID uint, clientKey string) error {
	key := contract.RemotePairingActiveKey(layoutID, commandStationID, clientKey)
	raw, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	active, err := contract.UnmarshalRemoteSession([]byte(raw))
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, key)
	byUser := contract.RemotePairingByUserKey(layoutID, commandStationID, active.UserID)
	pipe.Del(ctx, byUser)
	_, err = pipe.Exec(ctx)
	return err
}

// UnpairAllForUser removes the active session for one user on a command station.
func (s *Store) UnpairAllForUser(ctx context.Context, layoutID, commandStationID, userID uint) error {
	byUserKey := contract.RemotePairingByUserKey(layoutID, commandStationID, userID)
	clientKey, err := s.client.Get(ctx, byUserKey).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Unpair(ctx, layoutID, commandStationID, clientKey)
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
	reqKey := contract.RemotePairingReqKey(layoutID, commandStationID, req.ReqID)
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, reqKey)
	pipe.Del(ctx, contract.RemotePairingReqByUserKey(layoutID, commandStationID, userID))
	if req.Protocol == contract.RemoteProtocolZ21 && req.PairingCV3 != 0 && req.PairingCV4 != 0 {
		label := contract.Z21PairLabel(req.PairingCV3, req.PairingCV4)
		pipe.SRem(ctx, contract.RemotePairingReqDedupKey(layoutID, commandStationID, req.Protocol), label)
	}
	if req.Protocol == contract.RemoteProtocolWithrottle && req.PairingCode != "" {
		pipe.SRem(ctx, contract.RemotePairingReqDedupKey(layoutID, commandStationID, req.Protocol), contract.WithrottlePairLabel(req.PairingCode))
	}
	_, err = pipe.Exec(ctx)
	return err
}

// PendingExpiresAt returns the absolute expiry for a pending req.
func PendingExpiresAt(req contract.RemotePendingWire) time.Time {
	return time.UnixMilli(req.CreatedAt).UTC().Add(contract.RemotePairingReqTTL)
}

// ClientKeyFromAddr formats a Z21 endpoint without protocol prefix (legacy helper).
func ClientKeyFromAddr(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

// GetClientsSnapshot reads the latest handset presence snapshot written by dcc-bus.
func (s *Store) GetClientsSnapshot(ctx context.Context, layoutID, commandStationID uint) (contract.RemoteClientsSnapshotWire, bool, error) {
	raw, err := s.client.Get(ctx, contract.RemoteClientsSnapshotKey(layoutID, commandStationID)).Result()
	if err == redis.Nil {
		return contract.RemoteClientsSnapshotWire{}, false, nil
	}
	if err != nil {
		return contract.RemoteClientsSnapshotWire{}, false, err
	}
	snap, err := contract.UnmarshalRemoteClientsSnapshot([]byte(raw))
	if err != nil {
		return contract.RemoteClientsSnapshotWire{}, false, err
	}
	return snap, true, nil
}

// PublishSessionSync notifies dcc-bus daemons that a REST mutation changed
// an active handset session (unpair or scope update) so they can re-sync
// the affected client's in-process state without per-packet Redis reads.
// The caller is loco-server (server/cmd); daemon-side evictions do NOT
// publish (they already own the in-process state).
func (s *Store) PublishSessionSync(ctx context.Context, layoutID, commandStationID uint, clientKey, action string) error {
	if clientKey == "" {
		return nil
	}
	payload, err := contract.MarshalRemoteSessionSync(contract.RemoteSessionSyncEventWire{
		LayoutID:         layoutID,
		CommandStationID: commandStationID,
		ClientKey:        clientKey,
		Action:           action,
	})
	if err != nil {
		return err
	}
	return s.client.Publish(ctx, contract.RemoteSessionSyncChannel(layoutID, commandStationID), payload).Err()
}

// SubscribeSessionSync opens a pub/sub subscription on the per-CS sync
// channel. The returned *redis.PubSub MUST be Close'd by the caller.
func (s *Store) SubscribeSessionSync(ctx context.Context, layoutID, commandStationID uint) (*redis.PubSub, error) {
	sub := s.client.Subscribe(ctx, contract.RemoteSessionSyncChannel(layoutID, commandStationID))
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, err
	}
	return sub, nil
}
