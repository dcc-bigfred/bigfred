package service

import (
	"context"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

const (
	defaultRadioTTL         = 4 * time.Hour
	defaultRadioReplayLimit = 200
)

// RadioStore persists walkie-talkie messages in Redis streams (§4.4.4).
type RadioStore struct {
	redis       *RedisService
	ttl         time.Duration
	replayLimit int
}

// RadioStoreConfig configures stream TTL and replay cap.
type RadioStoreConfig struct {
	Redis       *RedisService
	TTL         time.Duration
	ReplayLimit int
}

// NewRadioStore returns a ready store. TTL defaults to 4h; replay limit
// to 200 when unset.
func NewRadioStore(cfg RadioStoreConfig) *RadioStore {
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = defaultRadioTTL
	}
	limit := cfg.ReplayLimit
	if limit <= 0 {
		limit = defaultRadioReplayLimit
	}
	return &RadioStore{
		redis:       cfg.Redis,
		ttl:         ttl,
		replayLimit: limit,
	}
}

// ReplayLimit returns the configured cap for history reads.
func (s *RadioStore) ReplayLimit() int { return s.replayLimit }

// Append writes msg to every stream key in keys, refreshing TTL on each.
// Returns the Redis stream id assigned on the first key.
func (s *RadioStore) Append(ctx context.Context, msg domain.RadioMessage, keys []string) (string, error) {
	if s == nil || s.redis == nil || len(keys) == 0 {
		return "", nil
	}
	raw, err := contract.MarshalRadioMessage(msg)
	if err != nil {
		return "", err
	}
	client := s.redis.Client()
	var firstID string
	for _, key := range keys {
		id, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: key,
			Values: map[string]interface{}{"payload": raw},
		}).Result()
		if err != nil {
			return "", err
		}
		if firstID == "" {
			firstID = id
		}
		_ = client.Expire(ctx, key, s.ttl).Err()
	}
	if msg.ID == "" {
		msg.ID = firstID
	}
	return firstID, nil
}

// Replay reads up to limit messages from each stream, merges them by
// SentAt ascending and returns the newest `limit` rows.
func (s *RadioStore) Replay(ctx context.Context, keys []string, limit int) ([]domain.RadioMessage, error) {
	if s == nil || s.redis == nil || len(keys) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = s.replayLimit
	}
	client := s.redis.Client()
	merged := make([]domain.RadioMessage, 0, limit)
	seen := make(map[string]struct{}, limit)

	for _, key := range keys {
		rows, err := client.XRange(ctx, key, "-", "+").Result()
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			raw, ok := row.Values["payload"].(string)
			if !ok {
				continue
			}
			msg, err := contract.UnmarshalRadioMessage([]byte(raw))
			if err != nil {
				continue
			}
			if msg.ID == "" {
				msg.ID = row.ID
			}
			if _, dup := seen[msg.ID]; dup {
				continue
			}
			seen[msg.ID] = struct{}{}
			merged = append(merged, msg)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].SentAt.Before(merged[j].SentAt)
	})
	if len(merged) > limit {
		merged = merged[len(merged)-limit:]
	}
	return merged, nil
}

// StreamKeysForSend returns the Redis stream keys a message should be
// appended to so both sides replay a consistent view (§4.4.4).
func StreamKeysForSend(msg domain.RadioMessage, senderInterlockingID uint) []string {
	keys := make([]string, 0, 3)
	keys = append(keys, contract.RadioUserStreamKey(msg.LayoutID, msg.FromUserID))

	if msg.ToUserID != nil {
		keys = append(keys, contract.RadioUserStreamKey(msg.LayoutID, *msg.ToUserID))
		if senderInterlockingID != 0 {
			keys = append(keys, contract.RadioInterlockingStreamKey(msg.LayoutID, senderInterlockingID))
		}
	}
	if msg.ToInterlockingID != nil {
		keys = append(keys, contract.RadioInterlockingStreamKey(msg.LayoutID, *msg.ToInterlockingID))
	}
	return dedupeStrings(keys)
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
