package remotes

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
)

func TestCoordinatorBuildSnapshotIPStickiness(t *testing.T) {
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{IPStickiness: true})

	snap := c.BuildSnapshot()
	if !snap.IPStickiness {
		t.Fatal("expected ipStickiness true when Z21 policy enables it")
	}

	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{IPStickiness: false})
	snap = c.BuildSnapshot()
	if snap.IPStickiness {
		t.Fatal("expected ipStickiness false when no policy enables it")
	}
}

// TestBuildSnapshotSessionExpiresAt verifies the sticky-session expiry is
// surfaced per client so the admin UI can render the eviction countdown.
func TestBuildSnapshotSessionExpiresAt(t *testing.T) {
	addr := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         addr,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IPStickiness:    true,
		StickyIdleEvict: 30 * time.Minute,
	})

	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.5:40001")
	client := addr.Touch(contract.RemoteProtocolZ21, udpAddr, time.Now().UTC(), true)
	addr.SetSession(client.Key, &contract.RemoteSessionWire{Protocol: contract.RemoteProtocolZ21, UserID: 7})

	snap := c.BuildSnapshot()
	if len(snap.Clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(snap.Clients))
	}
	if snap.Clients[0].SessionExpiresAt == 0 {
		t.Fatal("expected SessionExpiresAt set for sticky paired client")
	}

	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{IPStickiness: false, StickyIdleEvict: 30 * time.Minute})
	snap = c.BuildSnapshot()
	if snap.Clients[0].SessionExpiresAt != 0 {
		t.Fatal("non-sticky Z21 pairing dies with presence; SessionExpiresAt must be omitted")
	}
}

type countingPublisher struct{ count int32 }

func (p *countingPublisher) PublishClientsSnapshot(ctx context.Context, snap contract.RemoteClientsSnapshotWire) error {
	atomic.AddInt32(&p.count, 1)
	return nil
}

// TestSweepDoesNotPublishWhenIdle verifies the dirty-flag optimisation:
// an idle sweep with no evict/brake must not write to Redis.
func TestSweepDoesNotPublishWhenIdle(t *testing.T) {
	addr := inbound.NewClientRegistry()
	pub := &countingPublisher{}
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         addr,
		Publisher:        pub,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{IdleEvict: time.Hour})

	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.9:40001")
	_ = addr.Touch(contract.RemoteProtocolZ21, udpAddr, time.Now().UTC(), false)

	// Fresh client, no session, well within idle evict — sweep is a no-op.
	c.sweep(context.Background())
	if got := atomic.LoadInt32(&pub.count); got != 0 {
		t.Fatalf("expected 0 publishes on idle sweep, got %d", got)
	}

	// Forcing a dirty flag (e.g. via markDirty) must publish once.
	c.markDirty()
	c.sweep(context.Background())
	if got := atomic.LoadInt32(&pub.count); got != 1 {
		t.Fatalf("expected 1 publish after dirty sweep, got %d", got)
	}
	// A subsequent idle sweep must not publish again (dirty cleared).
	c.sweep(context.Background())
	if got := atomic.LoadInt32(&pub.count); got != 1 {
		t.Fatalf("expected still 1 publish, got %d", got)
	}
}

// TestSessionSyncEventUpdatesRegistry verifies the WS-1 event-driven sync:
// loco-server publishes a sync event on the per-CS channel and the
// coordinator's registered handler re-syncs the affected client's session
// from Redis — no per-packet GET needed on the daemon hot path.
func TestSessionSyncEventUpdatesRegistry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	store := remotepairing.NewStore(client)

	addr := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         addr,
		Store:            store,
		Publisher:        &countingPublisher{},
	})

	var gotKey atomic.Value
	var called int32
	c.RegisterSessionSyncHandler(contract.RemoteProtocolZ21, func(ctx context.Context, clientKey string) {
		gotKey.Store(clientKey)
		atomic.AddInt32(&called, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	readyCtx, readyCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readyCancel()
	if err := c.WaitSyncSubscriber(readyCtx); err != nil {
		t.Fatalf("sync subscriber not ready: %v", err)
	}

	// Seed an active session in Redis for the client the event will name.
	clientKey := contract.RemoteProtocolZ21 + ":10.0.0.55:40001"
	active := contract.RemoteSessionWire{
		Protocol:         contract.RemoteProtocolZ21,
		UserID:           9,
		AllowAllVehicles: true,
		ClientKey:        clientKey,
	}
	payload, _ := contract.MarshalRemoteSession(active)
	mr.Set(contract.RemotePairingActiveKey(1, 2, clientKey), string(payload))

	if err := store.PublishSessionSync(ctx, 1, 2, clientKey, contract.RemoteSessionSyncScope); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&called) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&called) == 0 {
		t.Fatal("sync handler not invoked after publish")
	}
	if got, _ := gotKey.Load().(string); got != clientKey {
		t.Fatalf("handler received clientKey %q, want %q", got, clientKey)
	}
}

func TestSessionSyncUnpairInvokesOnUnpairHook(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	store := remotepairing.NewStore(client)

	addr := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         addr,
		Store:            store,
		Publisher:        &countingPublisher{},
	})

	var unpairKey atomic.Value
	var unpairCalled int32
	c.RegisterOnUnpair(func(key string) {
		unpairKey.Store(key)
		atomic.AddInt32(&unpairCalled, 1)
	})
	c.RegisterSessionSyncHandler(contract.RemoteProtocolZ21, func(ctx context.Context, clientKey string) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	readyCtx, readyCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readyCancel()
	if err := c.WaitSyncSubscriber(readyCtx); err != nil {
		t.Fatalf("sync subscriber not ready: %v", err)
	}

	clientKey := contract.RemoteProtocolZ21 + ":10.0.0.55:40001"
	if err := store.PublishSessionSync(ctx, 1, 2, clientKey, contract.RemoteSessionSyncUnpair); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&unpairCalled) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&unpairCalled) == 0 {
		t.Fatal("onUnpair hook not invoked after unpair sync")
	}
	if got, _ := unpairKey.Load().(string); got != clientKey {
		t.Fatalf("onUnpair received clientKey %q, want %q", got, clientKey)
	}
}

func TestSessionSyncScopeDoesNotInvokeOnUnpairHook(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	store := remotepairing.NewStore(client)

	addr := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         addr,
		Store:            store,
		Publisher:        &countingPublisher{},
	})

	var unpairCalled int32
	c.RegisterOnUnpair(func(key string) {
		atomic.AddInt32(&unpairCalled, 1)
	})
	c.RegisterSessionSyncHandler(contract.RemoteProtocolZ21, func(ctx context.Context, clientKey string) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	readyCtx, readyCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readyCancel()
	if err := c.WaitSyncSubscriber(readyCtx); err != nil {
		t.Fatalf("sync subscriber not ready: %v", err)
	}

	clientKey := contract.RemoteProtocolZ21 + ":10.0.0.55:40001"
	if err := store.PublishSessionSync(ctx, 1, 2, clientKey, contract.RemoteSessionSyncScope); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&unpairCalled) != 0 {
		t.Fatal("onUnpair hook must not run for scope sync")
	}
}

func TestCoordinatorEvictClearsVirtualLoco(t *testing.T) {
	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
	})
	addr, _ := net.ResolveUDPAddr("udp", "10.0.0.5:40001")
	client := reg.Touch(contract.RemoteProtocolZ21, addr, time.Now().UTC(), false)
	c.VirtualLocos().SetSpeed(client.Key, 31, 10, true)
	c.Evict(context.Background(), client.Key)
	if c.VirtualLocos().HasClient(client.Key) {
		t.Fatal("expected virtual loco state cleared on evict")
	}
}

func TestSweepUnpairsIdleZ21WithoutStickiness(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := remotepairing.NewStore(rdb)

	ctx := context.Background()
	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
		Store:            store,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IdleEvict:       60 * time.Second,
		StickyIdleEvict: 72 * time.Hour,
	})
	past := time.Now().UTC().Add(-2 * time.Minute)
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:40001")
	client := reg.Touch(contract.RemoteProtocolZ21, udpAddr, past, false)
	if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, client.Key, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: client.Key})

	c.sweep(ctx)
	if _, ok := reg.Get(client.Key); ok {
		t.Fatal("idle Z21 without stickiness must leave the registry")
	}
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, client.Key); err != nil || ok {
		t.Fatalf("idle Z21 without stickiness must unpair Redis: ok=%v err=%v", ok, err)
	}
}

func TestSweepKeepsZ21WithinIdleWindow(t *testing.T) {
	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IdleEvict:       60 * time.Second,
		StickyIdleEvict: 72 * time.Hour,
	})
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:40001")
	client := reg.Touch(contract.RemoteProtocolZ21, udpAddr, time.Now().UTC(), false)
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: client.Key})

	c.sweep(context.Background())
	if _, ok := reg.Get(client.Key); !ok {
		t.Fatal("Z21 still inside the idle window must stay")
	}
}

func TestSweepKeepsStickyZ21BeyondIdleWindow(t *testing.T) {
	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IdleEvict:       60 * time.Second,
		StickyIdleEvict: 72 * time.Hour,
		IPStickiness:    true,
	})
	past := time.Now().UTC().Add(-2 * time.Minute)
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:40001")
	client := reg.Touch(contract.RemoteProtocolZ21, udpAddr, past, true)
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: client.Key})

	c.sweep(context.Background())
	if _, ok := reg.Get(client.Key); !ok {
		t.Fatal("sticky Z21 must survive IdleEvict and wait for StickyIdleEvict")
	}
}

func TestSweepDropsWithrottlePresenceWithoutUnpair(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := remotepairing.NewStore(rdb)

	ctx := context.Background()
	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "withrottle:4242"
	if _, ok, _, err := store.PairViaWithrottleCode(ctx, 1, 2, req.PairingCode, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}

	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
		Store:            store,
	})
	c.RegisterPolicy(contract.RemoteProtocolWithrottle, ProtocolPolicy{
		IdleEvict:         60 * time.Second,
		StickyIdleEvict:   72 * time.Hour,
		SweepKeepsPairing: true,
	})
	past := time.Now().UTC().Add(-2 * time.Minute)
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:12090")
	client := reg.TouchByEndpoint(contract.RemoteProtocolWithrottle, "4242", udpAddr, past)
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: clientKey})

	c.sweep(ctx)
	if _, ok := reg.Get(client.Key); ok {
		t.Fatal("idle WiThrottle must leave the registry")
	}
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, clientKey); err != nil || !ok {
		t.Fatalf("WiThrottle Redis pairing must survive sweep: ok=%v err=%v", ok, err)
	}
}

func TestBuildSnapshotOmitsExpiryForNonStickyZ21(t *testing.T) {
	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IdleEvict:       60 * time.Second,
		StickyIdleEvict: 72 * time.Hour,
	})
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:40001")
	client := reg.Touch(contract.RemoteProtocolZ21, udpAddr, time.Now().UTC(), false)
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: client.Key})

	snap := c.BuildSnapshot()
	if len(snap.Clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(snap.Clients))
	}
	if snap.Clients[0].SessionExpiresAt != 0 {
		t.Fatalf("SessionExpiresAt=%d, want 0", snap.Clients[0].SessionExpiresAt)
	}
}

func TestBuildSnapshotExpiryMatchesSweep(t *testing.T) {
	stickyTTL := 30 * time.Minute
	now := time.Now().UTC()

	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
	})
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{
		IdleEvict:       60 * time.Second,
		StickyIdleEvict: stickyTTL,
		IPStickiness:    true,
	})
	c.RegisterPolicy(contract.RemoteProtocolWithrottle, ProtocolPolicy{
		IdleEvict:         120 * time.Second,
		StickyIdleEvict:   stickyTTL,
		SweepKeepsPairing: true,
	})

	z21Addr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:40001")
	z21 := reg.Touch(contract.RemoteProtocolZ21, z21Addr, now, true)
	reg.SetSession(z21.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: z21.Key})

	wtAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.9:12090")
	wt := reg.TouchByEndpoint(contract.RemoteProtocolWithrottle, "4242", wtAddr, now)
	reg.SetSession(wt.Key, &contract.RemoteSessionWire{UserID: 8, ClientKey: wt.Key})

	want := now.Add(stickyTTL).UnixMilli()
	snap := c.BuildSnapshot()
	if len(snap.Clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(snap.Clients))
	}
	for _, row := range snap.Clients {
		if row.SessionExpiresAt != want {
			t.Fatalf("%s SessionExpiresAt=%d, want %d (pairing TTL)", row.Protocol, row.SessionExpiresAt, want)
		}
	}
}

func TestDropPresenceKeepsRedisPairing(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := remotepairing.NewStore(rdb)

	ctx := context.Background()
	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "withrottle:4242"
	if _, ok, _, err := store.PairViaWithrottleCode(ctx, 1, 2, req.PairingCode, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}

	reg := inbound.NewClientRegistry()
	c := NewCoordinator(CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 2,
		Registry:         reg,
		Store:            store,
	})
	udpAddr, _ := net.ResolveUDPAddr("udp", "10.0.0.8:12090")
	client := reg.TouchByEndpoint(contract.RemoteProtocolWithrottle, "4242", udpAddr, time.Now().UTC())
	reg.SetSession(client.Key, &contract.RemoteSessionWire{UserID: 9, ClientKey: clientKey})

	c.DropPresence(ctx, client.Key)
	if _, ok := reg.Get(client.Key); ok {
		t.Fatal("presence should be gone")
	}
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, clientKey); err != nil || !ok {
		t.Fatalf("redis pairing must survive drop: ok=%v err=%v", ok, err)
	}
}
