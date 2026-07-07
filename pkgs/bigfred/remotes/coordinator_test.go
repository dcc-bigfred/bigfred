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
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
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

	// Non-sticky policy must omit SessionExpiresAt.
	c.RegisterPolicy(contract.RemoteProtocolZ21, ProtocolPolicy{IPStickiness: false, StickyIdleEvict: 30 * time.Minute})
	snap = c.BuildSnapshot()
	if snap.Clients[0].SessionExpiresAt != 0 {
		t.Fatalf("expected no SessionExpiresAt for non-sticky, got %d", snap.Clients[0].SessionExpiresAt)
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
