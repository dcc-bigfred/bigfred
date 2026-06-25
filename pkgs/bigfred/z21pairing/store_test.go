package z21pairing_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

func newTestStore(t *testing.T) (*z21pairing.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return z21pairing.NewStore(client), mr
}

func TestCreateAndPairViaCV3CV4(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	req, err := store.CreatePairingRequest(ctx, z21pairing.CreatePairingRequestInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           9,
		VehicleIDs:       []string{"V-1"},
		AllowedAddrs:     []uint16{3},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
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
	active, ok, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, "10.0.0.1:21105", now)
	if err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	if active.ClientKey != "10.0.0.1:21105" || active.UserID != 9 {
		t.Fatalf("active: %+v", active)
	}

	_, ok, err = store.GetPendingByUser(ctx, 1, 2, 9)
	if err != nil || ok {
		t.Fatalf("pending should be gone: ok=%v err=%v", ok, err)
	}

	loaded, ok, err := store.GetActiveByClientKey(ctx, 1, 2, "10.0.0.1:21105")
	if err != nil || !ok || loaded.LastSeenAt != now {
		t.Fatalf("load active: ok=%v err=%v active=%+v", ok, err, loaded)
	}
}

func TestPairRejectsInvalidCV(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	_, ok, err := store.PairViaCV3CV4(ctx, 1, 2, 99, 145, "1.2.3.4:1", contract.NowMS())
	if err == nil || ok {
		t.Fatalf("expected invalid CV error, ok=%v err=%v", ok, err)
	}
}

func TestTouchSeenAndUnpair(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	req, err := store.CreatePairingRequest(ctx, z21pairing.CreatePairingRequestInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           4,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientKey := "192.168.1.10:40000"
	if _, ok, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, clientKey, contract.NowMS()); err != nil || !ok {
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
	in := z21pairing.CreatePairingRequestInput{
		LayoutID:         3,
		CommandStationID: 4,
		UserID:           1,
		AllowedAddrs:     []uint16{10},
	}
	first, err := store.CreatePairingRequest(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePairingRequest(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.PairingCV3 == second.PairingCV3 && first.PairingCV4 == second.PairingCV4 {
		t.Fatalf("expected new pair, got same: %+v", second)
	}
	_, ok, err := store.PairViaCV3CV4(ctx, 3, 4, first.PairingCV3, first.PairingCV4, "1.1.1.1:1", contract.NowMS())
	if err != nil || ok {
		t.Fatalf("stale req should not pair: ok=%v err=%v", ok, err)
	}
}
