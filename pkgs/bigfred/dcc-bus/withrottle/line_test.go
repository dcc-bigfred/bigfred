package withrottle

import (
	"fmt"
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

func TestBuildSentinelAcquireReply(t *testing.T) {
	const addr uint16 = 10239
	lines := buildSentinelAcquireReply('0', addr)
	if len(lines) < 14 {
		t.Fatalf("expected at least 14 lines, got %d: %v", len(lines), lines)
	}
	wantLabels := "M0LL10239<;>]\\[F0]\\[F1]\\[F2]\\[F3]\\[F4]\\[F5]\\[F6]\\[F7]\\[F8]\\[F9"
	if lines[1] != wantLabels {
		t.Fatalf("labels line:\n got %q\nwant %q", lines[1], wantLabels)
	}
	for fn := 0; fn <= 9; fn++ {
		want := fmt.Sprintf("M0AL10239<;>F0%d", fn)
		if lines[2+fn] != want {
			t.Fatalf("F0%d state: got %q want %q", fn, lines[2+fn], want)
		}
	}
	if lines[12] != "M0AL10239<;>V0" {
		t.Fatalf("V0: %q", lines[12])
	}
	if lines[13] != "M0AL10239<;>R1" {
		t.Fatalf("R1: %q", lines[13])
	}
	if lines[14] != "M0AL10239<;>s1" {
		t.Fatalf("s1: %q", lines[14])
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
