package remotes

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

const (
	defaultSweeperInterval   = 3 * time.Second
	defaultClientsPublishMin = 2 * time.Second
	defaultIdleEvict         = 60 * time.Second
	defaultStickyIdleEvict   = 30 * time.Minute
	// syncSubscribeRetry is the backoff before re-subscribing to the
	// session-sync channel after a receive error.
	syncSubscribeRetry = 2 * time.Second
	// seenFlushInterval is the cadence of the batched lastSeenAt flush
	// (WS-1b). 1s balances Redis write rate against lastSeenAt precision.
	seenFlushInterval = 1 * time.Second
)

// ProtocolPolicy configures idle eviction for one inbound protocol.
type ProtocolPolicy struct {
	IdleEvict       time.Duration
	StickyIdleEvict time.Duration
	IPStickiness    bool
	// HeartbeatTimeout, when > 0, evicts a paired client whose last
	// activity is older than the timeout. Used by line-oriented protocols
	// with a dead-man's switch (e.g. WiThrottle heartbeat). Z21 leaves
	// this zero and relies on IdleEvict/StickyIdleEvict.
	// TODO(withrottle): on expiry, emit a handset emergency stop before
	// evicting instead of a plain idle evict.
	HeartbeatTimeout time.Duration
}

// CoordinatorConfig wires the shared inbound handset coordinator.
type CoordinatorConfig struct {
	LayoutID         uint
	CommandStationID uint
	Registry         *inbound.ClientRegistry
	Store            *remotepairing.Store
	Drive            HandsetDrivePort
	Publisher        ClientsSnapshotPublisher
	Log              *logrus.Logger
	SweeperInterval  time.Duration
	PublishMin       time.Duration
}

// Coordinator runs idle sweeps, handset braking, and snapshot publishing
// for every inbound protocol on one command station.
type Coordinator struct {
	cfg          CoordinatorConfig
	registry     *inbound.ClientRegistry
	policies     map[string]ProtocolPolicy
	policiesMu   sync.RWMutex
	syncHandlers map[string]SessionSyncHandler
	evictMu      sync.RWMutex
	onEvict      []func(string)
	pubMu        sync.Mutex
	lastPub      time.Time
	dirty        bool
	runOnce      sync.Once
	bgWg         sync.WaitGroup
	// syncSubReady is closed by markSyncSubReady once the session-sync
	// subscription is live. Allocated in NewCoordinator when Store is set;
	// nil otherwise. Lets tests await readiness instead of racing PUBLISH.
	syncSubReady chan struct{}
	syncSubOnce  sync.Once
}

// NewCoordinator returns a coordinator that is not yet running.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	if cfg.Registry == nil {
		cfg.Registry = inbound.NewClientRegistry()
	}
	if cfg.SweeperInterval == 0 {
		cfg.SweeperInterval = defaultSweeperInterval
	}
	if cfg.PublishMin == 0 {
		cfg.PublishMin = defaultClientsPublishMin
	}
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	cfg.Log = log
	c := &Coordinator{
		cfg:      cfg,
		registry: cfg.Registry,
		policies: make(map[string]ProtocolPolicy),
	}
	if cfg.Store != nil {
		c.syncSubReady = make(chan struct{})
	}
	return c
}

// Registry returns the shared client registry.
func (c *Coordinator) Registry() *inbound.ClientRegistry {
	return c.registry
}

// RegisterOnEvict adds a hook invoked after a client is evicted. Safe to
// call before or while Run is active.
func (c *Coordinator) RegisterOnEvict(fn func(key string)) {
	if fn == nil {
		return
	}
	c.evictMu.Lock()
	c.onEvict = append(c.onEvict, fn)
	c.evictMu.Unlock()
}

// RegisterPolicy sets sweep behaviour for one protocol.
func (c *Coordinator) RegisterPolicy(protocol string, policy ProtocolPolicy) {
	if policy.IdleEvict == 0 {
		policy.IdleEvict = defaultIdleEvict
	}
	if policy.StickyIdleEvict == 0 {
		policy.StickyIdleEvict = defaultStickyIdleEvict
	}
	c.policiesMu.Lock()
	c.policies[protocol] = policy
	c.policiesMu.Unlock()
}

// SessionSyncHandler reconciles one client's in-process session after a
// REST mutation published on the sync channel. Registered per protocol by
// the gateway (e.g. z21server clears its wire pairing buffer on unpair).
type SessionSyncHandler func(ctx context.Context, clientKey string)

// RegisterSessionSyncHandler wires the per-protocol handler invoked when
// loco-server signals a session change. Safe to call before Run.
func (c *Coordinator) RegisterSessionSyncHandler(protocol string, h SessionSyncHandler) {
	if h == nil || protocol == "" {
		return
	}
	c.policiesMu.Lock()
	if c.syncHandlers == nil {
		c.syncHandlers = map[string]SessionSyncHandler{}
	}
	c.syncHandlers[protocol] = h
	c.policiesMu.Unlock()
}

// Run sweeps until ctx is cancelled and consumes session-sync events from
// loco-server so REST mutations (unpair / scope update) reach the daemon's
// in-process registry without per-packet Redis reads.
func (c *Coordinator) Run(ctx context.Context) {
	c.runOnce.Do(func() {
		if c.cfg.Store != nil {
			c.bgWg.Add(2)
			go func() {
				defer c.bgWg.Done()
				c.runSessionSyncSubscriber(ctx)
			}()
			go func() {
				defer c.bgWg.Done()
				c.runSeenFlusher(ctx)
			}()
		}
	})
	ticker := time.NewTicker(c.cfg.SweeperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.bgWg.Wait()
			return
		case <-ticker.C:
			c.sweep(ctx)
		}
	}
}

// WaitSyncSubscriber blocks until the session-sync pub/sub subscription is
// active, or ctx is cancelled. No-op when Store is not configured.
func (c *Coordinator) WaitSyncSubscriber(ctx context.Context) error {
	if c.syncSubReady == nil {
		return nil
	}
	select {
	case <-c.syncSubReady:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Coordinator) markSyncSubReady() {
	c.syncSubOnce.Do(func() {
		if c.syncSubReady != nil {
			close(c.syncSubReady)
		}
	})
}

// runSeenFlusher drains pending lastSeenAt updates and writes them to
// Redis in batch (one round-trip per protocol group per tick) instead of
// a per-packet SET. Loses at most one flush interval (1s) of lastSeenAt
// precision on a crash — acceptable given the 60s/30min idle windows.
func (c *Coordinator) runSeenFlusher(ctx context.Context) {
	ticker := time.NewTicker(seenFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.flushSeen(ctx)
		}
	}
}

func (c *Coordinator) flushSeen(ctx context.Context) {
	if c.cfg.Store == nil {
		return
	}
	dirty := c.registry.PeekSeenDirty()
	if len(dirty) == 0 {
		return
	}
	// Group by protocol so each group can use its policy's TTL (sticky
	// sessions refresh the idle window; non-sticky preserve existing TTL).
	type bucket struct {
		keys []string
		ts   []int64
		ttl  time.Duration
	}
	buckets := make(map[string]*bucket)
	for key, ts := range dirty {
		protocol, _ := inbound.ParseClientKey(key)
		policy := c.policyFor(protocol)
		b, ok := buckets[protocol]
		if !ok {
			b = &bucket{}
			buckets[protocol] = b
			if policy.IPStickiness {
				b.ttl = policy.StickyIdleEvict
			}
		}
		b.keys = append(b.keys, key)
		b.ts = append(b.ts, ts)
	}
	var failed []string
	for _, b := range buckets {
		if err := c.cfg.Store.TouchSeenBatch(ctx, c.cfg.LayoutID, c.cfg.CommandStationID, b.keys, b.ts, b.ttl); err != nil {
			c.cfg.Log.WithError(err).Debug("remote seen batch flush failed")
			failed = append(failed, b.keys...)
		}
	}
	if len(failed) == 0 {
		keys := make([]string, 0, len(dirty))
		for key := range dirty {
			keys = append(keys, key)
		}
		c.registry.ClearSeenDirty(keys)
		return
	}
	okKeys := make([]string, 0, len(dirty)-len(failed))
	failSet := make(map[string]struct{}, len(failed))
	for _, key := range failed {
		failSet[key] = struct{}{}
	}
	for key := range dirty {
		if _, bad := failSet[key]; !bad {
			okKeys = append(okKeys, key)
		}
	}
	c.registry.ClearSeenDirty(okKeys)
}

// runSessionSyncSubscriber drains the per-CS sync channel until ctx cancels.
// On a non-fatal receive error the subscription is retried with backoff.
func (c *Coordinator) runSessionSyncSubscriber(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		sub, err := c.cfg.Store.SubscribeSessionSync(ctx, c.cfg.LayoutID, c.cfg.CommandStationID)
		if err != nil {
			c.cfg.Log.WithError(err).Warn("remote sync subscribe failed; retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(syncSubscribeRetry):
			}
			continue
		}
		c.markSyncSubReady()
		c.consumeSync(ctx, sub)
	}
}

func (c *Coordinator) consumeSync(ctx context.Context, sub *redis.PubSub) {
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			go func(m *redis.Message) {
				defer func() {
					if r := recover(); r != nil {
						c.cfg.Log.WithField("panic", r).Error("remote sync handler panic")
					}
				}()
				c.handleSyncMessage(ctx, m)
			}(msg)
		}
	}
}

func (c *Coordinator) handleSyncMessage(ctx context.Context, msg *redis.Message) {
	ev, err := contract.UnmarshalRemoteSessionSync([]byte(msg.Payload))
	if err != nil {
		c.cfg.Log.WithError(err).Debug("remote sync: bad payload")
		return
	}
	if ev.ClientKey == "" {
		return
	}
	protocol, _ := inbound.ParseClientKey(ev.ClientKey)
	c.policiesMu.RLock()
	handler := c.syncHandlers[protocol]
	c.policiesMu.RUnlock()
	if handler != nil {
		handler(ctx, ev.ClientKey)
	} else {
		c.syncPairedClientGeneric(ctx, ev.ClientKey)
		return
	}
	c.registry.MarkSynced(ev.ClientKey)
	c.markDirty()
	c.PublishSnapshotThrottled(ctx)
}

// syncPairedClientGeneric is the fallback re-sync for protocols without a
// registered handler: fetch the active session from Redis and mirror it
// into the registry. Protocol-specific cleanup (e.g. Z21 wire buffer) is
// the registered handler's responsibility.
func (c *Coordinator) syncPairedClientGeneric(ctx context.Context, clientKey string) {
	if c.cfg.Store == nil {
		return
	}
	active, ok, err := c.cfg.Store.GetActiveByClientKey(ctx, c.cfg.LayoutID, c.cfg.CommandStationID, clientKey)
	if err != nil {
		c.cfg.Log.WithError(err).WithField("client", clientKey).Debug("remote sync: get active")
		return
	}
	if !ok {
		c.registry.SetSession(clientKey, nil)
	} else {
		c.registry.SetSession(clientKey, &active)
	}
	c.registry.MarkSynced(clientKey)
	c.markDirty()
	c.PublishSnapshotThrottled(ctx)
}

// NoteActivity clears idle-brake and throttles snapshot publish.
func (c *Coordinator) NoteActivity(ctx context.Context, clientKey string) {
	if c.registry.IsPaired(clientKey) && c.registry.IdleBraked(clientKey) {
		c.registry.ClearIdleBraked(clientKey)
		c.markDirty()
	}
	c.PublishSnapshotThrottled(ctx)
}

// markDirty flags the snapshot as changed so the next sweep publishes it
// even when a throttled publish was suppressed.
func (c *Coordinator) markDirty() {
	c.pubMu.Lock()
	c.dirty = true
	c.pubMu.Unlock()
}

// PublishSnapshotThrottled stores and fans out a clients snapshot.
func (c *Coordinator) PublishSnapshotThrottled(ctx context.Context) {
	if c.cfg.Publisher == nil {
		return
	}
	c.pubMu.Lock()
	if !c.lastPub.IsZero() && time.Since(c.lastPub) < c.cfg.PublishMin {
		c.pubMu.Unlock()
		return
	}
	c.lastPub = time.Now().UTC()
	snap := c.BuildSnapshot()
	c.dirty = false
	c.pubMu.Unlock()
	_ = c.cfg.Publisher.PublishClientsSnapshot(ctx, snap)
}

// PublishSnapshot stores and fans out a clients snapshot immediately.
func (c *Coordinator) PublishSnapshot(ctx context.Context) error {
	if c.cfg.Publisher == nil {
		return nil
	}
	c.pubMu.Lock()
	snap := c.BuildSnapshot()
	c.dirty = false
	c.pubMu.Unlock()
	return c.cfg.Publisher.PublishClientsSnapshot(ctx, snap)
}

// publishIfDirty publishes the snapshot only when something changed since
// the last publish. Used by the sweeper to avoid a constant Redis write
// every tick when the registry is idle.
func (c *Coordinator) publishIfDirty(ctx context.Context) {
	if c.cfg.Publisher == nil {
		return
	}
	c.pubMu.Lock()
	if !c.dirty {
		c.pubMu.Unlock()
		return
	}
	c.lastPub = time.Now().UTC()
	snap := c.BuildSnapshot()
	c.dirty = false
	c.pubMu.Unlock()
	_ = c.cfg.Publisher.PublishClientsSnapshot(ctx, snap)
}

// BuildSnapshot returns the current handset presence view.
func (c *Coordinator) BuildSnapshot() contract.RemoteClientsSnapshotWire {
	now := time.Now().UTC()
	clients := c.registry.Snapshot()
	out := make([]contract.RemoteClientWire, 0, len(clients))
	for _, cl := range clients {
		policy := c.policyFor(cl.Protocol)
		w := contract.RemoteClientWire{
			ClientKey:   cl.Key,
			Protocol:    cl.Protocol,
			IP:          cl.Addr.IP.String(),
			Port:        cl.Addr.Port,
			Paired:      cl.Session != nil,
			LastSeenAt:  cl.LastSeen.UnixMilli(),
			ConnectedAt: cl.ConnectedAt.UnixMilli(),
			IdleBraked:  cl.IdleBraked,
		}
		if cl.Session != nil {
			w.UserID = cl.Session.UserID
			// Surface the sticky-session expiry so the admin UI can show
			// when an IP-sticky handset will be evicted without activity.
			// Non-sticky sessions omit the field (UI only renders it when
			// ipStickiness is on).
			if policy.IPStickiness {
				w.SessionExpiresAt = cl.LastSeen.Add(policy.StickyIdleEvict).UnixMilli()
			}
		}
		out = append(out, w)
	}
	return contract.RemoteClientsSnapshotWire{
		LayoutID:         c.cfg.LayoutID,
		CommandStationID: c.cfg.CommandStationID,
		IPStickiness:     c.snapshotIPStickiness(),
		UpdatedAt:        now.UnixMilli(),
		Clients:          out,
	}
}

func (c *Coordinator) snapshotIPStickiness() bool {
	c.policiesMu.RLock()
	defer c.policiesMu.RUnlock()
	for _, p := range c.policies {
		if p.IPStickiness {
			return true
		}
	}
	return false
}

func (c *Coordinator) sweep(ctx context.Context) {
	now := time.Now().UTC()
	for _, cl := range c.registry.Snapshot() {
		idle := now.Sub(cl.LastSeen)
		policy := c.policyFor(cl.Protocol)
		if cl.Session != nil {
			brakeAfter := time.Duration(contract.NormaliseHandsetBrakeSecs(cl.Session.HandsetBrakeSecs)) * time.Second
			if idle >= brakeAfter && !cl.IdleBraked {
				c.brakeHandsetLocos(ctx, cl)
				c.registry.SetIdleBraked(cl.Key, true)
				c.markDirty()
			}
		}
		evictAfter := policy.IdleEvict
		if policy.IPStickiness && cl.Session != nil {
			evictAfter = policy.StickyIdleEvict
		}
		if policy.HeartbeatTimeout > 0 && cl.Session != nil && cl.HeartbeatMonitor && idle >= policy.HeartbeatTimeout {
			c.estopHandsetLocos(ctx, cl)
			evictAfter = policy.HeartbeatTimeout
		}
		if idle >= evictAfter {
			c.evictClient(ctx, cl.Key)
		}
	}
	// Only write to Redis when the registry actually changed this tick.
	c.publishIfDirty(ctx)
}

func (c *Coordinator) policyFor(protocol string) ProtocolPolicy {
	c.policiesMu.RLock()
	p, ok := c.policies[protocol]
	c.policiesMu.RUnlock()
	if ok {
		return p
	}
	return ProtocolPolicy{
		IdleEvict:       defaultIdleEvict,
		StickyIdleEvict: defaultStickyIdleEvict,
	}
}

func (c *Coordinator) brakeHandsetLocos(ctx context.Context, client *inbound.Client) {
	if c.cfg.Drive == nil || client.Session == nil {
		return
	}
	p := client.Session
	scope := DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
	session := HandsetSession{ClientKey: client.Key, UserID: p.UserID}
	addrs := c.cfg.Drive.CollectHandsetDriveTargets(ctx, p.UserID, client.SubscribedLocos, scope)
	if len(addrs) == 0 {
		return
	}
	c.cfg.Drive.ApplyHandsetIdleBrake(ctx, session, client.SubscribedLocos, scope)
	c.cfg.Log.WithFields(logrus.Fields{
		"client":   client.Key,
		"userId":   p.UserID,
		"protocol": client.Protocol,
		"addrs":    addrs,
	}).Info("handset idle brake")
}

func (c *Coordinator) estopHandsetLocos(ctx context.Context, client *inbound.Client) {
	if c.cfg.Drive == nil || client.Session == nil {
		return
	}
	p := client.Session
	scope := DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
	session := HandsetSession{ClientKey: client.Key, UserID: p.UserID}
	addrs := c.cfg.Drive.CollectHandsetDriveTargets(ctx, p.UserID, client.SubscribedLocos, scope)
	for _, addr := range addrs {
		c.cfg.Drive.ApplyHandsetPilotEStop(ctx, session, addr)
	}
	if len(addrs) > 0 {
		c.cfg.Log.WithFields(logrus.Fields{
			"client":   client.Key,
			"userId":   p.UserID,
			"protocol": client.Protocol,
			"addrs":    addrs,
		}).Info("handset heartbeat estop")
	}
}

func (c *Coordinator) evictClient(ctx context.Context, key string) {
	storeCtx := ctx
	var cancel context.CancelFunc
	if ctx.Err() != nil {
		storeCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if c.cfg.Store != nil {
		if err := c.cfg.Store.Unpair(storeCtx, c.cfg.LayoutID, c.cfg.CommandStationID, key); err != nil {
			c.cfg.Log.WithError(err).WithField("client", key).Debug("unpair on evict")
		}
	}
	c.registry.Remove(key)
	c.evictMu.RLock()
	hooks := append([]func(string){}, c.onEvict...)
	c.evictMu.RUnlock()
	for _, fn := range hooks {
		fn(key)
	}
	c.markDirty()
	c.PublishSnapshotThrottled(ctx)
}

// Evict removes one client and unpairs it in Redis.
func (c *Coordinator) Evict(ctx context.Context, key string) {
	c.evictClient(ctx, key)
}
