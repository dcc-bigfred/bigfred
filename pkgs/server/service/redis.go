package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// LayoutRosterInvalidateChannel is the pub/sub topic every dcc-bus
// daemon for layout L subscribes to. A publish forces a SQLite
// roster re-read (layout_vehicles → DCC addresses).
func LayoutRosterInvalidateChannel(layoutID uint) string {
	return fmt.Sprintf("bigfred:layout:%d:invalidate", layoutID)
}

// RedisService wraps a singleton go-redis client used by the rest of
// the server (DccBusService, ws hub fan-in, etc.) so test code and
// production share one connection pool.
//
// The struct is intentionally minimal: it owns no state beyond the
// underlying *redis.Client and a few lazily-derived helpers. Higher-
// level abstractions (port pool, lease cache) live in the services
// that need them.
type RedisService struct {
	client *redis.Client
	addr   string
}

// RedisServiceConfig configures the connection. Addr is a `host:port`
// string (e.g. "127.0.0.1:6379"); DB is the logical database number
// (default 0). Password is optional.
type RedisServiceConfig struct {
	Addr     string
	DB       int
	Password string
}

// NewRedisService returns a service bound to a brand-new *redis.Client.
// The client is created eagerly but no connection is opened until the
// first command — call Ping (or WaitReady) to verify the daemon is
// dial-able.
func NewRedisService(cfg RedisServiceConfig) *RedisService {
	addr := cfg.Addr
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		DB:       cfg.DB,
		Password: cfg.Password,
	})
	return &RedisService{client: client, addr: addr}
}

// Client returns the raw go-redis client. Most callers should prefer
// the typed helpers on this struct; the raw client is exposed for
// tests and packages that need direct pub/sub access.
func (r *RedisService) Client() *redis.Client { return r.client }

// Addr returns the configured `host:port` of the Redis server. Used
// by dcc-bus daemons that need to dial the same instance.
func (r *RedisService) Addr() string { return r.addr }

// Ping issues a PING command and returns the round-trip error.
func (r *RedisService) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// WaitReady polls Ping until it succeeds or the deadline expires.
// Used during loco-server bootstrap to block until the supervisord-
// managed redis-server is accepting connections — without it, the
// first dcc-bus enqueue would race the daemon's startup and explode
// with `connection refused`.
func (r *RedisService) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := r.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("redis not ready")
	}
	return fmt.Errorf("redis not ready after %s: %w", timeout, lastErr)
}

// PublishLayoutRosterInvalidate notifies dcc-bus daemons serving
// layoutID that the vehicle roster (or DCC addresses) changed.
func (r *RedisService) PublishLayoutRosterInvalidate(ctx context.Context, layoutID uint) error {
	if r == nil || layoutID == 0 {
		return nil
	}
	return r.client.Publish(ctx, LayoutRosterInvalidateChannel(layoutID), "{}").Err()
}

// Close drains the underlying connection pool.
func (r *RedisService) Close() error { return r.client.Close() }
