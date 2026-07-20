package remotepairing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

func newTestStore(t *testing.T) (*remotepairing.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return remotepairing.NewStore(client), mr
}

func TestCreateAndPairViaCV3CV4(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           9,
		UserLogin:        "alice",
		VehicleIDs:       []string{"V-1"},
		AllowedAddrs:     []uint16{3},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if req.UserLogin != "alice" {
		t.Fatalf("pending UserLogin=%q", req.UserLogin)
	}
	if !contract.ValidPairingCV(req.PairingCV3) || !contract.ValidPairingCV(req.PairingCV4) {
		t.Fatalf("invalid pair: %+v", req)
	}

	pending, ok, err := store.GetPendingByUser(ctx, 1, 2, 9)
	if err != nil || !ok {
		t.Fatalf("pending: ok=%v err=%v", ok, err)
	}
	if pending.PairingCV3 != req.PairingCV3 {
		t.Fatalf("pending mismatch: %+v", pending)
	}

	now := contract.NowMS()
	clientKey := contract.RemoteProtocolZ21 + ":10.0.0.1:21105"
	active, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, clientKey, now)
	if err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	if active.ClientKey != clientKey || active.UserID != 9 {
		t.Fatalf("active: %+v", active)
	}
	if active.UserLogin != "alice" {
		t.Fatalf("active UserLogin=%q want alice", active.UserLogin)
	}

	_, ok, err = store.GetPendingByUser(ctx, 1, 2, 9)
	if err != nil || ok {
		t.Fatalf("pending should be gone: ok=%v err=%v", ok, err)
	}

	loaded, ok, err := store.GetActiveByClientKey(ctx, 1, 2, clientKey)
	if err != nil || !ok || loaded.LastSeenAt != now {
		t.Fatalf("load active: ok=%v err=%v active=%+v", ok, err, loaded)
	}
}

func TestPairRejectsInvalidCV(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	_, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, 99, 145, "z21:1.2.3.4:1", contract.NowMS())
	if err == nil || ok {
		t.Fatalf("expected invalid CV error, ok=%v err=%v", ok, err)
	}
}

func TestTouchSeenAndUnpair(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           4,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "z21:192.168.1.10:40000"
	if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}

	touch := contract.NowMS() + int64(time.Second/time.Millisecond)
	if err := store.TouchSeen(ctx, 1, 2, clientKey, touch, 0); err != nil {
		t.Fatal(err)
	}
	active, ok, err := store.GetActiveByClientKey(ctx, 1, 2, clientKey)
	if err != nil || !ok || active.LastSeenAt != touch {
		t.Fatalf("touch: %+v ok=%v err=%v", active, ok, err)
	}

	if err := store.Unpair(ctx, 1, 2, clientKey); err != nil {
		t.Fatal(err)
	}
	_, ok, err = store.GetActiveByClientKey(ctx, 1, 2, clientKey)
	if err != nil || ok {
		t.Fatalf("expected unpaired, ok=%v err=%v", ok, err)
	}
}

func TestCreateReplacesPriorPending(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	in := remotepairing.CreateZ21PairingInput{
		LayoutID:         3,
		CommandStationID: 4,
		UserID:           1,
		AllowedAddrs:     []uint16{10},
	}
	first, err := store.CreateZ21PairingRequest(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateZ21PairingRequest(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.PairingCV3 == second.PairingCV3 && first.PairingCV4 == second.PairingCV4 {
		t.Fatalf("expected new pair, got same: %+v", second)
	}
	_, ok, _, err := store.PairViaCV3CV4(ctx, 3, 4, first.PairingCV3, first.PairingCV4, "z21:1.1.1.1:1", contract.NowMS())
	if err != nil || ok {
		t.Fatalf("stale req should not pair: ok=%v err=%v", ok, err)
	}
}

func TestCreateRejectsWhenUserAlreadyPaired(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	in := remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           7,
		AllowAllVehicles: true,
	}
	req, err := store.CreateZ21PairingRequest(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "z21:10.0.0.2:21105"
	if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 1, req.PairingCV3, req.PairingCV4, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	_, err = store.CreateZ21PairingRequest(ctx, in)
	if !errors.Is(err, remotepairing.ErrUserAlreadyPaired) {
		t.Fatalf("expected ErrUserAlreadyPaired, got %v", err)
	}
}

// TestDedupKeyHasTTL guards against the unbounded-growth regression: the
// reqdedup SET must expire so abandoned pending codes do not eventually
// exhaust the 2500-pair space and make pairing impossible.
func TestDedupKeyHasTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()
	in := remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           11,
		AllowAllVehicles: true,
	}
	if _, err := store.CreateZ21PairingRequest(ctx, in); err != nil {
		t.Fatal(err)
	}
	dedupKey := contract.RemotePairingReqDedupKey(1, 1, contract.RemoteProtocolZ21)
	ttl := mr.TTL(dedupKey)
	if ttl <= 0 {
		t.Fatalf("dedup SET has no TTL (would accumulate labels forever): ttl=%v", ttl)
	}
}

// TestUpdateSessionScopePreservesTTL ensures a PATCH on a sticky session
// does not strip its idle expiry (the previous RMW SET-with-TTL-0 path).
func TestUpdateSessionScopePreservesTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()
	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           21,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "z21:10.0.0.21:21105"
	if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, clientKey, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	activeKey := contract.RemotePairingActiveKey(1, 2, clientKey)
	const stickyTTL = 30 * time.Minute
	if err := store.TouchSeen(ctx, 1, 2, clientKey, contract.NowMS(), stickyTTL); err != nil {
		t.Fatal(err)
	}
	if got := mr.TTL(activeKey); got <= 0 {
		t.Fatalf("sticky session should have TTL after touch, got %v", got)
	}
	if _, ok, err := store.UpdateSessionScope(ctx, 1, 2, clientKey, []string{"V-1"}, []uint16{3}, false); err != nil || !ok {
		t.Fatalf("update scope: ok=%v err=%v", ok, err)
	}
	if got := mr.TTL(activeKey); got <= 0 {
		t.Fatalf("TTL stripped by UpdateSessionScope: got %v", got)
	}
}

// TestTouchSeenBatchUpdatesAndPreservesTTL verifies the batched seen
// flusher path: multiple clients' lastSeenAt update in one round-trip
// and sticky TTLs are refreshed.
func TestTouchSeenBatchUpdatesAndPreservesTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	clients := []string{"z21:10.0.0.1:40001", "z21:10.0.0.2:40002"}
	for i, ck := range clients {
		req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
			LayoutID: 1, CommandStationID: 2, UserID: uint(100 + i), AllowAllVehicles: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, ck, contract.NowMS()); err != nil || !ok {
			t.Fatalf("pair %s: ok=%v err=%v", ck, ok, err)
		}
	}

	const stickyTTL = 30 * time.Minute
	ts := []int64{contract.NowMS() + 1000, contract.NowMS() + 2000}
	if err := store.TouchSeenBatch(ctx, 1, 2, clients, ts, stickyTTL); err != nil {
		t.Fatal(err)
	}

	for i, ck := range clients {
		active, ok, err := store.GetActiveByClientKey(ctx, 1, 2, ck)
		if err != nil || !ok {
			t.Fatalf("get %s: ok=%v err=%v", ck, ok, err)
		}
		if active.LastSeenAt != ts[i] {
			t.Fatalf("lastSeenAt %s: got %d want %d", ck, active.LastSeenAt, ts[i])
		}
		key := contract.RemotePairingActiveKey(1, 2, ck)
		if got := mr.TTL(key); got <= 0 {
			t.Fatalf("sticky TTL not refreshed for %s: %v", ck, got)
		}
	}
}

// TestPairViaCV3CV4ReportsEvicted verifies the re-pair path surfaces the
// prior clientKey so the caller can clean up its in-process state.
func TestPairViaCV3CV4ReportsEvicted(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()
	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           31,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	first := "z21:10.0.0.31:40001"
	if _, ok, evicted, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, first, contract.NowMS()); err != nil || !ok || evicted != "" {
		t.Fatalf("first pair: ok=%v evicted=%q err=%v", ok, evicted, err)
	}

	// Plant a second pending req for the already-paired user directly
	// (CreateZ21PairingRequest refuses re-pair), then complete it from a
	// new clientKey. The Lua script must evict the prior session and
	// report the old clientKey.
	second := "z21:10.0.0.31:40002"
	reqID := "z21:211:222"
	pending := contract.RemotePendingWire{
		LayoutID:         1,
		CommandStationID: 2,
		Protocol:         contract.RemoteProtocolZ21,
		UserID:           31,
		ReqID:            reqID,
		DisplayLabel:     "211-222",
		AllowAllVehicles: true,
		CreatedAt:        contract.NowMS(),
	}
	payload, _ := contract.MarshalRemotePending(pending)
	mr.Set(contract.RemotePairingReqKey(1, 2, reqID), string(payload))
	mr.Set(contract.RemotePairingReqByUserKey(1, 2, 31), contract.RemotePairingReqKey(1, 2, reqID))

	_, ok, evicted, err := store.CompletePairing(ctx, 1, 2, reqID, second, contract.NowMS(), "211:222")
	if err != nil || !ok {
		t.Fatalf("complete: ok=%v err=%v", ok, err)
	}
	if evicted != first {
		t.Fatalf("expected evicted=%q, got %q", first, evicted)
	}
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, first); err != nil || ok {
		t.Fatalf("prior session should be evicted: ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, second); err != nil || !ok {
		t.Fatalf("new session should be active: ok=%v err=%v", ok, err)
	}
}
