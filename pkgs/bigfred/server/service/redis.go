package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

const redisProbeTimeout = 300 * time.Millisecond

// RedisManagement describes whether supervisord should spawn redis-server.
type RedisManagement struct {
	// Managed is true when loco-server should run redis under supervisord.
	Managed bool
	// Source is one of "managed", "explicit-external", or "auto-detected".
	Source string
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

// RedisReachable reports whether addr accepts a Redis PING within timeout.
func RedisReachable(ctx context.Context, cfg RedisServiceConfig, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = redisProbeTimeout
	}
	addr := cfg.Addr
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	probe := redis.NewClient(&redis.Options{
		Addr:     addr,
		DB:       cfg.DB,
		Password: cfg.Password,
	})
	defer probe.Close()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return probe.Ping(probeCtx).Err() == nil
}

// ResolveRedisManagement decides whether supervisord should spawn redis-server.
// Explicit --redis-external always skips the managed daemon. When autoDetect
// is enabled, an existing instance at cfg.Addr is reused instead of spawning.
func ResolveRedisManagement(ctx context.Context, cfg RedisServiceConfig, explicitExternal, autoDetect bool) RedisManagement {
	if explicitExternal {
		return RedisManagement{Managed: false, Source: "explicit-external"}
	}
	if autoDetect && RedisReachable(ctx, cfg, redisProbeTimeout) {
		return RedisManagement{Managed: false, Source: "auto-detected"}
	}
	return RedisManagement{Managed: true, Source: "managed"}
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

// PublishLayoutAllowedVehicles stores the snapshot and notifies
// subscribers on bigfred:layout:<id>:allowed_vehicles.
func (r *RedisService) PublishLayoutAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) error {
	if r == nil || snap.LayoutID == 0 {
		return nil
	}
	raw, err := contract.Marshal(snap)
	if err != nil {
		return err
	}
	key := contract.AllowedVehiclesKey(snap.LayoutID)
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, key, raw, 0)
	pipe.Publish(ctx, key, raw)
	_, err = pipe.Exec(ctx)
	return err
}

// PublishLayoutDefinedTrains stores the snapshot and notifies
// subscribers on bigfred:layout:<id>:defined_trains.
func (r *RedisService) PublishLayoutDefinedTrains(ctx context.Context, snap contract.DefinedTrains) error {
	if r == nil || snap.LayoutID == 0 {
		return nil
	}
	raw, err := contract.Marshal(snap)
	if err != nil {
		return err
	}
	key := contract.DefinedTrainsKey(snap.LayoutID)
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, key, raw, 0)
	pipe.Publish(ctx, key, raw)
	_, err = pipe.Exec(ctx)
	return err
}

// Close drains the underlying connection pool.
func (r *RedisService) Close() error { return r.client.Close() }

// GetAllowedVehicles reads the cached layout roster snapshot written by
// loco-server. Returns an empty snapshot when the key is missing.
func (r *RedisService) GetAllowedVehicles(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	if r == nil || layoutID == 0 {
		return contract.AllowedVehicles{}, nil
	}
	raw, err := r.client.Get(ctx, contract.AllowedVehiclesKey(layoutID)).Result()
	if err == redis.Nil {
		return contract.AllowedVehicles{LayoutID: layoutID}, nil
	}
	if err != nil {
		return contract.AllowedVehicles{}, err
	}
	return contract.UnmarshalAllowedVehicles([]byte(raw))
}

// PublishLayoutRadioStop notifies every dcc-bus daemon on the layout
// to run the roster halt (§4.6.4).
func (r *RedisService) PublishLayoutRadioStop(ctx context.Context, layoutID uint, cmd contract.RadioStopCommandWire) error {
	if r == nil || layoutID == 0 {
		return nil
	}
	raw, err := contract.BuildRadioStopCommandPayload(cmd)
	if err != nil {
		return err
	}
	return r.client.Publish(ctx, contract.LayoutRadioStopChannel(layoutID), raw).Err()
}
