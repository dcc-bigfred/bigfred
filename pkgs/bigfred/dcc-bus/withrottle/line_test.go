package withrottle

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestParseMAction_acquire(t *testing.T) {
	cmd, ok := parseMAction("M0+S3<;>S3")
	if !ok || cmd.Op != MOpAdd || cmd.LocoKey != "S3" {
		t.Fatalf("parseMAction: %+v ok=%v", cmd, ok)
	}
}

func TestParseFunctionAction(t *testing.T) {
	fn, on, _, ok := parseFunctionAction("F112")
	if !ok || !on || fn != 12 {
		t.Fatalf("F112: fn=%d on=%v ok=%v", fn, on, ok)
	}
	_, _, _, ok = parseFunctionAction("F132")
	if ok {
		t.Fatal("F132 should be rejected")
	}
}

func TestBuildRosterLineAllowAll(t *testing.T) {
	session := &contract.RemoteSessionWire{AllowAllVehicles: true}
	allowed := contract.AllowedVehicles{
		Vehicles: []contract.AllowedVehicle{
			{VehicleID: "A", Addr: 3},
			{VehicleID: "B", Addr: 128},
		},
	}
	line := BuildRosterLine(session, allowed, 10239, true)
	if line == "RL0" {
		t.Fatalf("allow-all should list layout vehicles, got %q", line)
	}
	if want := "RL2"; line[:3] != want {
		t.Fatalf("count prefix: got %q want %s", line[:3], want)
	}
}

func TestDccSpeedRoundTrip(t *testing.T) {
	for _, dcc := range []uint8{0, 1, 64, 127} {
		wire := wireSpeedFromDCC(dcc, 128)
		back := dccSpeedFromWire(wire, 128)
		if back != dcc {
			t.Fatalf("dcc %d -> wire %d -> %d", dcc, wire, back)
		}
	}
}
