package service

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
)

// DefaultAuditTTL is the stream TTL applied on every write.
// After this duration with no new writes the key expires automatically.
// Each write refreshes the TTL, so active installations never lose data
// unless the stream is completely idle for 24 h.
const DefaultAuditTTL = 24 * time.Hour

// AuditService persists audit entries in a Redis Stream and exposes a
// read path for the audit-log REST endpoint. It implements cmd.AuditPublisher.
type AuditService struct {
	redis *RedisService
	ttl   time.Duration
}

// AuditServiceConfig configures AuditService.
type AuditServiceConfig struct {
	Redis *RedisService
	TTL   time.Duration // defaults to DefaultAuditTTL
}

// NewAuditService constructs an AuditService.
func NewAuditService(cfg AuditServiceConfig) *AuditService {
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultAuditTTL
	}
	return &AuditService{redis: cfg.Redis, ttl: ttl}
}

// Publish implements cmd.AuditPublisher. It appends one entry to the
// global stream and refreshes its TTL. Errors are non-fatal — the
// caller should log them but must not block the primary operation.
func (s *AuditService) Publish(ctx context.Context, layoutID uint, actor cmd.AuditActor, msg string, vars map[string]string) error {
	if s == nil || s.redis == nil {
		return nil
	}
	entry := contract.AuditEntryWire{
		LayoutID:   layoutID,
		ActorID:    actor.UserID,
		ActorLogin: actor.Login,
		Msg:        msg,
		Vars:       vars,
		OccurredAt: time.Now().UTC().UnixMilli(),
	}
	raw, err := contract.MarshalAuditEntry(entry)
	if err != nil {
		return err
	}
	client := s.redis.Client()
	if err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: contract.AuditStreamKey,
		MaxLen: contract.AuditStreamMaxLen,
		Approx: true,
		Values: map[string]any{"payload": string(raw)},
	}).Err(); err != nil {
		return err
	}
	_ = client.Expire(ctx, contract.AuditStreamKey, s.ttl).Err()
	return nil
}

// List returns the most recent entries in newest-first order.
// limit ≤ 0 is treated as 200.
func (s *AuditService) List(ctx context.Context, limit int) ([]contract.AuditEntryWire, error) {
	if s == nil || s.redis == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.redis.Client().XRevRangeN(ctx, contract.AuditStreamKey, "+", "-", int64(limit)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]contract.AuditEntryWire, 0, len(rows))
	for _, row := range rows {
		raw, ok := row.Values["payload"].(string)
		if !ok {
			continue
		}
		e, err := contract.UnmarshalAuditEntry([]byte(raw))
		if err != nil {
			continue
		}
		e.StreamID = row.ID
		entries = append(entries, e)
	}
	return entries, nil
}
