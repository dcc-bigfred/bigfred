package withrottle

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotepairing"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes"
)

func TestHUTriggersInitialBurst(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	wtWrite(t, conn, "HUengine-driver")
	if got := wtRead(t, r); got != "VN2.0" {
		t.Fatalf("first line after HU: got %q want VN2.0", got)
	}
	if got := wtRead(t, r); got != "*10" {
		t.Fatalf("second line after HU: got %q want *10", got)
	}
}

func TestHUReconnectWithSameDeviceGetsFreshBurst(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})

	first, _ := dialWT(t, srv)
	wtWrite(t, first, "HUsame-device")
	assertInitialBurst(t, first)

	second, _ := dialWT(t, srv)
	wtWrite(t, second, "HUsame-device")
	assertInitialBurst(t, second)
}

func assertInitialBurst(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	sawRoster := false
	for i := 0; i < 5; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch i {
		case 0:
			if line != "VN2.0" {
				t.Fatalf("first burst line: got %q want VN2.0", line)
			}
		case 1:
			if line != "*10" {
				t.Fatalf("second burst line: got %q want *10", line)
			}
		}
		sawRoster = sawRoster || strings.HasPrefix(line, "RL")
	}
	if !sawRoster {
		t.Fatal("initial burst did not contain roster")
	}
}

// Engine Driver sends N<name> before HU<id>. v1 ignored N until HU, so the
// burst carried the paired roster; the anonymous session must not get a
// burst with the "Pair with BigFred" sentinel first.
func TestNBeforeHUPairedGetsPairedRoster(t *testing.T) {
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
	if _, ok, _, err := store.PairViaWithrottleCode(ctx, 1, 1, req.PairingCode, ClientKeyForDevice("ed-n-first"), contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		Store:            store,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
		AllowedVehicles: contract.AllowedVehicles{Vehicles: []contract.AllowedVehicle{
			{VehicleID: "v42", DisplayName: "Shunter", Addr: 42, OwnerUserID: 9, ControllerUserIDs: []uint{9}},
		}},
	})
	conn, r := dialWT(t, srv)
	wtWrite(t, conn, "NMy Throttle")
	if n := drainQuiet(t, conn, r, 150*time.Millisecond); n != 0 {
		t.Fatalf("N before HU must not trigger the initial burst, got %d lines", n)
	}
	wtWrite(t, conn, "HUed-n-first")
	roster := drainUntilLine(t, r, func(line string) bool { return strings.HasPrefix(line, "RL") })
	if !strings.Contains(roster, "Shunter") || strings.Contains(roster, "Pair with BigFred") {
		t.Fatalf("burst roster after N,HU: %q", roster)
	}
	if got := srv.registry.deviceName(ClientKeyForDevice("ed-n-first")); got != "" {
		t.Fatalf("device name should not be recorded pre-HU (v1 ignored N before HU), got %q", got)
	}
}

func TestHUReconnectWithCoordinatorKeepsTCP(t *testing.T) {
	coord := remotes.NewCoordinator(remotes.CoordinatorConfig{
		LayoutID:         1,
		CommandStationID: 1,
	})
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
		Coordinator:      coord,
	})

	first, _ := dialWT(t, srv)
	wtWrite(t, first, "HUsame-device")
	assertInitialBurst(t, first)

	second, _ := dialWT(t, srv)
	wtWrite(t, second, "HUsame-device")
	assertInitialBurst(t, second)

	if _, err := second.Write([]byte("*\n")); err != nil {
		t.Fatalf("replacement TCP closed after HU takeover: %v", err)
	}
}
