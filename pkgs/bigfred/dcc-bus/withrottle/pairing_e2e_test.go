package withrottle

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotepairing"
)

func testPairingStore(t *testing.T) *remotepairing.Store {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return remotepairing.NewStore(rdb)
}

func TestNNamePairing(t *testing.T) {
	store := testPairingStore(t)
	ctx := context.Background()
	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		Store:            store,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "n-pair")
	wtWrite(t, conn, "N"+req.PairingCode)
	drainUntilLine(t, r, func(line string) bool { return strings.HasPrefix(line, "HmPaired") })
	if !srv.registry.IsPaired("withrottle:n-pair") {
		t.Fatal("expected paired after N-code")
	}
}

func TestFailedNCodeDoesNotSendHeartbeat(t *testing.T) {
	store := testPairingStore(t)
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		Store:            store,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "n-miss")
	wtWrite(t, conn, "N000000")
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if line, err := r.ReadString('\n'); err == nil {
		t.Fatalf("unexpected reply after failed N-code %q", strings.TrimRight(line, "\r\n"))
	}
}

func TestFunctionKeyPairingEndToEnd(t *testing.T) {
	store := testPairingStore(t)
	ctx := context.Background()
	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		Store:            store,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "fn-pair")
	key := locoKeyForAddr(srv.cfg.PairingAddr)
	wtWrite(t, conn, fmt.Sprintf("M0+%s%s", key, propSep))
	drainUntilLine(t, r, func(line string) bool { return strings.Contains(line, "<;>s1") })
	for _, ch := range req.PairingCode {
		wtWrite(t, conn, fmt.Sprintf("M0A%s%sF1%c", key, propSep, ch))
		wtWrite(t, conn, fmt.Sprintf("M0A%s%sF0%c", key, propSep, ch))
	}
	drainUntilLine(t, r, func(line string) bool { return strings.HasPrefix(line, "HmPaired") })
	if !srv.registry.IsPaired("withrottle:fn-pair") {
		t.Fatal("expected paired after F-key code")
	}
}

func TestQuitUnpairsButTcpDropKeepsPairing(t *testing.T) {
	store := testPairingStore(t)
	ctx := context.Background()
	req, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Q unpairs", func(t *testing.T) {
		srv := startWT(t, Config{
			LayoutID:         1,
			CommandStationID: 1,
			Store:            store,
			HeartbeatSecs:    10,
		})
		conn, r := dialWT(t, srv)
		handshakeHU(t, conn, r, "quit-dev")
		key := ClientKeyForDevice("quit-dev")
		if _, ok, _, err := store.PairViaWithrottleCode(ctx, 1, 1, req.PairingCode, key, contract.NowMS()); err != nil || !ok {
			t.Fatalf("pair: ok=%v err=%v", ok, err)
		}
		srv.registry.SetPaired(key, &contract.RemoteSessionWire{ClientKey: key, UserID: 9, AllowAllVehicles: true})
		wtWrite(t, conn, "Q")
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok, err := store.GetActiveByClientKey(ctx, 1, 1, key); err == nil && !ok {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("Q should unpair Redis session")
	})

	req2, err := store.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         1,
		CommandStationID: 1,
		UserID:           9,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		Store:            store,
		HeartbeatSecs:    10,
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "drop-dev")
	key := ClientKeyForDevice("drop-dev")
	if _, ok, _, err := store.PairViaWithrottleCode(ctx, 1, 1, req2.PairingCode, key, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	srv.registry.SetPaired(key, &contract.RemoteSessionWire{ClientKey: key, UserID: 9, AllowAllVehicles: true})
	_ = conn.Close()
	time.Sleep(200 * time.Millisecond)
	if _, ok, err := store.GetActiveByClientKey(ctx, 1, 1, key); err != nil || !ok {
		t.Fatalf("TCP drop must keep Redis pairing: ok=%v err=%v", ok, err)
	}
}
